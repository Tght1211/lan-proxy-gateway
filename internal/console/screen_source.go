package console

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/engine"
	"github.com/tght/lan-proxy-gateway/internal/source"
)

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

		// mihomo 自带的 external-ui 控制台（切节点 / 看流量），跟源类型无关：
		// 只要服务在跑就能访问。配置改动统一走 CLI 菜单。
		if c.app.Engine != nil && c.app.Engine.Running() && c.app.Cfg.Runtime.Ports.API > 0 {
			apiPort := c.app.Cfg.Runtime.Ports.API
			localIP := c.app.Status().Gateway.LocalIP
			fmt.Fprintln(c.out)
			titleC.Fprintln(c.out, "  ── Mihomo 完整控制台（mihomo 引擎自带面板：切节点 / 看流量）──")
			if localIP != "" {
				dimC.Fprintf(c.out, "    http://%s:%d/ui/\n", localIP, apiPort)
			}
			dimC.Fprintf(c.out, "    http://127.0.0.1:%d/ui/\n", apiPort)
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
