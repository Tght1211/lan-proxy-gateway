// Package console is the numbered-menu interactive UI for lan-proxy-gateway.
//
// Design goals (per the v2 refactor):
//   - Low barrier. No arrow keys, no mouse. User types a number and presses Enter.
//     Works on every terminal, every SSH client, every Windows PowerShell.
//   - One path. Every action routes through internal/app; there is no duplicate
//     implementation in a cobra command.
//   - Three top-level screens matching the three feature layers:
//     1) 网关状态 & 设备接入  (gateway)
//     2) 流量控制               (traffic)
//     3) 代理端口                (source)
//   - First-time users go through a 3-step onboarding wizard before reaching the menu.
package console

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/devices"
	"github.com/tght/lan-proxy-gateway/internal/engine"
	"github.com/tght/lan-proxy-gateway/internal/gateway"
	"github.com/tght/lan-proxy-gateway/internal/geoip"
	"github.com/tght/lan-proxy-gateway/internal/platform"
	"github.com/tght/lan-proxy-gateway/internal/source"
)

// Run is the entry point. It blocks until the user exits.
func Run(ctx context.Context, a *app.App) error {
	c := newConsole(a, os.Stdin, os.Stdout)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if !a.Configured() {
		if err := c.onboard(runCtx); err != nil {
			return err
		}
	}
	// 拉起代理源健康 supervisor：mihomo 在跑时自动体检，挂了切 direct 保命。
	a.StartSupervisor(runCtx)
	return c.main(runCtx)
}

// RunOnboarding runs only the 3-step wizard (no main menu). Used by `install`
// which has its own post-wizard flow (predownload geodata, auto-start).
func RunOnboarding(ctx context.Context, a *app.App) error {
	c := newConsole(a, os.Stdin, os.Stdout)
	return c.onboard(ctx)
}

type consoleUI struct {
	app *app.App
	in  *bufio.Reader
	out io.Writer

	// 仪表盘相关：geo 打开失败时留 nil，Lookup 会安全返回空。
	// resolver 总是非 nil；DeviceLabels 变化时要调 SetLabels 同步。
	geo       *geoip.DB
	resolver  *devices.Resolver
	dashState dashboardState
}

func newConsole(a *app.App, in io.Reader, out io.Writer) *consoleUI {
	c := &consoleUI{app: a, in: bufio.NewReader(in), out: out}
	// 打 country.mmdb。文件来自 EnsureGeodata；没下来就留 nil，Lookup 安全降级。
	if a != nil && a.Engine != nil {
		if db, err := geoip.Open(filepath.Join(a.Engine.Workdir(), "country.mmdb")); err == nil {
			c.geo = db
		}
	}
	var labels map[string]string
	if a != nil {
		labels = a.Cfg.Gateway.DeviceLabels
	}
	c.resolver = devices.NewResolver(labels)
	return c
}

// readLine 同步从 stdin 读一行。bubbletea 跑期间会接管 stdin，不要在 TUI
// 活着时调这个函数；进 simple screen（M 菜单、切节点 etc.）时 tea 已 Run 返
// 回，stdin 归还给我们。
func (c *consoleUI) readLine() string {
	line, _ := c.in.ReadString('\n')
	return strings.TrimSpace(line)
}

// --- Rendering helpers ---

var (
	titleC = color.New(color.FgCyan, color.Bold)
	okC    = color.New(color.FgGreen, color.Bold)
	warnC  = color.New(color.FgYellow)
	dimC   = color.New(color.Faint)
	badC   = color.New(color.FgRed, color.Bold)
	bar    = "────────────────────────────────────────────────"
)

func (c *consoleUI) banner(title string) {
	fmt.Fprintln(c.out)
	titleC.Fprintln(c.out, bar)
	titleC.Fprintf(c.out, "  %s\n", title)
	titleC.Fprintln(c.out, bar)
}

func (c *consoleUI) prompt(label string) string {
	fmt.Fprintf(c.out, "%s", label)
	return c.readLine()
}

func (c *consoleUI) ask(label, def string) string {
	if def != "" {
		dimC.Fprintf(c.out, "%s（回车=%s）: ", label, def)
	} else {
		fmt.Fprintf(c.out, "%s: ", label)
	}
	line := c.readLine()
	if line == "" {
		return def
	}
	return line
}

func (c *consoleUI) yesNo(label string, def bool) bool {
	hint := "(Y/n)"
	if !def {
		hint = "(y/N)"
	}
	for {
		line := strings.ToLower(c.prompt(fmt.Sprintf("%s %s ", label, hint)))
		switch line {
		case "":
			return def
		case "y", "yes":
			return true
		case "n", "no":
			return false
		}
		warnC.Fprintln(c.out, "请输入 y 或 n。")
	}
}

// --- Onboarding: 只问代理端口，其它用推荐默认 ---
//
// 设计：小白用户 80% 的情况下只需要告诉网关 "把流量转发到哪"。
// 网关开关 / TUN / DNS / 规则模式 / 广告拦截 全部用推荐默认值，
// 在主菜单里可以随时调整。
func (c *consoleUI) onboard(ctx context.Context) error {
	c.banner("欢迎使用 lan-proxy-gateway · 首次配置")

	// 检测网络，展示给用户看（让人心里有数，但不用选）
	if err := c.app.Gateway.Detect(); err == nil {
		info := c.app.Gateway.Info()
		fmt.Fprintf(c.out, "  已检测到你的网络：\n")
		fmt.Fprintf(c.out, "    默认接口  %s\n", info.Interface)
		fmt.Fprintf(c.out, "    本机 IP   %s\n", info.IP)
		fmt.Fprintf(c.out, "    路由器 IP %s\n\n", info.Gateway)
	}

	// 推荐默认（主菜单里可随时改）
	c.app.Cfg.Gateway.Enabled = true
	c.app.Cfg.Gateway.TUN.Enabled = true
	c.app.Cfg.Gateway.DNS.Enabled = true
	c.app.Cfg.Traffic.Mode = config.ModeRule
	c.app.Cfg.Traffic.Adblock = true

	okC.Fprintln(c.out, "  已为你启用推荐配置：")
	fmt.Fprintln(c.out, "    ✓ 局域网共享网关")
	fmt.Fprintln(c.out, "    ✓ TUN 虚拟网卡  【必开】")
	warnC.Fprintln(c.out, "       关了 TUN 的话，Switch/PS5/Apple TV 改了网关也只会被【傻路由】转发，")
	warnC.Fprintln(c.out, "       它们照样被墙。TUN 才是真正让流量走代理的关键。")
	warnC.Fprintln(c.out, "       只有"+dimC.Sprint("手机/电脑能手动填代理服务器时")+"才能关。")
	fmt.Fprintln(c.out, "    ✓ DNS 代理（端口 53）")
	fmt.Fprintln(c.out, "    ✓ 规则模式（国内直连 + 国外代理）")
	fmt.Fprintln(c.out, "    ✓ 广告拦截")
	dimC.Fprintln(c.out, "    （以上都能在主菜单→分流 & 规则 里随时改）")
	fmt.Fprintln(c.out)

	// 唯一需要用户决策的：代理端口来源
	titleC.Fprintln(c.out, "  只剩一件事：把流量转发到哪里？")
	fmt.Fprintln(c.out, "    1) 单点代理        (填 主机+端口；本机 Clash Verge / 远程机场的单个节点都走这个)")
	fmt.Fprintln(c.out, "    2) 机场订阅        (粘一个订阅 URL，网关自己抓节点列表)")
	fmt.Fprintln(c.out, "    3) 本地配置文件    (指向一个 .yaml；格式和机场订阅一致，只是本地)")
	fmt.Fprintln(c.out, "    4) 暂不配置        (全部走直连，以后再来)")
	fmt.Fprintln(c.out)
	choice := c.ask("请选择 1-4", "1")
	switch choice {
	case "2":
		c.configureSubscription()
	case "3":
		c.configureFile()
	case "4":
		c.app.Cfg.Source.Type = config.SourceTypeNone
	default:
		c.configureSingle()
	}

	if err := c.app.Save(); err != nil {
		badC.Fprintf(c.out, "保存配置失败: %v\n", err)
		return err
	}
	okC.Fprintln(c.out, "\n✔ 配置已保存到 "+c.app.Paths.ConfigFile)
	return nil
}

// --- Source-type configurators ---

// configureSingle 是「单点代理」入口，合并以前的 external + remote。
// 填了用户名就存 SourceTypeRemote（有认证），否则存 SourceTypeExternal（无认证），
// 两种 type 底层 materialize 出来的 mihomo proxy 形态一致，只是认证字段的有无。
func (c *consoleUI) configureSingle() {
	// 从当前配置取已有值做默认（无论目前是 external 还是 remote）
	defServer, defPort, defKind, defUser, defPass := "127.0.0.1", 7890, "http", "", ""
	switch c.app.Cfg.Source.Type {
	case config.SourceTypeExternal:
		e := c.app.Cfg.Source.External
		defServer = firstNonEmpty(e.Server, defServer)
		defPort = firstNonZero(e.Port, defPort)
		defKind = firstNonEmpty(e.Kind, defKind)
	case config.SourceTypeRemote:
		r := c.app.Cfg.Source.Remote
		defServer = firstNonEmpty(r.Server, defServer)
		defPort = firstNonZero(r.Port, defPort)
		defKind = firstNonEmpty(r.Kind, defKind)
		defUser = r.Username
		defPass = r.Password
	}

	server := c.ask("  主机（本机代理就填 127.0.0.1）", defServer)
	portStr := c.ask("  端口", strconv.Itoa(defPort))
	port := defPort
	if p, err := strconv.Atoi(strings.TrimSpace(portStr)); err == nil && p > 0 {
		port = p
	}
	kind := strings.ToLower(c.ask("  类型 (http 或 socks5)", defKind))
	if kind != "socks5" {
		kind = "http"
	}
	user := c.ask("  用户名（不需要认证直接回车）", defUser)
	pass := defPass
	if user != "" {
		pass = c.ask("  密码", defPass)
	}

	if user == "" {
		c.app.Cfg.Source.Type = config.SourceTypeExternal
		c.app.Cfg.Source.External = config.ExternalProxy{
			Name:   firstNonEmpty(c.app.Cfg.Source.External.Name, "单点代理"),
			Server: server,
			Port:   port,
			Kind:   kind,
		}
	} else {
		c.app.Cfg.Source.Type = config.SourceTypeRemote
		c.app.Cfg.Source.Remote = config.RemoteProxy{
			Name:     firstNonEmpty(c.app.Cfg.Source.Remote.Name, "单点代理"),
			Server:   server,
			Port:     port,
			Kind:     kind,
			Username: user,
			Password: pass,
		}
	}
}

// configureScript 分三路：预设（链式代理向导）/ 自定义 .js 路径 / 清除。
// 预设会把用户填的住宅 IP 实例化到 workdir 的模板脚本，不用自己写 JS。
func (c *consoleUI) configureScript() {
	for {
		c.banner("全局扩展脚本")
		// 当前状态
		state := "未配置"
		if c.app.Cfg.Source.ChainResidential != nil {
			r := c.app.Cfg.Source.ChainResidential
			state = fmt.Sprintf("预设 · 链式代理（住宅 IP %s:%d %s）", r.Server, r.Port, r.Kind)
		} else if c.app.Cfg.Source.ScriptPath != "" {
			state = "自定义脚本 · " + c.app.Cfg.Source.ScriptPath
		}
		fmt.Fprintf(c.out, "  当前：%s\n\n", state)
		fmt.Fprintln(c.out, "  1  预设 · 链式代理（住宅 IP 落地）")
		dimC.Fprintln(c.out, "      填订阅节点先走机场，再链到住宅 IP 出国；AI 域名自动走住宅 IP")
		fmt.Fprintln(c.out, "  2  自定义 .js 文件路径")
		dimC.Fprintln(c.out, "      Clash Verge Rev 同款 main(config) 脚本")
		fmt.Fprintln(c.out, "  3  清除当前脚本")
		fmt.Fprintln(c.out)
		titleC.Fprintln(c.out, "  ── 操作 ── 0 返回（或按 Q）")
		switch strings.ToLower(c.prompt("选择：> ")) {
		case "1":
			c.configureScriptResidentialChain()
			return
		case "2":
			c.configureScriptCustomPath()
			return
		case "3":
			c.app.Cfg.Source.ScriptPath = ""
			c.app.Cfg.Source.ChainResidential = nil
			okC.Fprintln(c.out, "  已清除全局扩展脚本")
			return
		case "0", "q", "":
			return
		default:
			warnC.Fprintln(c.out, "无效选项")
		}
	}
}

// configureScriptResidentialChain 引导用户填住宅 IP 节点字段，然后写入配置。
// 脚本本身会在 render 时从内嵌模板实例化到 workdir，用户不用碰 JS 代码。
func (c *consoleUI) configureScriptResidentialChain() {
	cur := c.app.Cfg.Source.ChainResidential
	defName, defKind, defServer, defPort, defUser, defPass := "🏠 住宅IP", "socks5", "", 0, "", ""
	if cur != nil {
		defName = firstNonEmpty(cur.Name, defName)
		defKind = firstNonEmpty(cur.Kind, defKind)
		defServer = cur.Server
		defPort = cur.Port
		defUser = cur.Username
		defPass = cur.Password
	}

	fmt.Fprintln(c.out, "\n  请填写住宅 IP 落地节点（最终流量经机场 → 住宅 IP 出国）：")
	name := c.ask("  节点名称", defName)
	kind := strings.ToLower(c.ask("  类型 (http / socks5)", defKind))
	if kind != "http" && kind != "socks5" {
		kind = "socks5"
	}
	server := c.ask("  服务器地址", defServer)
	if server == "" {
		warnC.Fprintln(c.out, "  服务器地址不能为空，取消")
		return
	}
	portStr := c.ask("  端口", strconv.Itoa(firstNonZero(defPort, 443)))
	port, err := strconv.Atoi(strings.TrimSpace(portStr))
	if err != nil || port <= 0 || port > 65535 {
		warnC.Fprintln(c.out, "  端口无效，取消")
		return
	}
	user := c.ask("  用户名（无需认证回车）", defUser)
	pass := defPass
	if user != "" {
		pass = c.ask("  密码", defPass)
	}

	c.app.Cfg.Source.ChainResidential = &config.ChainResidentialConfig{
		Name:        name,
		Kind:        kind,
		Server:      server,
		Port:        port,
		Username:    user,
		Password:    pass,
		DialerProxy: "🛫 AI起飞节点",
	}
	// 预设会接管 ScriptPath，清掉用户自定义路径避免混乱。
	c.app.Cfg.Source.ScriptPath = ""
	okC.Fprintf(c.out, "  ✓ 已保存链式代理预设（%s:%d %s），下次 start/reload 生效\n", server, port, kind)
}

// configureScriptCustomPath 让用户填自定义 .js 路径（高级用法）。
func (c *consoleUI) configureScriptCustomPath() {
	fmt.Fprintln(c.out, "\n  填入 .js 绝对路径。直接回车 = 清除。")
	path := strings.TrimSpace(c.ask("  脚本路径", c.app.Cfg.Source.ScriptPath))
	if path == "" {
		c.app.Cfg.Source.ScriptPath = ""
		c.app.Cfg.Source.ChainResidential = nil
		okC.Fprintln(c.out, "  已清除全局扩展脚本")
		return
	}
	if _, err := os.Stat(path); err != nil {
		warnC.Fprintf(c.out, "  ⚠ 找不到 %s: %v\n", path, err)
		if !c.yesNo("  仍要保存这个路径？", false) {
			return
		}
	}
	c.app.Cfg.Source.ScriptPath = path
	c.app.Cfg.Source.ChainResidential = nil
	okC.Fprintf(c.out, "  已设置自定义脚本: %s\n", path)
}

func (c *consoleUI) configureSubscription() bool {
	oldType := c.app.Cfg.Source.Type
	oldURL := c.app.Cfg.Source.Subscription.URL
	oldName := c.app.Cfg.Source.Subscription.Name

	c.app.Cfg.Source.Type = config.SourceTypeSubscription
	s := &c.app.Cfg.Source.Subscription
	s.URL = c.ask("  订阅 URL", s.URL)
	s.Name = c.ask("  订阅名称", firstNonEmpty(s.Name, "subscription"))

	return oldType != c.app.Cfg.Source.Type || oldURL != s.URL || oldName != s.Name
}

func (c *consoleUI) configureFile() bool {
	oldType := c.app.Cfg.Source.Type
	oldPath := c.app.Cfg.Source.File.Path

	c.app.Cfg.Source.Type = config.SourceTypeFile
	c.app.Cfg.Source.File.Path = c.ask("  本地配置文件绝对路径 (Clash/mihomo YAML)", c.app.Cfg.Source.File.Path)

	return oldType != c.app.Cfg.Source.Type || oldPath != c.app.Cfg.Source.File.Path
}

// --- Main loop: 仪表盘首页 ---
//
// 默认只画 dashboard（实时速率 / 累计 / 起飞落地 / 设备表），每 2 秒自动刷。
// 旧的多级菜单藏到 [M] 键后面，避免首屏被操作项淹没。
// 快捷键：M 菜单、N 切节点、T 重测代理源、Q 退出控制台（网关留后台）。

func (c *consoleUI) main(ctx context.Context) error {
	// 静态首页：画一次 dashboard，等用户输入。回车 / R 主动刷新，避免自动刷新
	// 抢终端。想要更顺手的操作直接看首页给出的 Web 控制台地址，用浏览器做。
	for {
		c.drawDashboardOnce(ctx)
		choice := strings.ToLower(strings.TrimSpace(c.readLine()))
		switch choice {
		case "", "r":
			// 回车 / R → 重新拉数据再画一次
		case "m", "menu":
			if c.screenMenu(ctx) {
				return nil
			}
		case "n":
			c.screenSwitchNode(ctx)
		case "t":
			c.screenSource(ctx)
		case "q", "exit", "quit":
			return nil
		default:
			warnC.Fprintln(c.out, "无效选项（回车 / R 刷新 · M 菜单 · N 切节点 · T 重测 · Q 退出）")
		}

		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
}

// drawDashboardOnce 拉一次数据 + 画一帧。失败路径会自己输出占位文案。
func (c *consoleUI) drawDashboardOnce(ctx context.Context) {
	running := c.app.Engine != nil && c.app.Engine.Running()
	// resolver 的 labels 可能被菜单里「设备标签」页改过，每帧同步一次成本极低。
	c.resolver.SetLabels(c.app.Cfg.Gateway.DeviceLabels)

	localIP := c.app.Status().Gateway.LocalIP
	var cli *engine.Client
	if running {
		cli = c.app.Engine.API()
	}
	snap := fetchDashboardSnapshot(ctx, cli, c.app.Cfg, localIP, c.geo, c.resolver, &c.dashState)

	// 代理源异常告警（supervisor 维护的状态），叠在仪表盘顶部警示。
	drawDashboard(c.out, snap, running)
	if running {
		apiPort := c.app.Cfg.Runtime.Ports.API
		fmt.Fprintln(c.out)
		titleC.Fprintln(c.out, "  Web 控制台（推荐在浏览器操作）")
		fmt.Fprintf(c.out, "    本机    http://127.0.0.1:%d/ui/\n", apiPort)
		if localIP != "" {
			fmt.Fprintf(c.out, "    局域网  http://%s:%d/ui/\n", localIP, apiPort)
		}
		dimC.Fprintln(c.out, "    浏览器里更适合切节点、看连接、做细操作。")
	}
	h := c.app.Health()
	if h.FallbackActive {
		badC.Fprintln(c.out)
		badC.Fprintf(c.out, "  ⚠ 代理源异常 · 已临时切直连：%s\n", h.LastError)
	}
	fmt.Fprint(c.out, "\n请选择：> ")
}

// screenMenu 是旧版主菜单折成的「操作抽屉」。返回 true 表示用户在里头关了
// gateway，主循环该一起退出。
func (c *consoleUI) screenMenu(ctx context.Context) (exitConsole bool) {
	for {
		c.drawMainMenu()
		choice := strings.ToLower(c.readLine())
		switch choice {
		case "":
		case "1":
			c.screenGateway()
		case "2":
			c.screenTraffic(ctx)
		case "3":
			c.screenSource(ctx)
		case "4":
			c.screenLifecycle(ctx)
		case "5":
			c.screenLogs()
		case "6":
			if c.shutdownGateway() {
				return true
			}
		case "0", "back":
			return false
		case "q", "exit", "quit":
			return true
		default:
			warnC.Fprintln(c.out, "无效选项（数字=子菜单，0 返回仪表盘，Q 退出控制台）")
		}
	}
}

// drawMainMenu 画一次主菜单屏。main loop 里的每次进入/重绘都用它。
func (c *consoleUI) drawMainMenu() {
	c.banner("LAN 代理网关")
	c.printStatus()
	fmt.Fprintln(c.out)
	fmt.Fprintln(c.out, "  1  设备接入指引      Switch / PS5 / 手机怎么连到这里")
	fmt.Fprintln(c.out, "  2  分流 & 规则        国内直连 / 国外走代理 / 广告拦截 / TUN 开关")
	fmt.Fprintln(c.out, "  3  代理 & 订阅        换代理 · 切节点 · 连通测试 · 全局扩展脚本")
	fmt.Fprintln(c.out, "  4  启动 / 重启 / 停止")
	fmt.Fprintln(c.out, "  5  看日志")
	fmt.Fprintln(c.out, "  6  关闭 gateway 并退出（停 mihomo）")
	fmt.Fprintln(c.out, "  Q  退出控制台（mihomo 留在后台继续跑）")
	fmt.Fprint(c.out, "\n请选择：> ")
}

// printStatus 在主菜单顶部显示 3 件最关键的信息：
//   - 运行状态（● 运行中 / ○ 未启动）
//   - 本机 IP（LAN 设备要填这个做网关）
//   - 代理源（用中文描述代替 external/subscription 黑话）
//
// 模式 / 广告 / TUN / DNS 这些细节进「2 流量控制」看；这里只保留必要时的
// ⚠ 警告（TUN 关 / DNS 关），正常状态下不出现。
func (c *consoleUI) printStatus() {
	s := c.app.Status()
	if s.Running {
		okC.Fprint(c.out, "  ● 运行中")
	} else {
		dimC.Fprint(c.out, "  ○ 未启动")
	}
	ip := s.Gateway.LocalIP
	if ip == "" {
		ip = "<未检测到>"
	}
	fmt.Fprintf(c.out, "    本机 IP: %s\n", ip)
	fmt.Fprintf(c.out, "  代理源: %s\n", sourceLabel(s.Source))

	// 代理源异常 → supervisor 已切 direct 保证 LAN 通网，但要让用户一眼看到。
	h := c.app.Health()
	if h.FallbackActive {
		badC.Fprintln(c.out, "  ⚠ 代理源异常 · 已临时切到直连（LAN 设备不会断网，但不再走代理）")
		badC.Fprintf(c.out, "    原因: %s\n", h.LastError)
		dimC.Fprintln(c.out, "    修复后会自动切回；想立刻重试去「代理 & 订阅 → T 重新测试」")
	}

	admin, _ := c.app.Plat.IsAdmin()
	if !admin {
		dimC.Fprintln(c.out, "  （未用 sudo；看状态、改配置都不需要；启动时才需要）")
	}
	if s.Running && !s.TUN {
		warnC.Fprintln(c.out, "  ⚠ TUN 已关：Switch / PS5 等就算把网关指到本机，流量也不会走代理")
	}
	if !c.app.Cfg.Gateway.DNS.Enabled {
		warnC.Fprintln(c.out, "  ⚠ DNS 代理已关：LAN 设备 DNS 不能指向本机 IP，需另设能用的 DNS")
	}
}

// sourceLabel 把 config.SourceType 映射成小白能看懂的中文标签。
func sourceLabel(s string) string {
	switch s {
	case config.SourceTypeExternal:
		return "单点代理（本机，external）"
	case config.SourceTypeSubscription:
		return "机场订阅（subscription）"
	case config.SourceTypeFile:
		return "本地配置文件（file）"
	case config.SourceTypeRemote:
		return "单点代理（远程带认证，remote）"
	case config.SourceTypeNone, "":
		return "未配置 · 全部直连"
	default:
		return s
	}
}

// --- Screens ---

func (c *consoleUI) screenGateway() {
	for {
		c.banner("设备接入指引")
		_ = c.app.Gateway.Detect()
		fmt.Fprint(c.out, gateway.DeviceGuide(c.app.Status().Gateway, c.app.Cfg.Runtime.Ports.Mixed))

		// Windows 下 TUN 已经接管本机出向流量，方式 3 不需要切 DNS；且
		// SetLocalDNSToLoopback 在 Windows 上是 ErrNotSupported，按钮按下
		// 只会报错。整块隐藏，避免跟上面指引里"TUN 已开，自动走 mihomo"
		// 自相矛盾。
		if runtime.GOOS == "windows" {
			fmt.Fprintln(c.out)
			titleC.Fprintln(c.out, "  ── 操作 ── T 给 IP 起名字   0 返回（或按 Q）")
			switch strings.ToLower(strings.TrimSpace(c.prompt("选择：> "))) {
			case "t":
				c.screenDeviceLabels()
			case "", "0", "q":
				return
			default:
				warnC.Fprintln(c.out, "无效选项")
			}
			continue
		}

		// 方式 3 的状态灯 + 一键开关（macOS 实打实切；Linux 显示命令提示）
		isLoopback, _ := c.app.Plat.LocalDNSIsLoopback()
		if isLoopback {
			okC.Fprintln(c.out, "\n  本机 DNS: 127.0.0.1（停止 gateway 时自动恢复）")
		} else {
			dimC.Fprintln(c.out, "\n  本机 DNS: 默认")
		}
		fmt.Fprintln(c.out)
		titleC.Fprintln(c.out, "  ── 操作 ── L 切本机 DNS   R 恢复 DNS   T 标记设备   0 返回（或按 Q）")

		switch strings.ToLower(strings.TrimSpace(c.prompt("选择：> "))) {
		case "l":
			if err := c.app.Plat.SetLocalDNSToLoopback(); err != nil {
				if errors.Is(err, platform.ErrNotSupported) {
					warnC.Fprintln(c.out, "  当前系统不支持一键切换，请照上面命令手动改")
					c.pause()
				} else {
					badC.Fprintf(c.out, "  切换失败: %v\n", err)
				}
			} else {
				okC.Fprintln(c.out, "  ✓ 已把本机 DNS 切到 127.0.0.1（顺手关了系统 HTTP/SOCKS 代理）")
			}
		case "r":
			if err := c.app.Plat.RestoreLocalDNS(); err != nil {
				if errors.Is(err, platform.ErrNotSupported) {
					warnC.Fprintln(c.out, "  当前系统不支持一键恢复")
					c.pause()
				} else {
					badC.Fprintf(c.out, "  恢复失败: %v\n", err)
				}
			} else {
				okC.Fprintln(c.out, "  ✓ 已恢复系统默认 DNS")
			}
		case "t":
			c.screenDeviceLabels()
		case "", "0", "q":
			return
		default:
			warnC.Fprintln(c.out, "无效选项")
		}
	}
}

func (c *consoleUI) screenTraffic(ctx context.Context) {
	for {
		c.banner("分流 & 规则")
		cfg := c.app.Cfg
		fmt.Fprintf(c.out, "  模式 %s  ·  TUN %s  ·  广告拦截 %s  ·  自定义规则 %d\n\n",
			cfg.Traffic.Mode,
			onOff(cfg.Gateway.TUN.Enabled),
			onOff(cfg.Traffic.Adblock),
			len(cfg.Traffic.Extras.Direct)+len(cfg.Traffic.Extras.Proxy)+len(cfg.Traffic.Extras.Reject))
		fmt.Fprintln(c.out, "  1  切换模式     rule=国内直连+国外代理（推荐）/ global=全走代理 / direct=全直连")
		fmt.Fprintln(c.out, "  2  开关 TUN     （Switch/PS5 等能走代理的关键，一般别动）")
		fmt.Fprintln(c.out, "  3  开关广告拦截")
		fmt.Fprintln(c.out, "  4  自定义规则   直连 / 代理 / 拒绝 三组（优先级最高，盖过内置 china_direct 等）")
		fmt.Fprintf(c.out, "  5  策略组自动补全 %s   订阅里缺 Auto/Fallback 时自动加（直选节点也能自动切换）\n",
			onOff(cfg.Traffic.AutoGroups))
		dimC.Fprintln(c.out, "  9  高级设置     （DNS 开关 / 端口调整，端口冲突时才来）")
		fmt.Fprintln(c.out)
		titleC.Fprintln(c.out, "  ── 操作 ── 0 返回主菜单（或按 Q）")
		switch c.prompt("选择：> ") {
		case "1":
			fmt.Fprintln(c.out, "  1) rule    规则模式（推荐）")
			fmt.Fprintln(c.out, "  2) global  全局代理")
			fmt.Fprintln(c.out, "  3) direct  全部直连")
			choice := c.prompt("请选择：> ")
			var m string
			switch choice {
			case "1":
				m = config.ModeRule
			case "2":
				m = config.ModeGlobal
			case "3":
				m = config.ModeDirect
			default:
				warnC.Fprintln(c.out, "取消")
				continue
			}
			if err := c.app.SetMode(ctx, m); err != nil {
				badC.Fprintf(c.out, "应用失败: %v\n", err)
			} else {
				okC.Fprintf(c.out, "已切换到 %s 模式\n", m)
			}
		case "2":
			// TUN 是让流量真正走代理的关键；关之前警告一下。
			if c.app.Cfg.Gateway.TUN.Enabled {
				warnC.Fprintln(c.out, "\n⚠ 关闭 TUN 的后果：")
				fmt.Fprintln(c.out, "  Switch / PS5 / Apple TV / 智能电视")
				fmt.Fprintln(c.out, "  即使改了网关指向本机，流量也只会被普通路由转发，")
				fmt.Fprintln(c.out, "  【不会走代理】，跟没开网关一样被墙。")
				fmt.Fprintln(c.out, "  只有手机/电脑等可以手动填代理服务器的设备才能关 TUN。")
				if !c.yesNo("确定要关闭 TUN？", false) {
					continue
				}
			}
			if err := c.app.ToggleTUN(ctx); err != nil {
				badC.Fprintln(c.out, err.Error())
			} else {
				okC.Fprintf(c.out, "TUN 已 %s\n", onOff(c.app.Cfg.Gateway.TUN.Enabled))
			}
		case "3":
			if err := c.app.ToggleAdblock(ctx); err != nil {
				badC.Fprintln(c.out, err.Error())
			}
		case "4":
			c.screenCustomRules(ctx)
		case "5":
			// 策略组自动补全：只影响下一次 render。订阅里类型是 url-test /
			// fallback 的组已经在则不动；两个都不在时补 Auto + Fallback，
			// 引用订阅里全部节点。reload 后 mihomo Web UI 就能看到新组。
			c.app.Cfg.Traffic.AutoGroups = !c.app.Cfg.Traffic.AutoGroups
			c.saveAndMaybeReload(ctx, fmt.Sprintf("策略组自动补全已 %s", onOff(c.app.Cfg.Traffic.AutoGroups)))
		case "9":
			c.screenTrafficAdvanced(ctx)
		case "0", "q", "Q", "":
			return
		default:
			warnC.Fprintln(c.out, "无效选项，按 0 或 Q 返回主菜单")
		}
	}
}

// screenCustomRules 管理用户自定义规则（config.Traffic.Extras 的 Direct/Proxy/Reject）。
// 这些规则在 traffic.Render 里**最先**被 emit，优先级高过所有内置 ruleset。
func (c *consoleUI) screenCustomRules(ctx context.Context) {
	type item struct {
		verdict string // DIRECT / PROXY / REJECT
		rule    string
	}
	for {
		c.banner("自定义规则")
		ex := &c.app.Cfg.Traffic.Extras
		dimC.Fprintln(c.out, "  规则越靠上优先级越高。mihomo 先扫自定义，再扫内置 china_direct / adblock 等。")
		dimC.Fprintln(c.out, "  常见类型：DOMAIN-SUFFIX / DOMAIN / DOMAIN-KEYWORD / IP-CIDR / PROCESS-NAME")
		fmt.Fprintln(c.out)

		var listed []item
		dump := func(group string, list []string, verdict string) {
			titleC.Fprintf(c.out, "  [%s] (%d)\n", group, len(list))
			if len(list) == 0 {
				dimC.Fprintln(c.out, "    (无)")
				return
			}
			for _, r := range list {
				listed = append(listed, item{verdict, r})
				fmt.Fprintf(c.out, "    %2d  %s\n", len(listed), r)
			}
		}
		dump("直连", ex.Direct, "DIRECT")
		dump("走代理", ex.Proxy, "PROXY")
		dump("拒绝", ex.Reject, "REJECT")

		fmt.Fprintln(c.out)
		titleC.Fprint(c.out, "  ── 操作 ── ")
		fmt.Fprintln(c.out, "A 添加一条   D <编号> 删除某条   0 返回（或按 Q）")
		input := strings.ToLower(strings.TrimSpace(c.prompt("选择：> ")))

		switch {
		case input == "" || input == "0" || input == "q":
			return
		case input == "a":
			c.addCustomRule(ctx)
		case strings.HasPrefix(input, "d"):
			// 兼容 "d 3" 和 "d3"
			numStr := strings.TrimSpace(strings.TrimPrefix(input, "d"))
			idx, err := strconv.Atoi(numStr)
			if err != nil || idx < 1 || idx > len(listed) {
				warnC.Fprintln(c.out, "无效编号（格式: d 3 或 d3）")
				continue
			}
			target := listed[idx-1]
			c.deleteCustomRule(ctx, target.verdict, target.rule)
		default:
			warnC.Fprintln(c.out, "无效操作（A 添加 / D <编号> 删除 / 0 返回）")
		}
	}
}

// addCustomRule 引导式添加一条自定义规则。
func (c *consoleUI) addCustomRule(ctx context.Context) {
	fmt.Fprintln(c.out, "\n  匹配类型：")
	fmt.Fprintln(c.out, "    1) DOMAIN-SUFFIX      xx.com 以及所有子域（最常用）")
	fmt.Fprintln(c.out, "    2) DOMAIN              完整域名精确匹配")
	fmt.Fprintln(c.out, "    3) DOMAIN-KEYWORD      包含关键字的域名")
	fmt.Fprintln(c.out, "    4) IP-CIDR             1.2.3.0/24 这种网段")
	fmt.Fprintln(c.out, "    5) PROCESS-NAME        按本机进程名匹配（如 Cursor）")
	fmt.Fprintln(c.out, "    6) 手写完整规则        自己拼 TYPE,TARGET[,modifier]")
	kindChoice := c.ask("请选择 1-6", "1")
	kindMap := map[string]string{
		"1": "DOMAIN-SUFFIX",
		"2": "DOMAIN",
		"3": "DOMAIN-KEYWORD",
		"4": "IP-CIDR",
		"5": "PROCESS-NAME",
	}
	var rule string
	if kind, ok := kindMap[kindChoice]; ok {
		target := strings.TrimSpace(c.ask(fmt.Sprintf("  %s 的匹配目标", kind), ""))
		if target == "" {
			warnC.Fprintln(c.out, "  匹配目标为空，取消")
			return
		}
		rule = kind + "," + target
	} else {
		rule = strings.TrimSpace(c.ask("  完整规则（不带 verdict）", ""))
		if rule == "" {
			warnC.Fprintln(c.out, "  规则为空，取消")
			return
		}
	}

	fmt.Fprintln(c.out, "\n  命中后去向：")
	fmt.Fprintln(c.out, "    1) 直连 DIRECT")
	fmt.Fprintln(c.out, "    2) 走代理 Proxy")
	fmt.Fprintln(c.out, "    3) 拒绝 REJECT")
	verdict := c.ask("请选择 1-3", "2")

	ex := &c.app.Cfg.Traffic.Extras
	var label string
	switch verdict {
	case "1":
		ex.Direct = append(ex.Direct, rule)
		label = "DIRECT"
	case "3":
		ex.Reject = append(ex.Reject, rule)
		label = "REJECT"
	default:
		ex.Proxy = append(ex.Proxy, rule)
		label = "Proxy"
	}
	c.saveAndMaybeReload(ctx, fmt.Sprintf("  ✓ 已加规则：%s → %s", rule, label))
}

// deleteCustomRule 删除命中的某条。
func (c *consoleUI) deleteCustomRule(ctx context.Context, verdict, rule string) {
	remove := func(list []string) []string {
		out := list[:0]
		removed := false
		for _, r := range list {
			if !removed && r == rule {
				removed = true
				continue
			}
			out = append(out, r)
		}
		return out
	}
	ex := &c.app.Cfg.Traffic.Extras
	switch verdict {
	case "DIRECT":
		ex.Direct = remove(ex.Direct)
	case "PROXY":
		ex.Proxy = remove(ex.Proxy)
	case "REJECT":
		ex.Reject = remove(ex.Reject)
	}
	c.saveAndMaybeReload(ctx, fmt.Sprintf("  ✓ 已删：%s", rule))
}

// screenTrafficAdvanced 收纳普通用户基本不需要碰的开关：DNS 代理开关、
// DNS 监听端口、mixed 端口、API 端口。99% 场景下来这里只是为了解端口冲突。
func (c *consoleUI) screenTrafficAdvanced(ctx context.Context) {
	for {
		c.banner("分流 & 规则 · 高级（端口冲突时才来）")
		cfg := c.app.Cfg
		fmt.Fprintf(c.out, "  DNS 代理 %s (端口 %d)  ·  mixed %d  ·  API %d\n\n",
			onOff(cfg.Gateway.DNS.Enabled), cfg.Gateway.DNS.Port,
			cfg.Runtime.Ports.Mixed, cfg.Runtime.Ports.API)
		fmt.Fprintln(c.out, "  1  开关 DNS 代理        （本机 53 端口被 Clash Verge 等占用时关掉）")
		fmt.Fprintln(c.out, "  2  修改 DNS 监听端口    （默认 53；改了 LAN 设备就基本解析不了，不建议动）")
		fmt.Fprintln(c.out, "  3  修改 mixed 端口      （HTTP+SOCKS5，默认 17890，避开 Clash 7890；也是局域网代理端口）")
		fmt.Fprintln(c.out, "  4  修改 API 端口        （默认 19090，避开 Clash 9090；Web 控制台也走这个）")
		fmt.Fprintln(c.out)
		titleC.Fprintln(c.out, "  ── 操作 ── 0 返回（或按 Q）")
		switch c.prompt("选择：> ") {
		case "1":
			// 关 DNS 是有重大后果的操作，必须讲清楚 LAN 设备会变啥样。
			if c.app.Cfg.Gateway.DNS.Enabled {
				warnC.Fprintln(c.out, "\n⚠ 关闭 DNS 代理的影响：")
				fmt.Fprintln(c.out, "  • LAN 设备（Switch/PS5 等）如果把 DNS 指向本机 IP")
				fmt.Fprintln(c.out, "    → 它们会完全【连不上网】（域名解析失败）")
				fmt.Fprintln(c.out, "  • 本机 TUN 模式的 fake-ip 机制也会失效")
				fmt.Fprintln(c.out, "    → 没有假 IP，TUN auto-route 的劫持可能不全面")
				fmt.Fprintln(c.out)
				fmt.Fprintln(c.out, "什么时候该关？")
				fmt.Fprintln(c.out, "  1) 本机已有别的进程占用 53 端口（比如已开着 Clash Verge）")
				fmt.Fprintln(c.out, "     这种情况下：LAN 设备的 DNS 还是指向本机 IP 即可，")
				fmt.Fprintln(c.out, "     端口 53 上的那个进程会接管回答。")
				fmt.Fprintln(c.out, "  2) 不想让本机做 DNS，希望设备自己用路由器/公共 DNS")
				fmt.Fprintln(c.out, "     这种情况下：LAN 设备要单独设一个能用的 DNS")
				fmt.Fprintln(c.out, "     （如 114.114.114.114 / 路由器 IP），不能指向本机。")
				fmt.Fprintln(c.out)
				if !c.yesNo("确定要关闭 DNS 代理？", false) {
					continue
				}
			} else {
				fmt.Fprintln(c.out, "\n启用 DNS 代理后，LAN 设备可以直接把 DNS 指向本机 IP。")
				if !c.yesNo("确定要启用 DNS 代理？", true) {
					continue
				}
			}
			c.app.Cfg.Gateway.DNS.Enabled = !c.app.Cfg.Gateway.DNS.Enabled
			c.saveAndMaybeReload(ctx, fmt.Sprintf("DNS 已 %s", onOff(c.app.Cfg.Gateway.DNS.Enabled)))
		case "2":
			c.promptPort(ctx, "DNS 监听端口", &c.app.Cfg.Gateway.DNS.Port)
		case "3":
			c.promptPort(ctx, "mixed 端口", &c.app.Cfg.Runtime.Ports.Mixed)
		case "4":
			c.promptPort(ctx, "API 端口", &c.app.Cfg.Runtime.Ports.API)
		case "0", "q", "Q", "":
			return
		default:
			warnC.Fprintln(c.out, "无效选项，按 0 或 Q 返回")
		}
	}
}

// promptPort asks for a new port, writes it back, saves, hot-reloads if running.
// If the CURRENT port is already occupied by someone else, we flag it up front
// so the user doesn't accidentally press Enter and keep a known-broken value.
func (c *consoleUI) promptPort(ctx context.Context, label string, target *int) {
	// Probe current port so the prompt can warn the user.
	// Note: engine.Running() skips the probe since mihomo would legitimately hold the port.
	if !c.app.Engine.Running() {
		if occupied, owner := probePort(*target); occupied {
			c.warnOccupied(label+" = "+strconv.Itoa(*target), owner)
		}
	}
	raw := c.ask(label, strconv.Itoa(*target))
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v <= 0 || v > 65535 {
		warnC.Fprintln(c.out, "端口无效（1-65535），已忽略")
		return
	}
	// Don't save a port we know is dead (only probe new ports we haven't already warned about).
	if v != *target && !c.app.Engine.Running() {
		if occupied, owner := probePort(v); occupied {
			c.warnOccupied(strconv.Itoa(v), owner)
			if !c.yesNo("仍要保存？", false) {
				return
			}
		}
	}
	*target = v
	c.saveAndMaybeReload(ctx, fmt.Sprintf("%s 已改为 %d", label, v))
}

// warnOccupied prints a pretty warning; owner may be nil when lsof can't see
// the holder (common for root-owned processes probed from a non-root shell).
func (c *consoleUI) warnOccupied(what string, owner *engine.PortOwner) {
	if owner != nil {
		warnC.Fprintf(c.out, "  ⚠ %s 已被占用（%s, PID %d）\n", what, owner.Name, owner.PID)
	} else {
		warnC.Fprintf(c.out, "  ⚠ %s 已被占用（可能是 root 进程，当前用户看不到 PID，试试 sudo gateway）\n", what)
	}
}

// probePort tests whether a TCP port is occupied. The bool is the ground truth;
// owner is best-effort and may be nil even when occupied.
func probePort(port int) (occupied bool, owner *engine.PortOwner) {
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err == nil {
		_ = ln.Close()
		return false, nil
	}
	return true, engine.LookupPortOwner(port)
}

// saveAndMaybeReload persists the in-memory config and, if mihomo is running,
// asks it to reload. Prints a one-line status line on completion.
func (c *consoleUI) saveAndMaybeReload(ctx context.Context, okMsg string) {
	if err := c.app.Save(); err != nil {
		badC.Fprintln(c.out, err.Error())
		return
	}
	if c.app.Engine != nil && c.app.Engine.Running() {
		if err := c.app.Engine.Reload(ctx, c.app.Cfg); err != nil {
			warnC.Fprintf(c.out, "已保存但热重载失败：%v\n", err)
			return
		}
	}
	okC.Fprintln(c.out, okMsg)
}

func (c *consoleUI) screenSource(ctx context.Context) {
	// 进菜单先并发探测一次。后续只有按 T 或改了配置才重新探测，
	// 避免每次菜单循环都去重复测（订阅 URL 能慢到 5-10 秒）。
	probes := c.probeAllSources(ctx, c.app.Cfg)
	for {
		c.banner("代理 & 订阅  ·  当前: " + sourceLabel(c.app.Cfg.Source.Type))

		// 代理源选项：编号 / 标签 / 图标 / 值 四列对齐（按显示宽度，不是字节）
		renderRow := func(num, label string, p sourceSlot) {
			iconStr := p.icon()
			if iconStr == "" {
				iconStr = dimC.Sprint("·")
			}
			fmt.Fprintf(c.out, "    %s  %s  %s  %s\n",
				num, padRightWide(label, 14), iconStr, p.value)
		}
		renderRow("1", "单点代理", probes.single)
		renderRow("2", "机场订阅", probes.subscription)
		renderRow("3", "本地配置文件", probes.file)
		renderRow("4", "暂不配置", sourceSlot{value: dimC.Sprint("全部走直连")})

		// Web 控制台是 mihomo 自带的，跟源类型无关：只要 mihomo 在跑，UI 就能
		// 访问（单点代理 / 订阅 / 本地文件一视同仁）。过去只对 subscription+file
		// 显示，导致选「单点代理」的人看不到 URL，误以为没这功能。
		if c.app.Engine != nil && c.app.Engine.Running() {
			apiPort := c.app.Cfg.Runtime.Ports.API
			localIP := c.app.Status().Gateway.LocalIP
			probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			var wg sync.WaitGroup
			var markLocal, markLAN string
			wg.Add(2)
			go func() {
				defer wg.Done()
				markLocal = probeHTTP(probeCtx, fmt.Sprintf("http://127.0.0.1:%d/ui/", apiPort))
			}()
			go func() {
				defer wg.Done()
				if localIP == "" {
					markLAN = ""
					return
				}
				markLAN = probeHTTP(probeCtx, fmt.Sprintf("http://%s:%d/ui/", localIP, apiPort))
			}()
			wg.Wait()
			cancel()

			fmt.Fprintln(c.out)
			titleC.Fprintln(c.out, "  ── Web 控制台（浏览器打开，切节点 / 查流量 / 改规则）──")
			urlLocal := fmt.Sprintf("http://127.0.0.1:%d/ui/", apiPort)
			fmt.Fprintf(c.out, "    本机    %-36s  %s\n", urlLocal, markLocal)
			if localIP != "" {
				urlLAN := fmt.Sprintf("http://%s:%d/ui/", localIP, apiPort)
				fmt.Fprintf(c.out, "    局域网  %-36s  %s  %s\n", urlLAN, markLAN, dimC.Sprint("（手机 / 平板也能用）"))
			}
		}

		// 操作按键：放最下面贴近 prompt，和其它页面一致
		fmt.Fprintln(c.out)
		titleC.Fprint(c.out, "  ── 操作 ── ")
		ops := []string{}
		if c.app.Cfg.Source.Type == config.SourceTypeSubscription ||
			c.app.Cfg.Source.Type == config.SourceTypeFile {
			ops = append(ops, "N 切换节点")
		}
		scriptMark := ""
		if c.app.Cfg.Source.ChainResidential != nil || c.app.Cfg.Source.ScriptPath != "" {
			scriptMark = okC.Sprint(" ●")
		}
		ops = append(ops, "S 全局扩展脚本"+scriptMark, "T 重新测试", "0 返回（或按 Q）")
		fmt.Fprintln(c.out, strings.Join(ops, "   "))

		choice := strings.ToLower(c.prompt("选择：> "))
		before := c.app.Cfg.Source
		switch choice {
		case "1":
			c.configureSingle()
		case "2":
			c.configureSubscription()
		case "3":
			c.configureFile()
		case "4":
			c.app.Cfg.Source.Type = config.SourceTypeNone
		case "n":
			c.screenSwitchNode(ctx)
			continue
		case "s":
			c.configureScript()
		case "t":
			probes = c.probeAllSources(ctx, c.app.Cfg)
			continue
		case "0", "q", "":
			return
		default:
			warnC.Fprintln(c.out, "无效选项，按 0 或 Q 返回")
			continue
		}
		changed := !reflect.DeepEqual(before, c.app.Cfg.Source)
		if !changed {
			continue
		}
		if err := c.app.Save(); err != nil {
			badC.Fprintln(c.out, err.Error())
			continue
		}
		if c.app.Engine != nil && c.app.Engine.Running() {
			if err := c.app.Engine.Reload(ctx, c.app.Cfg); err != nil {
				warnC.Fprintf(c.out, "热重载失败，下次启动生效: %v\n", err)
			} else {
				okC.Fprintln(c.out, "已热重载")
			}
		} else {
			okC.Fprintln(c.out, "已保存（下次 start 生效）")
		}
		// 配置改了就顺手重测一次
		probes = c.probeAllSources(ctx, c.app.Cfg)
	}
}

// sourceSlot 是单个代理源在「换代理源」菜单里的一行数据。
// icon() 负责 ✓/✗/· 三态；value 是人读的配置摘要（已做长度裁剪）。
type sourceSlot struct {
	value string // "127.0.0.1:6578 socks5" / "~/Documents/clash/long.yaml" / "(未配置)"
	err   error  // nil=可达；非 nil=探测失败
	empty bool   // true=未配置，不测，图标 ·
}

func (s sourceSlot) icon() string {
	if s.empty {
		return dimC.Sprint("·")
	}
	if s.err == nil {
		return okC.Sprint("✓")
	}
	return badC.Sprint("✗")
}

// sourceProbes 是三类可配置源的并发探测结果。
type sourceProbes struct {
	single, subscription, file sourceSlot
}

// probeAllSources 并发探测 external/remote、subscription、file。
// 5 秒 hard deadline 包干，避免订阅慢拖住菜单。
// 探测期间临时在 stdout 打一行「测试中…」做视觉反馈。
func (c *consoleUI) probeAllSources(ctx context.Context, cfg *config.Config) sourceProbes {
	dimC.Fprintln(c.out, "  测试中……")
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var r sourceProbes
	testOpts := c.sourceTestOptions()
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); r.single = probeSingle(probeCtx, cfg) }()
	go func() { defer wg.Done(); r.subscription = probeSubscription(probeCtx, cfg, testOpts) }()
	go func() { defer wg.Done(); r.file = probeFile(cfg) }()
	wg.Wait()
	return r
}

// probeSingle 优先看 Remote（有认证那个），没有就看 External。
// 两个都空 → 未配置。
func probeSingle(ctx context.Context, cfg *config.Config) sourceSlot {
	switch {
	case cfg.Source.Remote.Server != "" && cfg.Source.Remote.Port > 0:
		r := cfg.Source.Remote
		sum := fmt.Sprintf("%s:%d %s", r.Server, r.Port, r.Kind)
		if r.Username != "" {
			sum += " 带认证"
		}
		return sourceSlot{value: sum, err: source.Test(ctx, config.SourceConfig{Type: config.SourceTypeRemote, Remote: r})}
	case cfg.Source.External.Server != "" && cfg.Source.External.Port > 0:
		e := cfg.Source.External
		sum := fmt.Sprintf("%s:%d %s", e.Server, e.Port, e.Kind)
		return sourceSlot{value: sum, err: source.Test(ctx, config.SourceConfig{Type: config.SourceTypeExternal, External: e})}
	}
	return sourceSlot{value: dimC.Sprint("(未配置)"), empty: true}
}

func probeSubscription(ctx context.Context, cfg *config.Config, opts source.TestOptions) sourceSlot {
	s := cfg.Source.Subscription
	if s.URL == "" {
		return sourceSlot{value: dimC.Sprint("(未配置)"), empty: true}
	}
	return sourceSlot{
		value: truncateMiddle(s.URL, 50),
		err:   source.TestWithOptions(ctx, config.SourceConfig{Type: config.SourceTypeSubscription, Subscription: s}, opts),
	}
}

func probeFile(cfg *config.Config) sourceSlot {
	p := cfg.Source.File.Path
	if p == "" {
		return sourceSlot{value: dimC.Sprint("(未配置)"), empty: true}
	}
	return sourceSlot{
		value: homeAbbrev(p),
		err:   source.Test(context.Background(), config.SourceConfig{Type: config.SourceTypeFile, File: cfg.Source.File}),
	}
}

// homeAbbrev 把绝对路径里的 $HOME 替换成 ~，长度仍过长再 middle-truncate 保住文件名。
func homeAbbrev(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home) {
		p = "~" + p[len(home):]
	}
	return truncateMiddle(p, 50)
}

// truncateMiddle 过长时保留头尾，中间用 … 缩略（小白看得见盘根和文件名）。
func truncateMiddle(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n < 6 {
		return s[:n-1] + "…"
	}
	head := n / 2
	tail := n - head - 1
	return s[:head] + "…" + s[len(s)-tail:]
}

// probeHTTP 做一次 GET 并把结果格式化成 ✓ 200 / ⚠ HTTP 404 / ✗ 连接失败…
// 给 Web 控制台地址的连通性提示用。
func probeHTTP(ctx context.Context, url string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return badC.Sprint("✗ " + err.Error())
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		msg := err.Error()
		if len(msg) > 50 {
			msg = msg[:47] + "…"
		}
		return badC.Sprint("✗ " + msg)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == 200:
		return okC.Sprint("✓")
	case resp.StatusCode == 404:
		return warnC.Sprint("⚠ 404")
	default:
		return badC.Sprintf("✗ %d", resp.StatusCode)
	}
}

// probeMark 把 error 格式化成 ✓ / ✗ 简要原因，不超过 40 字。
func probeMark(err error) string {
	if err == nil {
		return okC.Sprint("✓")
	}
	msg := err.Error()
	if len(msg) > 40 {
		msg = msg[:37] + "…"
	}
	return badC.Sprint("✗ " + msg)
}

// truncate 把字符串按字节数裁到 n，溢出加 …。
// 英文路径/URL 场景下字节 ≈ 字符，不用处理宽字符。
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return s[:n-1] + "…"
}

// padRight 把字符串右填空格到指定宽度（按字节，仅 ASCII 用）。
func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

// displayWidth 按等宽终端显示列数算字符串宽度。
// CJK / 日韩 / 全角 / Emoji 一律 2 列，ASCII / 普通符号 1 列。
// fmt 的 %-Ns 按字节，中文一字 3 字节占 2 列，导致对齐错位，这个函数补正。
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		switch {
		case r < 0x80:
			w++
		case r >= 0x1100 && r <= 0x115F, // Hangul jamo
			r >= 0x2E80 && r <= 0x303E,   // CJK 符号 / 部首
			r >= 0x3041 && r <= 0x33FF,   // Hiragana / Katakana
			r >= 0x3400 && r <= 0x9FFF,   // CJK Unified Ideographs
			r >= 0xAC00 && r <= 0xD7A3,   // Hangul
			r >= 0xF900 && r <= 0xFAFF,   // CJK compat
			r >= 0xFE30 && r <= 0xFE4F,   // CJK compat forms
			r >= 0xFF00 && r <= 0xFF60,   // 全角
			r >= 0xFFE0 && r <= 0xFFE6,   // 全角符号
			r >= 0x1F000 && r <= 0x1FFFF: // Emoji / supplementary
			w += 2
		default:
			w++
		}
	}
	return w
}

// padRightWide 按显示宽度右填空格。
func padRightWide(s string, cols int) string {
	need := cols - displayWidth(s)
	if need <= 0 {
		return s
	}
	return s + strings.Repeat(" ", need)
}

const nodeListPreviewLimit = 20

type nodeListMode int

const (
	nodeListPreview nodeListMode = iota
	nodeListCompact
	nodeListVerbose
)

func sortProxyNodes(all []string, delays map[string]int) []string {
	sorted := append([]string(nil), all...)
	sort.SliceStable(sorted, func(i, j int) bool {
		di, dj := delays[sorted[i]], delays[sorted[j]]
		if di == 0 && dj == 0 {
			return false
		}
		if di == 0 {
			return false
		}
		if dj == 0 {
			return true
		}
		return di < dj
	})
	return sorted
}

func renderProxyNodeList(w io.Writer, sorted []string, current string, delays map[string]int, mode nodeListMode) {
	displayed := sorted
	if mode == nodeListPreview && len(displayed) > nodeListPreviewLimit {
		displayed = displayed[:nodeListPreviewLimit]
	}
	switch mode {
	case nodeListVerbose:
		// ll：单列详细表格（# / 当前 / 延迟 / 节点），便于一眼看清每个节点状态
		fmt.Fprintf(w, "  %3s  %-4s  %-8s  %s\n", "#", "当前", "延迟", "节点")
		for i, n := range displayed {
			currentText := ""
			if n == current {
				currentText = "当前"
			}
			fmt.Fprintf(w, "  %3d  %-4s  %s  %s\n",
				i+1, currentText, delayLabel(delays[n]), n)
		}
	case nodeListCompact:
		// ls：Linux `ls` 风格多列网格（列优先：第一列装延迟最低的几个）
		renderCompactNodeGrid(w, displayed, current, delays)
	default:
		// preview：首屏前 20 个单列概览，用户一眼能看到编号→名字→延迟对齐
		for i, n := range displayed {
			mark := "  "
			if n == current {
				mark = "✓ "
			}
			fmt.Fprintf(w, "  %2d  %s%s  %s\n",
				i+1, mark, padRightWide(n, 30), delayLabel(delays[n]))
		}
	}
}

// renderCompactNodeGrid 按 Linux `ls` 风格把节点紧凑铺成多列网格。
//   - 列优先（column-major）：第一列装前 rows 个最快的，读起来跟 `ls` 一致；
//   - 单元格宽度 = 最宽一项 + 2 空格间距，按终端宽度算列数，CJK/emoji 算 2 列；
//   - 延迟带颜色时打印彩色串，但对齐按无色宽度来，不会被 ANSI 序列撑歪。
func renderCompactNodeGrid(w io.Writer, sorted []string, current string, delays map[string]int) {
	if len(sorted) == 0 {
		return
	}
	plain := make([]string, len(sorted))
	colored := make([]string, len(sorted))
	maxW := 0
	for i, n := range sorted {
		mark := " "
		if n == current {
			mark = "✓"
		}
		plain[i] = fmt.Sprintf("%2d%s %s  %s", i+1, mark, n, delayPlain(delays[n]))
		colored[i] = fmt.Sprintf("%2d%s %s  %s", i+1, mark, n, delayLabel(delays[n]))
		if dw := displayWidth(plain[i]); dw > maxW {
			maxW = dw
		}
	}
	const (
		gap    = 2 // 列与列之间的空格数
		indent = 2 // 整块左侧缩进
	)
	cellW := maxW + gap
	avail := terminalWidth() - indent
	cols := avail / cellW
	if cols < 1 {
		cols = 1
	}
	if cols > len(sorted) {
		cols = len(sorted)
	}
	rows := (len(sorted) + cols - 1) / cols
	for r := 0; r < rows; r++ {
		fmt.Fprint(w, "  ")
		for c := 0; c < cols; c++ {
			idx := c*rows + r
			if idx >= len(sorted) {
				break
			}
			fmt.Fprint(w, colored[idx])
			// 末列不补 padding，避免在行尾多出一段空白
			if c < cols-1 && c*rows+r+rows < len(sorted) {
				pad := cellW - displayWidth(plain[idx])
				if pad < 0 {
					pad = 0
				}
				fmt.Fprint(w, strings.Repeat(" ", pad))
			}
		}
		fmt.Fprintln(w)
	}
}

// delayPlain 是 delayLabel 的无色版，仅用于计算对齐宽度。
func delayPlain(ms int) string {
	if ms <= 0 {
		return "—"
	}
	return fmt.Sprintf("%4d ms", ms)
}

// terminalWidth 尽量拿到当前终端宽度。拿不到时退回 80 列的保守值。
// 这里不引入新依赖，优先读 COLUMNS 环境变量（大多数交互 shell 会设）。
func terminalWidth() int {
	if s := os.Getenv("COLUMNS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return 80
}

// screenSwitchNode 让用户在 mihomo 当前加载的 proxy-groups 里挑分组、挑节点。
// 依赖 mihomo API 在跑；不在跑就提示先启动。
func (c *consoleUI) screenSwitchNode(ctx context.Context) {
	if c.app.Engine == nil || !c.app.Engine.Running() {
		warnC.Fprintln(c.out, "\n切换节点需要 mihomo 在跑：先回主菜单 4 启动网关再来")
		c.pause()
		return
	}
	client := c.app.Engine.API()
	for {
		c.banner("切换节点")
		listCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		groups, err := client.ListProxyGroups(listCtx)
		cancel()
		if err != nil {
			badC.Fprintf(c.out, "拉取分组失败：%v\n", err)
			c.pause()
			return
		}
		if len(groups) == 0 {
			warnC.Fprintln(c.out, "没有可切换的分组（当前源只提供了单个 Proxy 组，不用切）")
			c.pause()
			return
		}
		for i, g := range groups {
			fmt.Fprintf(c.out, "  %2d  %s  当前: %s  (%d 节点)\n",
				i+1,
				padRightWide(g.Name, 24),
				padRightWide(g.Now, 20),
				len(g.All))
		}
		fmt.Fprintln(c.out)
		titleC.Fprintln(c.out, "  ── 操作 ── <编号> 进分组选节点   0 返回（或按 Q）")
		input := strings.ToLower(c.prompt("选择：> "))
		if input == "0" || input == "" || input == "q" {
			return
		}
		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(groups) {
			warnC.Fprintln(c.out, "无效选项")
			continue
		}
		c.screenSwitchNodeInGroup(ctx, groups[idx-1])
	}
}

// screenSwitchNodeInGroup 展示一个分组里的所有节点，带延迟测速和排序。
// 进入时自动对整组测一遍，按延迟升序排列；用户可按 R 再测、按数字选节点。
func (c *consoleUI) screenSwitchNodeInGroup(ctx context.Context, g engine.ProxyGroup) {
	delays := map[string]int{}
	testFailed := false
	listMode := nodeListPreview

	runDelay := func() {
		testCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		d, err := c.app.Engine.API().GroupDelay(testCtx, g.Name,
			"http://www.gstatic.com/generate_204", 3000)
		if err != nil {
			testFailed = true
			return
		}
		testFailed = false
		delays = d
	}

	// 首次进入先同步测一遍
	dimC.Fprintln(c.out, "\n  测速中……")
	runDelay()

	for {
		c.banner(fmt.Sprintf("分组：%s  (当前：%s)", g.Name, g.Now))

		sorted := sortProxyNodes(g.All, delays)

		if testFailed {
			warnC.Fprintln(c.out, "  ⚠ 上一次整组测速失败（可能 mihomo API 版本不支持或网络问题）")
		}
		renderProxyNodeList(c.out, sorted, g.Now, delays, listMode)
		if listMode == nodeListPreview && len(sorted) > nodeListPreviewLimit {
			dimC.Fprintf(c.out, "  共 %d 个节点，当前只显示前 %d 个；输入 ls 看完整列表，ll 看详细列表。\n",
				len(sorted), nodeListPreviewLimit)
		}

		fmt.Fprintln(c.out)
		titleC.Fprint(c.out, "  ── 操作 ── ")
		fmt.Fprintln(c.out, "R 刷新测速   ls/ll 列表   <编号> 切到该节点   0 返回（或按 Q）")
		input := strings.ToLower(strings.TrimSpace(c.prompt("选择：> ")))
		switch {
		case input == "" || input == "0" || input == "q":
			return
		case input == "r":
			dimC.Fprintln(c.out, "  测速中……")
			runDelay()
		case input == "ls":
			listMode = nodeListCompact
		case input == "ll":
			listMode = nodeListVerbose
		default:
			idx, err := strconv.Atoi(input)
			if err != nil || idx < 1 || idx > len(sorted) {
				warnC.Fprintln(c.out, "无效选项（按数字选节点 / ls / ll / R 刷新 / 0 返回）")
				continue
			}
			node := sorted[idx-1]
			ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
			if err := c.app.Engine.API().SelectNode(ctx2, g.Name, node); err != nil {
				cancel()
				badC.Fprintf(c.out, "切换失败：%v\n", err)
			} else {
				cancel()
				okC.Fprintf(c.out, "已切换 %s → %s\n", g.Name, node)
				g.Now = node
			}
		}
	}
}

func (c *consoleUI) sourceTestOptions() source.TestOptions {
	if c.app == nil || c.app.Engine == nil || !c.app.Engine.Running() {
		return source.TestOptions{}
	}
	return source.TestOptions{
		SubscriptionProxyURL: source.LocalMixedProxyURL(c.app.Cfg.Runtime.Ports.Mixed),
	}
}

// delayLabel 把毫秒格式化成带颜色的 "234 ms" / "超时" / "—"。
func delayLabel(ms int) string {
	switch {
	case ms <= 0:
		return dimC.Sprint("—")
	case ms < 300:
		return okC.Sprintf("%4d ms", ms)
	case ms < 1000:
		return warnC.Sprintf("%4d ms", ms)
	default:
		return badC.Sprintf("%4d ms", ms)
	}
}

func (c *consoleUI) screenLifecycle(ctx context.Context) {
	c.banner("启动 / 重启 / 停止")
	s := c.app.Status()
	runStatus := dimC.Sprint("○ 未启动")
	if s.Running {
		runStatus = okC.Sprint("● 运行中")
	}
	fmt.Fprintf(c.out, "  当前状态: %s\n\n", runStatus)
	fmt.Fprintln(c.out, "  1  启动")
	fmt.Fprintln(c.out, "  2  重启              （等同停止 + 启动，让新配置完整生效）")
	fmt.Fprintln(c.out, "  3  停止              （mihomo 结束，LAN 设备走直连）")
	fmt.Fprintln(c.out, "  4  清理残留 mihomo   （端口被占用时用，会杀掉系统里所有 mihomo 进程）")
	fmt.Fprintln(c.out)
	titleC.Fprintln(c.out, "  ── 操作 ── 0 返回主菜单（或按 Q）")
	admin, _ := c.app.Plat.IsAdmin()
	if !admin {
		// 运行中时重启/停止只是拉/杀用户态 mihomo，不需要 sudo；只有「首次启动」
		// 才要改系统状态（IP 转发、TUN 设备）。按是否在运行给两种语气的提示，
		// 避免旧文案把「启动/停止/清理全都要 sudo」这条过度告警给误导用户。
		if s.Running {
			if loopback, _ := c.app.Plat.LocalDNSIsLoopback(); loopback {
				warnC.Fprintln(c.out, "  （本机 DNS 指向 127.0.0.1；停止会恢复 DNS，建议用 sudo gateway stop）")
			} else {
				dimC.Fprintln(c.out, "  （未用 sudo：已在运行，重启/停止通常不需要 sudo；切 TUN 或绑 53 端口才需要）")
			}
		} else {
			warnC.Fprintln(c.out, "  （未用 sudo 运行：启动需要 sudo，请退出后运行 sudo gateway）")
		}
	}
	switch c.prompt("选择：> ") {
	case "1":
		if !admin {
			badC.Fprintln(c.out, "需要 sudo 权限。请退出后运行: sudo gateway start")
			return
		}
		c.tryStart(ctx)
	case "2":
		if err := c.app.Engine.Reload(ctx, c.app.Cfg); err != nil {
			badC.Fprintf(c.out, "重启失败: %v\n", err)
		} else {
			okC.Fprintln(c.out, "已重启")
		}
	case "3":
		if err := c.app.Stop(); err != nil {
			badC.Fprintln(c.out, err.Error())
		} else {
			okC.Fprintln(c.out, "已停止，并已检查本机 DNS")
		}
	case "4":
		c.cleanupStaleMihomo()
	case "0", "q", "Q", "":
		return
	default:
		warnC.Fprintln(c.out, "无效选项，按 0 或 Q 返回主菜单")
	}
}

// shutdownGateway stops mihomo (if running) and signals the main loop to exit.
// Returns true when the caller should return out of the menu loop.
func (c *consoleUI) shutdownGateway() bool {
	running := c.app.Engine != nil && c.app.Engine.Running()
	if running {
		warnC.Fprintln(c.out, "\n这会停止 mihomo 并退出控制台，LAN 里指向本机的设备会失去代理。")
		if !c.yesNo("确定要关闭 gateway？", false) {
			return false
		}
		if err := c.app.Stop(); err != nil {
			badC.Fprintf(c.out, "停止失败: %v\n", err)
			return false
		}
		okC.Fprintln(c.out, "已停止 mihomo，并已检查本机 DNS")
	}
	return true
}

// tryStart runs Start() and, on port conflict caused by a stale mihomo, offers
// to kill it and retry in-place. This is the most common recovery path for
// users who ran gateway once, crashed/killed the terminal, and now find the
// port squatted by an orphan process.
func (c *consoleUI) tryStart(ctx context.Context) {
	if c.app.Engine.Running() {
		okC.Fprintln(c.out, "已经在运行，无需再启动（如需应用新配置，选 2 重启 / 热重载）")
		return
	}
	err := c.app.Start(ctx)
	if err == nil {
		okC.Fprintln(c.out, "已启动")
		return
	}
	var pce *engine.PortConflictError
	if errors.As(err, &pce) && pce.HasStaleMihomo() {
		warnC.Fprintln(c.out, err.Error())
		pids := pce.StaleMihomoPIDs()
		fmt.Fprintf(c.out, "\n检测到残留的 mihomo 进程（PID %v），这是上一次运行没退干净留下的。\n", pids)
		if c.yesNo("是否自动干掉它们并重新启动？", true) {
			for _, pid := range pids {
				if kErr := engine.KillPID(pid); kErr != nil {
					badC.Fprintf(c.out, "  ✗ kill PID %d 失败: %v\n", pid, kErr)
				} else {
					okC.Fprintf(c.out, "  ✓ 已终止 PID %d\n", pid)
				}
			}
			// 再试一次
			if err := c.app.Start(ctx); err != nil {
				badC.Fprintf(c.out, "启动仍然失败: %v\n", err)
			} else {
				okC.Fprintln(c.out, "已启动")
			}
			return
		}
	}
	badC.Fprintf(c.out, "启动失败: %v\n", err)
}

// cleanupStaleMihomo finds every mihomo on the host and offers to kill them.
// Works even when no port conflict is known — useful after a hard crash.
func (c *consoleUI) cleanupStaleMihomo() {
	pids := engine.FindStaleMihomoPIDs()
	// Filter out our own child so we don't nuke a running gateway.
	if c.app.Engine != nil && c.app.Engine.Running() {
		ownPID := os.Getpid()
		filtered := pids[:0]
		for _, p := range pids {
			if p != ownPID {
				filtered = append(filtered, p)
			}
		}
		pids = filtered
	}
	if len(pids) == 0 {
		okC.Fprintln(c.out, "没发现残留 mihomo 进程")
		return
	}
	fmt.Fprintf(c.out, "发现 %d 个 mihomo 进程: %v\n", len(pids), pids)
	if !c.yesNo("全部杀掉？", true) {
		dimC.Fprintln(c.out, "取消")
		return
	}
	for _, pid := range pids {
		if err := engine.KillPID(pid); err != nil {
			badC.Fprintf(c.out, "  ✗ PID %d: %v\n", pid, err)
		} else {
			okC.Fprintf(c.out, "  ✓ 已终止 PID %d\n", pid)
		}
	}
}

func (c *consoleUI) screenLogs() {
	path := c.app.Engine.LogPath()
	tailN := 30
	rawMode := false // 默认走易读视图；r 切换回原始 mihomo 行
	for {
		view := "易读视图"
		if rawMode {
			view = "原始视图"
		}
		c.banner(fmt.Sprintf("日志（%s）· 末尾 %d 行 · %s", view, tailN, path))
		if err := c.renderTail(path, tailN, rawMode); err != nil {
			warnC.Fprintf(c.out, "%v\n", err)
		}
		dimC.Fprintln(c.out, "\n  [回车] 刷新   [t] tail 跟随   [r] 切换原始/易读   [数字] 改行数   [q] 返回")
		input := c.prompt("> ")
		switch {
		case input == "":
			// 手动刷新：重绘一遍即可
		case strings.EqualFold(input, "q") || strings.EqualFold(input, "quit"):
			return
		case strings.EqualFold(input, "t") || strings.EqualFold(input, "tail"):
			c.tailFollow(path, tailN, rawMode)
		case strings.EqualFold(input, "r") || strings.EqualFold(input, "raw"):
			rawMode = !rawMode
		default:
			if n, err := strconv.Atoi(input); err == nil && n > 0 {
				tailN = n
			} else {
				warnC.Fprintln(c.out, "无效输入（回车=刷新，t=tail，r=切视图，数字=改行数，q=返回）")
				c.pause()
			}
		}
	}
}

// renderTail 把日志文件的末尾 n 行打到输出。文件不存在时返回友好错误。
// rawMode=false 时每行走 humanizeMihomoLine 翻译成中文简要。
func (c *consoleUI) renderTail(path string, n int, rawMode bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("暂无日志 (%s)", path)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	start := 0
	if len(lines) > n {
		start = len(lines) - n
	}
	if rawMode {
		for _, l := range lines[start:] {
			fmt.Fprintln(c.out, l)
		}
		return nil
	}
	// 易读模式下同一告警每 15 秒重复一次会把末尾几十行刷成一片，走 dedup 折叠
	dd := newLineDeduper(c.out)
	for _, l := range lines[start:] {
		formatted, key, t := humanizeMihomoLineWithKey(l)
		dd.Write(formatted, key, t)
	}
	dd.Flush()
	return nil
}

// tailFollow 进入实时跟随模式：先打印末尾 n 行做上下文，然后轮询文件 size 把
// 新增字节流写到终端，按回车退出。文件被截断（mihomo rotate/重启）时重置 offset。
// rawMode=false 时按行缓冲并逐行 humanize。
func (c *consoleUI) tailFollow(path string, n int, rawMode bool) {
	c.banner("日志 tail · " + path)
	_ = c.renderTail(path, n, rawMode)

	f, err := os.Open(path)
	if err != nil {
		warnC.Fprintf(c.out, "无法打开 %s: %v\n", path, err)
		c.pause()
		return
	}
	defer f.Close()
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		warnC.Fprintf(c.out, "seek 失败: %v\n", err)
		c.pause()
		return
	}
	dimC.Fprintln(c.out, "\n（实时跟踪中，按回车退出）")

	// 等一个回车退出 tail。从 inputCh 读一行（已在后台持续接收），
	// 这样就不会和 startInputLoop 的 reader 争 c.in。
	done := make(chan struct{})
	go func() {
		_ = c.readLine()
		close(done)
	}()

	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	var lastSize int64
	if info, err := f.Stat(); err == nil {
		lastSize = info.Size()
	}
	buf := make([]byte, 8192)
	var pending strings.Builder // 易读模式下用来攒半行
	// 实时段独立一个 deduper：从 renderTail 过渡到实时可能短暂重复打印一条，
	// 容忍一下——比跨段共享 deduper 导致的锁/状态耦合简单。
	var dd *lineDeduper
	if !rawMode {
		dd = newLineDeduper(c.out)
		defer dd.Flush()
	}
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			info, err := f.Stat()
			if err != nil {
				continue
			}
			if info.Size() < lastSize {
				// 文件被截断（rotate / mihomo 重启），从头读
				_, _ = f.Seek(0, io.SeekStart)
				lastSize = 0
				pending.Reset()
				if dd != nil {
					dd.Flush()
				}
			}
			for {
				rn, rerr := f.Read(buf)
				if rn > 0 {
					if rawMode {
						_, _ = c.out.Write(buf[:rn])
					} else {
						pending.Write(buf[:rn])
						// 逐行切出来翻译。最后半行留给下一轮。
						for {
							s := pending.String()
							nl := strings.IndexByte(s, '\n')
							if nl < 0 {
								break
							}
							formatted, key, t := humanizeMihomoLineWithKey(s[:nl])
							dd.Write(formatted, key, t)
							pending.Reset()
							pending.WriteString(s[nl+1:])
						}
					}
				}
				if rerr != nil {
					break
				}
			}
			lastSize = info.Size()
		}
	}
}

func (c *consoleUI) pause() {
	dimC.Fprintln(c.out, "\n按回车返回…")
	_ = c.readLine()
}

// --- util ---

func onOff(v bool) string {
	if v {
		return "开"
	}
	return "关"
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func firstNonZero(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}
