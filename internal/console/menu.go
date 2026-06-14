package console

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/engine"
)

// --- Main loop: 仪表盘首页 ---
//
// 默认只画 dashboard（实时速率 / 累计 / 起飞落地 / 设备表），每 2 秒自动刷。
// 旧的多级菜单藏到 [M] 键后面，避免首屏被操作项淹没。
// 快捷键：M 菜单、N 切节点、T 重测代理源、Q 退出控制台（网关留后台）。

func (c *consoleUI) main(ctx context.Context) error {
	// 后台每分钟测一次主出口组延迟，记进健康条；ctx 取消时自动退出。
	go c.runHealthTicker(ctx)

	// 实时首页：每 2 秒自动重绘仪表盘，网速柱 / 健康条 / 速率随之滚动「活」起来，
	// 一有输入立刻让位处理命令。输入用一个常驻读取 goroutine 喂进 channel，与刷新
	// ticker 在 select 里竞争 —— 这样自动刷新和阻塞读 stdin 不互相抢终端。
	lines := make(chan string, 1)
	pendingRead := false
	refresh := time.NewTicker(2 * time.Second)
	defer refresh.Stop()

	for {
		c.clearScreen() // 原地重绘，实时首页不往下滚
		c.drawDashboardOnce(ctx)
		// 保证恰好有一个待命的读取 goroutine（重绘时它还阻塞在读，则不再起新的）。
		if !pendingRead {
			pendingRead = true
			go func() { lines <- c.readLine() }()
		}

		var raw string
		select {
		case <-ctx.Done():
			return nil
		case <-refresh.C:
			continue // 定时自动重绘，让图表动起来
		case raw = <-lines:
			pendingRead = false
		}

		choice := strings.ToLower(strings.TrimSpace(raw))
		switch choice {
		case "", "r":
			// 回车 / R → 立即重新拉数据再画一次
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
			// 非命令 → 交给 AI 配网助手（用原始行保留大小写/中文）
			c.handleNaturalLanguage(ctx, strings.TrimSpace(raw))
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

	// 把本帧下行速率 push 进网速柱状图（拉不到数据 downRate 为 0，自然画成空柱）。
	if c.spark != nil {
		c.spark.push(snap.downRate)
	}

	// 代理源异常告警（supervisor 维护的状态），叠在仪表盘顶部警示。
	drawDashboard(c.out, snap, running, c.spark, c.health)
	if running {
		apiPort := c.app.Cfg.Runtime.Ports.API
		if apiPort > 0 {
			fmt.Fprintln(c.out)
			titleC.Fprintln(c.out, "  Mihomo 完整控制台（mihomo 引擎自带面板：切节点 / 看流量）")
			if localIP != "" {
				dimC.Fprintf(c.out, "    http://%s:%d/ui/\n", localIP, apiPort)
			}
			dimC.Fprintf(c.out, "    http://127.0.0.1:%d/ui/\n", apiPort)
		}
	}
	h := c.app.Health()
	if h.FallbackActive {
		badC.Fprintln(c.out)
		badC.Fprintf(c.out, "  ⚠ 代理源异常 · 已临时切直连：%s\n", h.LastError)
	} else if !h.Healthy && h.LastError != "" {
		warnC.Fprintln(c.out)
		warnC.Fprintf(c.out, "  ⚠ 代理源健康探测失败（未切直连）：%s\n", h.LastError)
	}
	if c.aiAvailable() {
		dimC.Fprintln(c.out, "\n  💬 直接输入一句话，让 AI 配网助手帮你（如：帮我设置订阅源 https://...）")
	}
	fmt.Fprint(c.out, "\n请选择：> ")
}

// screenMenu 是旧版主菜单折成的「操作抽屉」。返回 true 表示用户在里头关了
// gateway，主循环该一起退出。Q/0/回车只返回首页仪表盘；首页的 Q 才退出控制台。
func (c *consoleUI) screenMenu(ctx context.Context) (exitConsole bool) {
	for {
		c.drawMainMenu()
		choice := strings.ToLower(c.readLine())
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
				return true
			}
		case "7":
			c.screenAIBackends(ctx)
		case "", "0", "q", "back", "exit", "quit":
			return false
		default:
			warnC.Fprintln(c.out, "无效选项（数字=子菜单，0/Q 返回首页，6 停止并退出）")
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
	fmt.Fprintln(c.out, "  7  AI 助手后端        配置/切换 AI 配网助手的大模型（内置免费可用）")
	fmt.Fprintln(c.out, "  6  关闭 gateway 并退出（停 mihomo）")
	fmt.Fprintln(c.out, "  Q  返回首页（mihomo 留在后台继续跑）")
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
	} else if !h.Healthy && h.LastError != "" {
		warnC.Fprintln(c.out, "  ⚠ 代理源健康探测失败（未切直连）")
		warnC.Fprintf(c.out, "    原因: %s\n", h.LastError)
		dimC.Fprintln(c.out, "    本机单点代理不会因探测失败自动切直连，避免影响正在使用的链路。")
	}

	admin, _ := c.app.Plat.IsAdmin()
	if !admin {
		dimC.Fprintln(c.out, "  （未用 sudo；看状态、改配置都不需要；启动时才需要）")
	}
	if s.Running && !s.TUN {
		if s.GatewayMode == config.GatewayModeForward {
			dimC.Fprintln(c.out, "  仅转发模式：宿主机流量直连，其他设备通过旁路由/代理端口走代理")
		} else {
			warnC.Fprintln(c.out, "  ⚠ TUN 已关：Switch / PS5 等就算把网关指到本机，流量也不会走代理")
		}
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
