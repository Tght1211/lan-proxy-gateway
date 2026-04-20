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
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/engine"
	"github.com/tght/lan-proxy-gateway/internal/gateway"
	"github.com/tght/lan-proxy-gateway/internal/source"
)

// Run is the entry point. It blocks until the user exits.
func Run(ctx context.Context, a *app.App) error {
	c := newConsole(a, os.Stdin, os.Stdout)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	c.startInputLoop(runCtx)
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

	// inputCh 是 stdin 的异步 channel：一个后台 goroutine 长期 ReadString 推这里。
	// 打开后所有交互（prompt/ask/yesNo/pause/tail）都从 channel 读，不再直接碰
	// c.in。这样主菜单可以 select ticker + inputCh，实现「后台告警变化立即重绘」。
	//
	// inputCh 在 Run 入口启动，console 退出时 close。没启动的场景（测试等）
	// 会 fallback 回 c.in.ReadString 同步读。
	inputCh chan string
}

func newConsole(a *app.App, in io.Reader, out io.Writer) *consoleUI {
	return &consoleUI{app: a, in: bufio.NewReader(in), out: out}
}

// startInputLoop 启一个后台 goroutine 把 stdin 每行喂给 c.inputCh。
// ctx 取消时关 channel 让读取方退出。
func (c *consoleUI) startInputLoop(ctx context.Context) {
	c.inputCh = make(chan string, 1)
	go func() {
		defer close(c.inputCh)
		for {
			line, err := c.in.ReadString('\n')
			if err != nil {
				return
			}
			select {
			case c.inputCh <- strings.TrimRight(line, "\r\n"):
			case <-ctx.Done():
				return
			}
		}
	}()
}

// readLine 从 inputCh 读一行；inputCh 没开的话回退到同步读 c.in。
func (c *consoleUI) readLine() string {
	if c.inputCh != nil {
		line, ok := <-c.inputCh
		if !ok {
			return ""
		}
		return strings.TrimSpace(line)
	}
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
	dimC.Fprintln(c.out, "    （以上都能在主菜单→流量控制 里随时改）")
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

func (c *consoleUI) configureSubscription() {
	c.app.Cfg.Source.Type = config.SourceTypeSubscription
	s := &c.app.Cfg.Source.Subscription
	s.URL = c.ask("  订阅 URL", s.URL)
	s.Name = c.ask("  订阅名称", firstNonEmpty(s.Name, "subscription"))
}

func (c *consoleUI) configureFile() {
	c.app.Cfg.Source.Type = config.SourceTypeFile
	c.app.Cfg.Source.File.Path = c.ask("  本地配置文件绝对路径 (Clash/mihomo YAML)", c.app.Cfg.Source.File.Path)
}

// --- Main menu ---

func (c *consoleUI) main(ctx context.Context) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		c.drawMainMenu()
		// 记录当前告警态；后台 supervisor 改变它时要触发重绘。
		lastFallback := c.app.Health().FallbackActive
		lastErr := c.app.Health().LastError

	waitInput:
		for {
			select {
			case line, ok := <-c.inputCh:
				if !ok {
					return nil // stdin 断了（EOF / Ctrl-D）
				}
				choice := strings.ToLower(strings.TrimSpace(line))
				switch choice {
				case "":
					// 用户直接回车：主动刷新主菜单
					break waitInput
				case "1":
					c.screenGateway()
					break waitInput
				case "2":
					c.screenTraffic(ctx)
					break waitInput
				case "3":
					c.screenSource(ctx)
					break waitInput
				case "4":
					c.screenLifecycle(ctx)
					break waitInput
				case "5":
					c.screenLogs()
					break waitInput
				case "6":
					if c.shutdownGateway() {
						return nil
					}
					break waitInput
				case "q", "exit", "quit":
					return nil
				default:
					warnC.Fprintln(c.out, "无效选项（回车刷新，数字=子菜单，Q=退出）。")
					fmt.Fprint(c.out, "请选择：> ")
				}

			case <-ticker.C:
				// 后台 health 发生变化 → 主菜单立即重绘，
				// 告警能在首页第一时间被看到。
				h := c.app.Health()
				if h.FallbackActive != lastFallback || h.LastError != lastErr {
					fmt.Fprintln(c.out)
					dimC.Fprintln(c.out, "  （检测到代理源状态变化，刷新主菜单）")
					break waitInput
				}

			case <-ctx.Done():
				return nil
			}
		}
	}
}

// drawMainMenu 画一次主菜单屏。main loop 里的每次进入/重绘都用它。
func (c *consoleUI) drawMainMenu() {
	c.banner("LAN 代理网关")
	c.printStatus()
	fmt.Fprintln(c.out)
	fmt.Fprintln(c.out, "  1  设备接入指引        Switch / PS5 / 手机怎么连到这里")
	fmt.Fprintln(c.out, "  2  流量控制            模式 / TUN / 广告拦截 / 高级")
	fmt.Fprintln(c.out, "  3  换代理源")
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
		dimC.Fprintln(c.out, "    修复后会自动切回；想立刻重试去「换代理源 → T 重新测试」")
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
	c.banner("网关状态 / 设备接入指引")
	_ = c.app.Gateway.Detect()
	fmt.Fprintln(c.out, gateway.DeviceGuide(c.app.Status().Gateway))
	c.pause()
}

func (c *consoleUI) screenTraffic(ctx context.Context) {
	for {
		c.banner("流量控制")
		cfg := c.app.Cfg
		fmt.Fprintf(c.out, "  模式: %s   TUN: %s   广告拦截: %s\n\n",
			cfg.Traffic.Mode,
			onOff(cfg.Gateway.TUN.Enabled),
			onOff(cfg.Traffic.Adblock))
		fmt.Fprintln(c.out, "  1  切换模式     rule=国内直连+国外代理（推荐）/ global=全走代理 / direct=全直连")
		fmt.Fprintln(c.out, "  2  开关 TUN     （Switch/PS5 等能走代理的关键，一般别动）")
		fmt.Fprintln(c.out, "  3  开关广告拦截")
		dimC.Fprintln(c.out, "  9  高级设置     （DNS 开关 / 端口冲突调整，日常不用来）")
		dimC.Fprintln(c.out, "  0  返回主菜单（或按 Q）")
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
		case "9":
			c.screenTrafficAdvanced(ctx)
		case "0", "q", "Q", "":
			return
		default:
			warnC.Fprintln(c.out, "无效选项，按 0 或 Q 返回主菜单")
		}
	}
}

// screenTrafficAdvanced 收纳普通用户基本不需要碰的开关：DNS 代理开关、
// DNS 监听端口、mixed 端口、API 端口。99% 场景下来这里只是为了解端口冲突。
func (c *consoleUI) screenTrafficAdvanced(ctx context.Context) {
	for {
		c.banner("流量控制 · 高级（端口冲突时才来）")
		cfg := c.app.Cfg
		fmt.Fprintf(c.out, "  DNS 代理: %s (监听端口 %d)   mixed: %d   API: %d\n\n",
			onOff(cfg.Gateway.DNS.Enabled), cfg.Gateway.DNS.Port,
			cfg.Runtime.Ports.Mixed, cfg.Runtime.Ports.API)
		fmt.Fprintln(c.out, "  1  开关 DNS 代理           （本机 53 端口被 Clash Verge 等占用时关掉）")
		fmt.Fprintln(c.out, "  2  修改 DNS 监听端口        （默认 53；改了 LAN 设备就基本解析不了，不建议动）")
		fmt.Fprintln(c.out, "  3  修改 mixed 端口          （HTTP+SOCKS5，默认 17890，避开了 Clash 7890）")
		fmt.Fprintln(c.out, "  4  修改 API 端口            （默认 19090，避开了 Clash 9090）")
		dimC.Fprintln(c.out, "  0  返回（或按 Q）")
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
		c.banner("换代理源")
		fmt.Fprintf(c.out, "  当前: %s\n\n", sourceLabel(c.app.Cfg.Source.Type))
		fmt.Fprintf(c.out, "  1  单点代理        %s\n", probes.single)
		fmt.Fprintf(c.out, "  2  机场订阅        %s\n", probes.subscription)
		fmt.Fprintf(c.out, "  3  本地配置文件    %s\n", probes.file)
		fmt.Fprintln(c.out, "  4  暂不配置        全部直连（无须测试）")
		fmt.Fprintln(c.out)
		// 切换节点只对 subscription / file 有意义（单点代理只有一个 Upstream）
		if c.app.Cfg.Source.Type == config.SourceTypeSubscription ||
			c.app.Cfg.Source.Type == config.SourceTypeFile {
			fmt.Fprintln(c.out, "  N  切换节点        从订阅/文件里挑分组+节点")
		}
		fmt.Fprintln(c.out, "  T  重新测试        再跑一次连通性测试")
		dimC.Fprintln(c.out, "  0  返回主菜单（或按 Q）")

		choice := strings.ToLower(c.prompt("选择：> "))
		changed := false // 是否改动了 config（需要 save+reload 并重新探测）
		switch choice {
		case "1":
			c.configureSingle()
			changed = true
		case "2":
			c.configureSubscription()
			changed = true
		case "3":
			c.configureFile()
			changed = true
		case "4":
			c.app.Cfg.Source.Type = config.SourceTypeNone
			changed = true
		case "n":
			c.screenSwitchNode(ctx)
			continue
		case "t":
			probes = c.probeAllSources(ctx, c.app.Cfg)
			continue
		case "0", "q", "":
			return
		default:
			warnC.Fprintln(c.out, "无效选项，按 0 或 Q 返回")
			continue
		}
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

// sourceProbes 是三类可配置源的「摘要 + ✓/✗」结果，在「换代理源」菜单
// 的每一行右侧呈现。未配置的返回「（未配置）」。
type sourceProbes struct {
	single, subscription, file string
}

// probeAllSources 并发探测 external/remote、subscription、file。
// 5 秒 hard deadline 包干，避免订阅慢拖住菜单。
// 探测期间临时在 stdout 打一行「测试中…」做视觉反馈，完事擦掉。
func (c *consoleUI) probeAllSources(ctx context.Context, cfg *config.Config) sourceProbes {
	dimC.Fprintln(c.out, "  测试中……")
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var r sourceProbes
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); r.single = probeSingle(probeCtx, cfg) }()
	go func() { defer wg.Done(); r.subscription = probeSubscription(probeCtx, cfg) }()
	go func() { defer wg.Done(); r.file = probeFile(cfg) }()
	wg.Wait()
	return r
}

// probeSingle 优先看 Remote（有认证那个），没有就看 External。
// 两个都空 → 未配置。有配置就当场 TCP dial 测。
func probeSingle(ctx context.Context, cfg *config.Config) string {
	switch {
	case cfg.Source.Remote.Server != "" && cfg.Source.Remote.Port > 0:
		r := cfg.Source.Remote
		sum := fmt.Sprintf("%s:%d %s", r.Server, r.Port, r.Kind)
		if r.Username != "" {
			sum += " 带认证"
		}
		err := source.Test(ctx, config.SourceConfig{Type: config.SourceTypeRemote, Remote: r})
		return padRight(sum, 32) + probeMark(err)
	case cfg.Source.External.Server != "" && cfg.Source.External.Port > 0:
		e := cfg.Source.External
		sum := fmt.Sprintf("%s:%d %s", e.Server, e.Port, e.Kind)
		err := source.Test(ctx, config.SourceConfig{Type: config.SourceTypeExternal, External: e})
		return padRight(sum, 32) + probeMark(err)
	}
	return dimC.Sprint("（未配置）")
}

func probeSubscription(ctx context.Context, cfg *config.Config) string {
	s := cfg.Source.Subscription
	if s.URL == "" {
		return dimC.Sprint("（未配置）")
	}
	sum := truncate(s.URL, 30)
	err := source.Test(ctx, config.SourceConfig{Type: config.SourceTypeSubscription, Subscription: s})
	return padRight(sum, 32) + probeMark(err)
}

func probeFile(cfg *config.Config) string {
	p := cfg.Source.File.Path
	if p == "" {
		return dimC.Sprint("（未配置）")
	}
	err := source.Test(context.Background(), config.SourceConfig{Type: config.SourceTypeFile, File: cfg.Source.File})
	return padRight(truncate(p, 30), 32) + probeMark(err)
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

// padRight 把字符串右填空格到指定宽度。ANSI 色码让 fmt 的 %-Ns 宽度失准，
// 所以自己算。这里不考虑已含 ANSI（probe 摘要都是纯文本），没问题。
func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
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
			fmt.Fprintf(c.out, "  %2d  %-20s  当前: %s  (共 %d 个节点)\n",
				i+1, g.Name, g.Now, len(g.All))
		}
		dimC.Fprintln(c.out, "   0  返回（或按 Q）")
		input := strings.ToLower(c.prompt("选分组：> "))
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

// screenSwitchNodeInGroup 展示一个分组里的所有节点，当前选中的加 ✓。
func (c *consoleUI) screenSwitchNodeInGroup(ctx context.Context, g engine.ProxyGroup) {
	c.banner(fmt.Sprintf("分组：%s  (当前：%s)", g.Name, g.Now))
	for i, n := range g.All {
		prefix := "    "
		if n == g.Now {
			prefix = "  ✓ "
		}
		fmt.Fprintf(c.out, "  %2d%s%s\n", i+1, prefix, n)
	}
	dimC.Fprintln(c.out, "   0  返回（或按 Q）")
	input := strings.ToLower(c.prompt("选节点：> "))
	if input == "0" || input == "" || input == "q" {
		return
	}
	idx, err := strconv.Atoi(input)
	if err != nil || idx < 1 || idx > len(g.All) {
		warnC.Fprintln(c.out, "无效选项")
		return
	}
	node := g.All[idx-1]
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := c.app.Engine.API().SelectNode(ctx2, g.Name, node); err != nil {
		badC.Fprintf(c.out, "切换失败：%v\n", err)
	} else {
		okC.Fprintf(c.out, "已切换 %s → %s\n", g.Name, node)
	}
	c.pause()
}

func (c *consoleUI) screenLifecycle(ctx context.Context) {
	c.banner("生命周期")
	fmt.Fprintln(c.out, "  1  启动")
	fmt.Fprintln(c.out, "  2  重启 (热重载)")
	fmt.Fprintln(c.out, "  3  停止")
	fmt.Fprintln(c.out, "  4  清理残留 mihomo 进程 (端口被占用时用)")
	dimC.Fprintln(c.out, "  0  返回主菜单（或按 Q）")
	admin, _ := c.app.Plat.IsAdmin()
	if !admin {
		warnC.Fprintln(c.out, "  （未用 sudo 运行，启动/停止/清理会失败，请先用 sudo gateway）")
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
			okC.Fprintln(c.out, "已停止")
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
		okC.Fprintln(c.out, "已停止 mihomo")
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
	for _, l := range lines[start:] {
		if rawMode {
			fmt.Fprintln(c.out, l)
		} else {
			fmt.Fprintln(c.out, humanizeMihomoLine(l))
		}
	}
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
							fmt.Fprintln(c.out, humanizeMihomoLine(s[:nl]))
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
