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
	"time"

	"github.com/fatih/color"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/engine"
	"github.com/tght/lan-proxy-gateway/internal/gateway"
)

// Run is the entry point. It blocks until the user exits.
func Run(ctx context.Context, a *app.App) error {
	c := newConsole(a, os.Stdin, os.Stdout)
	if !a.Configured() {
		if err := c.onboard(ctx); err != nil {
			return err
		}
	}
	return c.main(ctx)
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
}

func newConsole(a *app.App, in io.Reader, out io.Writer) *consoleUI {
	return &consoleUI{app: a, in: bufio.NewReader(in), out: out}
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
	line, _ := c.in.ReadString('\n')
	return strings.TrimSpace(line)
}

func (c *consoleUI) ask(label, def string) string {
	if def != "" {
		dimC.Fprintf(c.out, "%s（回车=%s）: ", label, def)
	} else {
		fmt.Fprintf(c.out, "%s: ", label)
	}
	line, _ := c.in.ReadString('\n')
	line = strings.TrimSpace(line)
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
	titleC.Fprintln(c.out, "  只剩一件事：把流量转发到哪个代理？")
	fmt.Fprintln(c.out, "    1) 使用本机已有代理端口    (已在跑 Clash Verge / Shadowrocket 选这个)")
	fmt.Fprintln(c.out, "    2) 订阅链接                (输入机场 URL，网关自己开代理)")
	fmt.Fprintln(c.out, "    3) 本地 Clash 配置文件     (指向一个 .yaml)")
	fmt.Fprintln(c.out, "    4) 远程单点代理            (手填 socks5 / http)")
	fmt.Fprintln(c.out, "    5) 暂不配置                (全部走直连，以后再来)")
	fmt.Fprintln(c.out)
	choice := c.ask("请选择 1-5", "1")
	switch choice {
	case "2":
		c.configureSubscription()
	case "3":
		c.configureFile()
	case "4":
		c.configureRemote()
	case "5":
		c.app.Cfg.Source.Type = config.SourceTypeNone
	default:
		c.configureExternal()
	}

	if err := c.app.Save(); err != nil {
		badC.Fprintf(c.out, "保存配置失败: %v\n", err)
		return err
	}
	okC.Fprintln(c.out, "\n✔ 配置已保存到 "+c.app.Paths.ConfigFile)
	return nil
}

// --- Source-type configurators ---

func (c *consoleUI) configureExternal() {
	c.app.Cfg.Source.Type = config.SourceTypeExternal
	e := &c.app.Cfg.Source.External
	e.Name = "本机已有代理"
	e.Server = c.ask("  代理主机", firstNonEmpty(e.Server, "127.0.0.1"))
	port := c.ask("  代理端口", strconv.Itoa(firstNonZero(e.Port, 7890)))
	if p, err := strconv.Atoi(port); err == nil && p > 0 {
		e.Port = p
	}
	kind := strings.ToLower(c.ask("  代理类型 (http 或 socks5)", firstNonEmpty(e.Kind, "http")))
	if kind != "socks5" {
		kind = "http"
	}
	e.Kind = kind
}

func (c *consoleUI) configureSubscription() {
	c.app.Cfg.Source.Type = config.SourceTypeSubscription
	s := &c.app.Cfg.Source.Subscription
	s.URL = c.ask("  订阅 URL", s.URL)
	s.Name = c.ask("  订阅名称", firstNonEmpty(s.Name, "subscription"))
}

func (c *consoleUI) configureFile() {
	c.app.Cfg.Source.Type = config.SourceTypeFile
	c.app.Cfg.Source.File.Path = c.ask("  Clash 配置文件绝对路径", c.app.Cfg.Source.File.Path)
}

func (c *consoleUI) configureRemote() {
	c.app.Cfg.Source.Type = config.SourceTypeRemote
	r := &c.app.Cfg.Source.Remote
	r.Name = c.ask("  代理名称", firstNonEmpty(r.Name, "RemoteProxy"))
	r.Server = c.ask("  代理主机", r.Server)
	portStr := c.ask("  代理端口", strconv.Itoa(firstNonZero(r.Port, 443)))
	if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
		r.Port = p
	}
	kind := strings.ToLower(c.ask("  代理类型 (http 或 socks5)", firstNonEmpty(r.Kind, "socks5")))
	if kind != "http" {
		kind = "socks5"
	}
	r.Kind = kind
	r.Username = c.ask("  用户名 (可选，无需认证直接回车)", r.Username)
	if r.Username != "" {
		r.Password = c.ask("  密码", r.Password)
	}
}

// --- Main menu ---

func (c *consoleUI) main(ctx context.Context) error {
	for {
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
		choice := strings.ToLower(c.prompt("\n请选择：> "))
		switch choice {
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
				return nil
			}
		case "q", "exit", "quit":
			return nil
		default:
			warnC.Fprintln(c.out, "无效选项。")
		}
	}
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
		return "本机已有代理（external）"
	case config.SourceTypeSubscription:
		return "机场订阅（subscription）"
	case config.SourceTypeFile:
		return "本地 Clash 配置文件（file）"
	case config.SourceTypeRemote:
		return "远程单点代理（remote）"
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
	for {
		c.banner("代理端口来源")
		fmt.Fprintf(c.out, "  当前: %s\n\n", c.app.Cfg.Source.Type)
		fmt.Fprintln(c.out, "  1  本机已有代理端口 (external)")
		fmt.Fprintln(c.out, "  2  订阅链接           (subscription)")
		fmt.Fprintln(c.out, "  3  Clash 配置文件     (file)")
		fmt.Fprintln(c.out, "  4  远程单点代理       (remote)")
		fmt.Fprintln(c.out, "  5  不配置 (仅直连)    (none)")
		dimC.Fprintln(c.out, "  0  返回主菜单（或按 Q）")
		choice := c.prompt("选择：> ")
		switch choice {
		case "1":
			c.configureExternal()
		case "2":
			c.configureSubscription()
		case "3":
			c.configureFile()
		case "4":
			c.configureRemote()
		case "5":
			c.app.Cfg.Source.Type = config.SourceTypeNone
		case "0", "q", "Q", "":
			return
		default:
			warnC.Fprintln(c.out, "无效选项，按 0 或 Q 返回")
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
	}
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

	// 用独立 goroutine 等一个回车；主循环按 tick 读增量。
	// 这期间 c.in 被 goroutine 独占 —— 外层 screenLogs 要等本函数返回后才会再读。
	done := make(chan struct{})
	go func() {
		_, _ = c.in.ReadString('\n')
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
	_, _ = c.in.ReadString('\n')
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
