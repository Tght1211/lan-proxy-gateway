package console

import (
	"context"
	"strings"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/engine"
)

// healthProbeTestURL 是测出口连通性的目标。generate_204 命中即 204，轻量且不被墙误判。
const healthProbeTestURL = "http://www.gstatic.com/generate_204"

// runHealthTicker 每分钟测一次主出口组延迟，把结果（通/不通）记进 c.health 柱。
// 引擎没起、组名拿不到、测速失败都记 record(false)，绝不 panic。ctx 取消时退出。
func (c *consoleUI) runHealthTicker(ctx context.Context) {
	if c == nil || c.health == nil {
		return
	}
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()

	probe := func() {
		c.health.record(c.probeEgressHealthy(ctx))
	}
	probe() // 开局先记一格，避免健康条全空白
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			probe()
		}
	}
}

// probeEgressHealthy 用 mihomo API 测一次主出口组延迟，至少一个节点有延迟即视为健康。
func (c *consoleUI) probeEgressHealthy(ctx context.Context) bool {
	if c.app == nil || c.app.Engine == nil || !c.app.Engine.Running() {
		return false
	}
	cli := c.app.Engine.API()
	if cli == nil {
		return false
	}
	group := c.primaryEgressGroup(ctx, cli)
	if group == "" {
		return false
	}
	pctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	delays, err := cli.GroupDelay(pctx, group, healthProbeTestURL, 3000)
	if err != nil {
		return false
	}
	// 至少一个节点测出非零延迟才算健康（0 = 超时/拒绝）。
	for _, ms := range delays {
		if ms > 0 {
			return true
		}
	}
	return false
}

// primaryEgressGroup 选一个出口组测速：优先名字带「起飞」/🛫 的组，否则第一个
// 可选 group，最后兜底 "GLOBAL"。拿不到列表也回退 GLOBAL，不硬编码 "Proxy"。
func (c *consoleUI) primaryEgressGroup(ctx context.Context, cli *engine.Client) string {
	listCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	groups, err := cli.ListProxyGroups(listCtx)
	cancel()
	if err != nil || len(groups) == 0 {
		return "GLOBAL"
	}
	for _, g := range groups {
		if strings.Contains(g.Name, "🛫") || strings.Contains(g.Name, "起飞") {
			return g.Name
		}
	}
	return groups[0].Name
}
