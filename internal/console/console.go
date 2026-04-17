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
		c.banner("lan-proxy-gateway · 主菜单")
		c.printStatus()
		fmt.Fprintln(c.out)
		fmt.Fprintln(c.out, "  1  网关状态 / 设备接入指引")
		fmt.Fprintln(c.out, "  2  流量控制（模式 / 广告拦截 / 规则）")
		fmt.Fprintln(c.out, "  3  代理端口来源")
		fmt.Fprintln(c.out, "  4  启动 / 重启 / 停止")
		fmt.Fprintln(c.out, "  5  日志 (mihomo.log 末尾 30 行)")
		fmt.Fprintln(c.out, "  Q  退出")
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
		case "q", "exit", "quit":
			return nil
		default:
			warnC.Fprintln(c.out, "无效选项。")
		}
	}
}

func (c *consoleUI) printStatus() {
	s := c.app.Status()
	running := "未启动"
	runStyle := dimC
	if s.Running {
		running = "运行中"
		runStyle = okC
	}
	admin, _ := c.app.Plat.IsAdmin()
	if !admin {
		dimC.Fprintln(c.out, "  （当前未用 sudo；看状态、改配置都不需要；启动时才需要）")
	}
	mode := s.Mode
	if mode == "" {
		mode = "?"
	}
	adblock := "关"
	if s.Adblock {
		adblock = "开"
	}
	tun := "关"
	if s.TUN {
		tun = "开"
	}
	tunDisplay := tun
	if !s.TUN {
		tunDisplay = badC.Sprint("关 ⚠")
	}
	dnsDisplay := "开"
	if !c.app.Cfg.Gateway.DNS.Enabled {
		dnsDisplay = warnC.Sprint("关")
	}
	fmt.Fprintf(c.out, "  状态: %s   模式: %s   广告拦截: %s   TUN: %s   DNS: %s\n",
		runStyle.Sprint(running), mode, adblock, tunDisplay, dnsDisplay)
	fmt.Fprintf(c.out, "  源  : %s\n", s.Source)
	fmt.Fprintf(c.out, "  本机: %s  网关: %s\n", s.Gateway.LocalIP, s.Gateway.Router)
	if !s.TUN {
		warnC.Fprintln(c.out, "  ⚠ TUN 关闭中：Switch/PS5 等改了网关的设备不会走代理！")
	}
	if !c.app.Cfg.Gateway.DNS.Enabled {
		warnC.Fprintln(c.out, "  ⚠ DNS 代理关闭中：LAN 设备 DNS 不能再指向本机 IP，要单独设能用的 DNS！")
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
		fmt.Fprintf(c.out, "  模式: %s   广告拦截: %s   TUN: %s   DNS: %s (端口 %d)\n\n",
			cfg.Traffic.Mode, onOff(cfg.Traffic.Adblock),
			onOff(cfg.Gateway.TUN.Enabled),
			onOff(cfg.Gateway.DNS.Enabled), cfg.Gateway.DNS.Port)
		fmt.Fprintln(c.out, "  1  切换模式 (rule / global / direct)")
		fmt.Fprintln(c.out, "  2  开关广告拦截")
		fmt.Fprintln(c.out, "  3  开关 TUN 模式")
		fmt.Fprintln(c.out, "  4  开关 DNS 代理（端口冲突时关掉它最常见）")
		fmt.Fprintln(c.out, "  5  修改 DNS 监听端口（默认 53）")
		fmt.Fprintln(c.out, "  6  修改 mixed 端口（默认 7890）")
		fmt.Fprintln(c.out, "  7  修改 API 端口（默认 9090）")
		dimC.Fprintln(c.out, "  0  返回主菜单（或按 Q）")
		switch c.prompt("选择：> ") {
		case "1":
			fmt.Fprintln(c.out, "  1) rule    规则模式")
			fmt.Fprintln(c.out, "  2) global  全局")
			fmt.Fprintln(c.out, "  3) direct  直连")
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
			if err := c.app.ToggleAdblock(ctx); err != nil {
				badC.Fprintln(c.out, err.Error())
			}
		case "3":
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
		case "4":
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
		case "5":
			c.promptPort(ctx, "DNS 监听端口", &c.app.Cfg.Gateway.DNS.Port)
		case "6":
			c.promptPort(ctx, "mixed 端口", &c.app.Cfg.Runtime.Ports.Mixed)
		case "7":
			c.promptPort(ctx, "API 端口", &c.app.Cfg.Runtime.Ports.API)
		case "0", "q", "Q", "":
			return
		default:
			warnC.Fprintln(c.out, "无效选项，按 0 或 Q 返回主菜单")
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
	}
}

// tryStart runs Start() and, on port conflict caused by a stale mihomo, offers
// to kill it and retry in-place. This is the most common recovery path for
// users who ran gateway once, crashed/killed the terminal, and now find the
// port squatted by an orphan process.
func (c *consoleUI) tryStart(ctx context.Context) {
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
	data, err := os.ReadFile(path)
	if err != nil {
		warnC.Fprintf(c.out, "暂无日志 (%s)\n", path)
		c.pause()
		return
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	start := 0
	if len(lines) > 30 {
		start = len(lines) - 30
	}
	c.banner("日志 · " + path)
	for _, l := range lines[start:] {
		fmt.Fprintln(c.out, l)
	}
	c.pause()
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
