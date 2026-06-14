package console

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/engine"
)

func (c *consoleUI) screenTraffic(ctx context.Context) {
	for {
		c.banner("分流 & 规则")
		cfg := c.app.Cfg
		gwMode := cfg.Gateway.Mode
		if gwMode == "" {
			gwMode = config.GatewayModeTUN
		}
		fmt.Fprintf(c.out, "  模式 %s  ·  网关 %s  ·  TUN %s  ·  广告拦截 %s  ·  自定义规则 %d\n\n",
			cfg.Traffic.Mode,
			gatewayModeLabel(gwMode),
			onOff(cfg.Gateway.TUN.Enabled),
			onOff(cfg.Traffic.Adblock),
			len(cfg.Traffic.Extras.Direct)+len(cfg.Traffic.Extras.Proxy)+len(cfg.Traffic.Extras.Reject))
		fmt.Fprintln(c.out, "  1  切换模式     rule=国内直连+国外代理（推荐）/ global=全走代理 / direct=全直连")
		fmt.Fprintln(c.out, "  2  开关 TUN     （改网关设备需要；本机单点上游会自动启用本机绕过）")
		fmt.Fprintln(c.out, "  3  开关广告拦截")
		fmt.Fprintln(c.out, "  4  自定义规则   直连 / 代理 / 拒绝 三组（优先级最高，盖过内置 china_direct 等）")
		fmt.Fprintf(c.out, "  5  策略组自动补全 %s   订阅里缺 Auto/Fallback 时自动加（直选节点也能自动切换）\n",
			onOff(cfg.Traffic.AutoGroups))
		fmt.Fprintf(c.out, "  6  切换网关模式   当前: %s\n", gatewayModeLabel(gwMode))
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
				fmt.Fprintln(c.out, "  如果你只是让手机/电脑手动填代理服务器=本机 IP:17890，")
				fmt.Fprintln(c.out, "  不需要 TUN 也能用；但本机单点上游会自动用本机绕过，")
				fmt.Fprintln(c.out, "  避免 Clash / 其它代理客户端自己的出站被 gateway 抓回来。")
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
		case "6":
			c.switchGatewayMode(ctx)
		case "9":
			c.screenTrafficAdvanced(ctx)
		case "0", "q", "Q", "":
			return
		default:
			warnC.Fprintln(c.out, "无效选项，按 0 或 Q 返回主菜单")
		}
	}
}

// screenCustomRules 管理用户自定义规则（config.Traffic.Extras 的 Direct/Proxy/Reject/Groups）。
// 这些规则在 traffic.Render 里**最先**被 emit，优先级高过所有内置 ruleset。
func (c *consoleUI) screenCustomRules(ctx context.Context) {
	type item struct {
		verdict string // DIRECT / PROXY / REJECT / GROUP
		target  string
		rule    string
	}
	for {
		c.banner("自定义规则")
		ex := &c.app.Cfg.Traffic.Extras
		dimC.Fprintln(c.out, "  规则越靠上优先级越高。mihomo 先扫自定义，再扫内置 china_direct / adblock 等。")
		dimC.Fprintln(c.out, "  常见类型：DOMAIN-SUFFIX / DOMAIN / DOMAIN-KEYWORD / IP-CIDR / PROCESS-NAME")
		fmt.Fprintln(c.out)

		var listed []item
		dump := func(group string, list []string, verdict string, target string) {
			titleC.Fprintf(c.out, "  [%s] (%d)\n", group, len(list))
			if len(list) == 0 {
				dimC.Fprintln(c.out, "    (无)")
				return
			}
			for _, r := range list {
				listed = append(listed, item{verdict: verdict, target: target, rule: r})
				fmt.Fprintf(c.out, "    %2d  %s\n", len(listed), r)
			}
		}
		dump("直连", ex.Direct, "DIRECT", "")
		dump("走代理 · Proxy", ex.Proxy, "PROXY", "")
		dump("拒绝", ex.Reject, "REJECT", "")
		for _, group := range ex.Groups {
			target := strings.TrimSpace(group.Target)
			if target == "" {
				continue
			}
			dump("指定策略组 · "+target, group.Rules, "GROUP", target)
		}

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
			c.deleteCustomRule(ctx, target.verdict, target.target, target.rule)
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
	fmt.Fprintln(c.out, "    4) 指定策略组")
	verdict := c.ask("请选择 1-4", "2")

	ex := &c.app.Cfg.Traffic.Extras
	var label string
	switch verdict {
	case "1":
		ex.Direct = append(ex.Direct, rule)
		label = "DIRECT"
	case "3":
		ex.Reject = append(ex.Reject, rule)
		label = "REJECT"
	case "4":
		target := c.askPolicyGroupTarget(ctx)
		if target == "" {
			warnC.Fprintln(c.out, "  策略组名为空，取消")
			return
		}
		addTargetedRule(ex, target, rule)
		label = target
	default:
		ex.Proxy = append(ex.Proxy, rule)
		label = "Proxy"
	}
	c.saveAndMaybeReload(ctx, fmt.Sprintf("  ✓ 已加规则：%s → %s", rule, label))
}

// deleteCustomRule 删除命中的某条。
func (c *consoleUI) deleteCustomRule(ctx context.Context, verdict, target, rule string) {
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
	case "GROUP":
		groups := ex.Groups[:0]
		for _, group := range ex.Groups {
			if group.Target == target {
				group.Rules = remove(group.Rules)
			}
			if strings.TrimSpace(group.Target) != "" && len(group.Rules) > 0 {
				groups = append(groups, group)
			}
		}
		ex.Groups = groups
	}
	c.saveAndMaybeReload(ctx, fmt.Sprintf("  ✓ 已删：%s", rule))
}

func (c *consoleUI) askPolicyGroupTarget(ctx context.Context) string {
	groups := c.knownPolicyGroups(ctx)
	if len(groups) > 0 {
		fmt.Fprintln(c.out, "\n  可用策略组：")
		for i, name := range groups {
			fmt.Fprintf(c.out, "    %2d) %s\n", i+1, name)
		}
		fmt.Fprintln(c.out, "     0) 手动输入")
		choice := strings.TrimSpace(c.ask("请选择策略组编号，或直接输入组名", "1"))
		if n, err := strconv.Atoi(choice); err == nil {
			if n >= 1 && n <= len(groups) {
				return groups[n-1]
			}
			if n != 0 {
				return ""
			}
		} else if choice != "" {
			return choice
		}
	}
	return strings.TrimSpace(c.ask("  策略组名", "Proxy"))
}

func (c *consoleUI) knownPolicyGroups(ctx context.Context) []string {
	defaults := []string{"Proxy", "🛫 AI起飞节点", "🛬 AI落地节点"}
	seen := make(map[string]bool, len(defaults)+8)
	out := make([]string, 0, len(defaults)+8)
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	for _, name := range defaults {
		add(name)
	}
	if c.app.Engine == nil || !c.app.Engine.Running() {
		return out
	}
	listCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	groups, err := c.app.Engine.API().ListProxyGroups(listCtx)
	cancel()
	if err != nil {
		return out
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	for _, group := range groups {
		add(group.Name)
	}
	return out
}

func addTargetedRule(ex *config.ExtraRules, target, rule string) {
	target = strings.TrimSpace(target)
	rule = strings.TrimSpace(rule)
	if target == "" || rule == "" {
		return
	}
	for i := range ex.Groups {
		if ex.Groups[i].Target == target {
			ex.Groups[i].Rules = append(ex.Groups[i].Rules, rule)
			return
		}
	}
	ex.Groups = append(ex.Groups, config.TargetedRules{Target: target, Rules: []string{rule}})
}

// switchGatewayMode 在 TUN 旁路由 和 端口模式 之间切换。
// 切换需要完整 stop + start（TUN 网卡 / iptables 规则要重建），不能只 hot-reload。
//
// 文案按平台分支：Mac 上 forward = 端口模式（无 TUN，零干扰）；Linux 上 forward
// = iptables 透明旁路由。两者都能让宿主机网络栈不受 TUN 干扰，但 LAN 侧体验
// 不一样，所以分开说。
func (c *consoleUI) switchGatewayMode(ctx context.Context) {
	cur := c.app.Cfg.Gateway.Mode
	if cur == "" {
		cur = config.GatewayModeTUN
	}
	mixed := strconv.Itoa(c.app.Cfg.Runtime.Ports.Mixed)
	fmt.Fprintln(c.out)
	if runtime.GOOS == "darwin" {
		fmt.Fprintln(c.out, "  1) TUN 旁路由 (推荐 / 默认)")
		dimC.Fprintln(c.out, "     投影仪、Switch、AppleTV 改默认网关 + DNS 到本机 IP 即走代理。")
		dimC.Fprintln(c.out, "     已做低干扰处理：不抢宿主机 53、不接管宿主机出向；")
		dimC.Fprintln(c.out, "     代价是会出现一个 utun 接口，少量 VPN 客户端可能重新评估路由。")
		fmt.Fprintln(c.out, "  2) 端口模式 (零干扰)")
		dimC.Fprintln(c.out, "     完全不开 TUN，宿主机网络栈纹丝不动 —— Tailscale 等彻底不受影响。")
		dimC.Fprintln(c.out, "     代价是 LAN 设备必须能手动填代理 → <本机IP>:"+mixed+"（投影仪/电视通常做不到）。")
	} else {
		fmt.Fprintln(c.out, "  1) TUN 全局       宿主机 + 其他设备全走代理")
		fmt.Fprintln(c.out, "  2) 仅转发         iptables REDIRECT 透明旁路由，宿主机直连不受影响")
		dimC.Fprintln(c.out, "     宿主机想走代理可手动设 http_proxy=127.0.0.1:"+mixed)
	}
	fmt.Fprintf(c.out, "\n  当前: %s\n\n", gatewayModeLabel(cur))

	choice := c.prompt("请选择 1-2（回车=取消）：> ")
	var target string
	switch choice {
	case "1":
		target = config.GatewayModeTUN
	case "2":
		target = config.GatewayModeForward
	default:
		return
	}
	if target == cur {
		dimC.Fprintln(c.out, "已经是该模式，无需切换")
		return
	}
	if err := c.app.SetGatewayMode(ctx, target); err != nil {
		badC.Fprintf(c.out, "切换失败: %v\n", err)
	} else {
		okC.Fprintf(c.out, "已切换到 %s\n", gatewayModeLabel(target))
	}
}

func gatewayModeLabel(mode string) string {
	switch mode {
	case config.GatewayModeForward:
		if runtime.GOOS == "darwin" {
			return "端口模式 (零干扰)"
		}
		return "仅转发 (iptables REDIRECT)"
	default:
		if runtime.GOOS == "darwin" {
			return "TUN 旁路由 (低干扰)"
		}
		return "TUN 全局"
	}
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
		fmt.Fprintln(c.out, "  1  开关 DNS 代理        （只用手机/电脑填 17890 时可关；改网关设备才需要）")
		fmt.Fprintln(c.out, "  2  修改 DNS 监听端口    （默认 53；改了 LAN 设备就基本解析不了，不建议动）")
		fmt.Fprintln(c.out, "  3  修改 mixed 端口      （HTTP+SOCKS5，默认 17890，避开 Clash 7890；也是局域网代理端口）")
		fmt.Fprintln(c.out, "  4  修改 API 端口        （默认 19090，避开 Clash 9090；mihomo 控制台 /ui 也走这个）")
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
