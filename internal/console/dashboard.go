package console

import (
	"context"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/devices"
	"github.com/tght/lan-proxy-gateway/internal/engine"
	"github.com/tght/lan-proxy-gateway/internal/geoip"
	"github.com/tght/lan-proxy-gateway/internal/ipinfo"
	"github.com/tght/lan-proxy-gateway/internal/source"
)

// ipinfoTTL 是真实出口查询的缓存有效期。ipinfo.io 免费版 1000 次/天，30 秒
// 对交互式手动刷新来说既够新、又不会把额度打爆。
const ipinfoTTL = 30 * time.Second

// dashboardState 维护两次采样之间的差分状态，用来算实时速率。
// 大部分字段只在主 loop 单线程里访问；ipinfo 相关字段会被后台刷新 goroutine
// 写入，所以通过 ipinfoMu 保护。
type dashboardState struct {
	lastFetch time.Time
	lastDL    int64
	lastUL    int64
	lastConns map[string]connSample // conn.ID → 上次采样的字节数 + sourceIP

	// ipinfo 真实出口缓存。后台 goroutine 写入，主 loop 读取。
	ipinfoMu       sync.Mutex
	ipinfoInfo     *ipinfo.Info // 最近一次成功的结果；即便后续失败也保留旧值
	ipinfoErr      string       // 最近一次请求的错误（空 = 成功或未查过）
	ipinfoAt       time.Time    // 最近一次请求完成时刻（零值 = 没查过）
	ipinfoInflight bool         // 后台请求是否在飞
}

type connSample struct {
	download int64
	upload   int64
	sourceIP string
}

// dashboardSnapshot 是一次刷新计算出的渲染数据，render 阶段只读。
type dashboardSnapshot struct {
	ok        bool   // false = 当前拉不到数据（mihomo 没跑 / API 失败）
	errMsg    string // ok=false 时给用户看的原因
	localIP   string
	proxySrc  string

	// 瞬时数据
	downRate  float64 // bytes/s
	upRate    float64
	downTotal int64
	upTotal   int64
	connCount int

	// 起飞 / 落地
	takeoff proxyHop // 第一跳（通常是机场/单点代理节点）
	landing proxyHop // 最终出口（链式代理的住宅 IP，或 == takeoff）

	// 真实出口（ipinfo.io 查到的落地 IP / 位置 / ISP）。nil 表示从没成功过
	// 或 mihomo 没跑；egressAge 表示这份数据距今多久，用来提示「30s 前」等。
	egress    *ipinfo.Info
	egressErr string        // 最近一次请求的错误（nil egress + 非空 err = 查失败）
	egressAge time.Duration // 数据新鲜度；零值 = 从没查过
	egressPending bool      // 后台首次查询进行中，还没有任何结果

	// 设备聚合
	devices []deviceRow
}

// proxyHop 表示一个代理位置点。
type proxyHop struct {
	name    string // 节点名（包含用户起的 emoji/国旗）
	flag    string // 2 个 regional indicator 字符 emoji；提取不到留空
	hint    string // 辅助信息（如 "234ms"、"住宅IP 1.2.3.4"），可空
}

// deviceRow 仪表盘设备表的一行。
type deviceRow struct {
	ip        string
	name      string  // 可能为空
	downRate  float64 // bytes/s
	upRate    float64
	connCount int
}

// fetchDashboardSnapshot 并发拉 mihomo 的 /connections + /proxies，合并成
// 一个 snapshot。全流程 2s 超时包干；任一步失败时返回 ok=false 让 render
// 显示占位提示，不崩。
func fetchDashboardSnapshot(
	ctx context.Context,
	cli *engine.Client,
	cfg *config.Config,
	localIP string,
	geo *geoip.DB,
	resolver *devices.Resolver,
	state *dashboardState,
) dashboardSnapshot {
	snap := dashboardSnapshot{
		localIP:  localIP,
		proxySrc: sourceLabel(cfg.Source.Type),
	}
	if cli == nil {
		snap.errMsg = "mihomo 未启动（按 M 进菜单选 4 启动）"
		return snap
	}
	fetchCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var (
		conns  *engine.ConnectionsSnapshot
		groups []engine.ProxyGroup
		cErr, gErr error
	)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); conns, cErr = cli.GetConnections(fetchCtx) }()
	go func() { defer wg.Done(); groups, gErr = cli.ListProxyGroups(fetchCtx) }()
	wg.Wait()

	if cErr != nil {
		snap.errMsg = fmt.Sprintf("拉取 /connections 失败：%v", cErr)
		return snap
	}
	_ = gErr // 分组拉不到时降级到「无起飞/落地信息」，不影响主体

	snap.ok = true
	snap.downTotal = conns.DownloadTotal
	snap.upTotal = conns.UploadTotal
	snap.connCount = len(conns.Connections)

	// 速率 = 两次采样差 ÷ 间隔；首次 / mihomo 重启（总量变小）给 0
	now := time.Now()
	if !state.lastFetch.IsZero() {
		elapsed := now.Sub(state.lastFetch).Seconds()
		if elapsed > 0.05 {
			if d := conns.DownloadTotal - state.lastDL; d > 0 {
				snap.downRate = float64(d) / elapsed
			}
			if u := conns.UploadTotal - state.lastUL; u > 0 {
				snap.upRate = float64(u) / elapsed
			}
		}
	}

	// 按设备聚合：(sourceIP) → 累计下行速率 + 上行速率 + 连接数
	byIP := map[string]*deviceRow{}
	thisConns := make(map[string]connSample, len(conns.Connections))
	elapsed := now.Sub(state.lastFetch).Seconds()
	hasPrev := !state.lastFetch.IsZero() && elapsed > 0.05
	for _, c := range conns.Connections {
		ip := c.Metadata.SourceIP
		if ip == "" || isLocalhost(ip) {
			continue
		}
		row, ok := byIP[ip]
		if !ok {
			row = &deviceRow{ip: ip, name: resolver.LookupName(ip)}
			byIP[ip] = row
		}
		row.connCount++
		// 单连接速率：这次 - 上次；旧连接查不到就从 0 算起
		if hasPrev {
			if prev, ok := state.lastConns[c.ID]; ok {
				if d := c.Download - prev.download; d > 0 {
					row.downRate += float64(d) / elapsed
				}
				if u := c.Upload - prev.upload; u > 0 {
					row.upRate += float64(u) / elapsed
				}
			}
		}
		thisConns[c.ID] = connSample{download: c.Download, upload: c.Upload, sourceIP: ip}
	}

	// 落盘本次采样，给下次差分用
	state.lastFetch = now
	state.lastDL = conns.DownloadTotal
	state.lastUL = conns.UploadTotal
	state.lastConns = thisConns

	// 排一下：速率高的在前，相同按 IP
	snap.devices = make([]deviceRow, 0, len(byIP))
	for _, row := range byIP {
		snap.devices = append(snap.devices, *row)
	}
	sort.Slice(snap.devices, func(i, j int) bool {
		if snap.devices[i].downRate != snap.devices[j].downRate {
			return snap.devices[i].downRate > snap.devices[j].downRate
		}
		return snap.devices[i].ip < snap.devices[j].ip
	})

	// 起飞 / 落地
	snap.takeoff, snap.landing = resolveHops(groups, cfg, geo)

	// 真实出口：读缓存填 snap，顺便在必要时 kick 后台刷新。
	fillEgressFromCache(&snap, state)
	kickIPInfoRefresh(cfg, state)

	return snap
}

// fillEgressFromCache 在 state.ipinfoMu 保护下读当前 ipinfo 缓存填进 snap。
func fillEgressFromCache(snap *dashboardSnapshot, state *dashboardState) {
	state.ipinfoMu.Lock()
	defer state.ipinfoMu.Unlock()
	snap.egress = state.ipinfoInfo
	snap.egressErr = state.ipinfoErr
	if !state.ipinfoAt.IsZero() {
		snap.egressAge = time.Since(state.ipinfoAt)
	}
	// 从没成功 + 从没失败 + 正在查 = 首次 pending
	snap.egressPending = state.ipinfoInflight && state.ipinfoInfo == nil && state.ipinfoErr == ""
}

// kickIPInfoRefresh 在需要时启动一个后台 goroutine 去 ipinfo.io 取真实出口。
// 同一时刻只跑一个请求；缓存新鲜就直接返回，不发请求。刷新结果下次绘制时
// 才会被用户看到——符合「手动刷新的静态仪表盘」哲学，不引入自动重绘。
func kickIPInfoRefresh(cfg *config.Config, state *dashboardState) {
	proxyURL := source.LocalMixedProxyURL(cfg.Runtime.Ports.Mixed)
	if proxyURL == "" {
		return
	}
	state.ipinfoMu.Lock()
	if state.ipinfoInflight {
		state.ipinfoMu.Unlock()
		return
	}
	// 成功过且还新鲜 → 不查
	if state.ipinfoInfo != nil && state.ipinfoErr == "" &&
		!state.ipinfoAt.IsZero() && time.Since(state.ipinfoAt) < ipinfoTTL {
		state.ipinfoMu.Unlock()
		return
	}
	state.ipinfoInflight = true
	state.ipinfoMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		info, err := ipinfo.Fetch(ctx, proxyURL)

		state.ipinfoMu.Lock()
		defer state.ipinfoMu.Unlock()
		state.ipinfoInflight = false
		state.ipinfoAt = time.Now()
		if err != nil {
			state.ipinfoErr = err.Error()
			// 失败时保留旧 info，让用户还能看到上次的真实出口
			return
		}
		state.ipinfoInfo = info
		state.ipinfoErr = ""
	}()
}

// resolveHops 从 proxy-groups 当前选中节点推断起飞点，再结合 ChainResidential
// 推断落地点。两者可能相等（没走链式代理时）。
func resolveHops(groups []engine.ProxyGroup, cfg *config.Config, geo *geoip.DB) (takeoff, landing proxyHop) {
	// 先找起飞：名字带 🛫 或「起飞」的组优先，否则第一个 Selector/URLTest 组。
	var chosen *engine.ProxyGroup
	for i := range groups {
		g := &groups[i]
		if strings.Contains(g.Name, "🛫") || strings.Contains(g.Name, "起飞") {
			chosen = g
			break
		}
	}
	if chosen == nil {
		for i := range groups {
			g := &groups[i]
			if g.Now != "" && !strings.EqualFold(g.Now, "DIRECT") && !strings.EqualFold(g.Now, "REJECT") {
				chosen = g
				break
			}
		}
	}
	if chosen != nil && chosen.Now != "" {
		takeoff = proxyHop{name: chosen.Now, flag: extractFlag(chosen.Now)}
	} else {
		// 没有任何可用组（源=none 或单点直代理），按 source type 兜个说明
		takeoff = takeoffFromSource(cfg)
	}

	// 落地：链式代理启用 → 是住宅 IP；否则落地 == 起飞
	if cfg.Source.ChainResidential != nil {
		r := cfg.Source.ChainResidential
		_, flag := geo.LookupString(r.Server)
		if flag == "" {
			flag = "🏠"
		}
		landing = proxyHop{
			name: r.Name,
			flag: flag,
			hint: fmt.Sprintf("%s:%d", r.Server, r.Port),
		}
	} else {
		landing = takeoff
	}
	return
}

// takeoffFromSource 在 mihomo 没给出 proxy-groups 时（例如 source=external 单
// 点直代理），根据 config.SourceConfig 直接画起飞 hop。
func takeoffFromSource(cfg *config.Config) proxyHop {
	switch cfg.Source.Type {
	case config.SourceTypeExternal:
		e := cfg.Source.External
		return proxyHop{name: e.Name, hint: fmt.Sprintf("%s:%d", e.Server, e.Port)}
	case config.SourceTypeRemote:
		r := cfg.Source.Remote
		return proxyHop{name: r.Name, hint: fmt.Sprintf("%s:%d", r.Server, r.Port)}
	case config.SourceTypeNone, "":
		return proxyHop{name: "（未配置）"}
	default:
		return proxyHop{name: "—"}
	}
}

// extractFlag 从节点名里扫一段 2 位 regional indicator（国旗 emoji）。没找到
// 返回空串。机场节点名基本都带 🇭🇰/🇯🇵 前缀；也兼容中间出现的形式。
func extractFlag(name string) string {
	runes := []rune(name)
	for i := 0; i < len(runes)-1; i++ {
		if isRegionalIndicator(runes[i]) && isRegionalIndicator(runes[i+1]) {
			return string(runes[i : i+2])
		}
	}
	return ""
}

func isRegionalIndicator(r rune) bool {
	return r >= 0x1F1E6 && r <= 0x1F1FF
}

// drawDashboard 把 snapshot 画到终端。保持紧凑：没 mihomo / 拉取失败时也给一
// 行占位，让用户知道控制台活着、问题在 mihomo。
func drawDashboard(w io.Writer, snap dashboardSnapshot, running bool) {
	fmt.Fprintln(w)
	titleC.Fprintln(w, bar)
	if running {
		titleC.Fprintf(w, "  LAN 代理网关  ")
		okC.Fprintln(w, "● 运行中")
	} else {
		titleC.Fprintf(w, "  LAN 代理网关  ")
		dimC.Fprintln(w, "○ 未启动")
	}
	titleC.Fprintln(w, bar)

	// 即便 ok=false 也给两行基础信息，再加一行错误
	fmt.Fprintf(w, "  本机 IP: %s    代理源: %s\n", nonEmpty(snap.localIP, "<未检测>"), snap.proxySrc)

	if !snap.ok {
		fmt.Fprintln(w)
		warnC.Fprintf(w, "  ⚠ %s\n", snap.errMsg)
		fmt.Fprintln(w)
		dimC.Fprintln(w, "  [M] 菜单   [Q] 退出")
		return
	}

	// 速率 + 累计
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  ↓ %s/s   ↑ %s/s   连接 %d\n",
		humanBytes(snap.downRate), humanBytes(snap.upRate), snap.connCount)
	dimC.Fprintf(w, "  本次累计  ↓ %s   ↑ %s  （mihomo 启动起）\n",
		humanBytes(float64(snap.downTotal)), humanBytes(float64(snap.upTotal)))

	// 起飞 / 落地
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  🛫 起飞  %s  %s\n", hopFlag(snap.takeoff), hopDesc(snap.takeoff))
	fmt.Fprintf(w, "  🛬 落地  %s  %s\n", hopFlag(snap.landing), hopDesc(snap.landing))
	// 真实出口：ipinfo.io 实测的 IP / 位置 / ISP。链式代理、IP 轮换、多跳
	// 的场景下，这一行比 landing 的节点名更能反映真相。
	drawEgressLine(w, snap)

	// 设备表
	fmt.Fprintln(w)
	titleC.Fprintf(w, "  接入设备 (%d)\n", len(snap.devices))
	if len(snap.devices) == 0 {
		dimC.Fprintln(w, "    暂无（LAN 设备把网关 / DNS 指向本机 IP 后就会出现）")
	} else {
		// 最多显示 8 条，避免超屏
		limit := len(snap.devices)
		if limit > 8 {
			limit = 8
		}
		for _, d := range snap.devices[:limit] {
			name := d.name
			if name == "" {
				name = dimC.Sprint("—")
			}
			fmt.Fprintf(w, "    %-15s  %s  ↓ %s/s  %d conn\n",
				d.ip, padRightWide(name, 14), humanBytes(d.downRate), d.connCount)
		}
		if len(snap.devices) > limit {
			dimC.Fprintf(w, "    …还有 %d 个设备（进菜单看完整列表）\n", len(snap.devices)-limit)
		}
	}

	fmt.Fprintln(w)
	titleC.Fprintln(w, "  [回车/R] 刷新   [M] 菜单   [N] 切节点   [T] 代理源   [Q] 退出")
	dimC.Fprintln(w, "  更顺手的详细操作建议直接用下方 Web 控制台。")
}

// drawEgressLine 画「真实出口」那一行。四种状态：
//   - 有 info：显示 IP / City, Country / ISP；过旧时附「Ns 前」
//   - 仅 err：显示查询失败原因（dim），不吓人
//   - pending（第一次后台查询飞行中）：显示「查询中…」
//   - 全空（不该到这里，代理 URL 为空时 kick 不会发请求）：不画
func drawEgressLine(w io.Writer, snap dashboardSnapshot) {
	switch {
	case snap.egress != nil:
		loc := egressLocation(snap.egress)
		isp := snap.egress.ISP()
		line := fmt.Sprintf("    ↳ 真实出口  %s", snap.egress.IP)
		if loc != "" {
			line += "  " + loc
		}
		if isp != "" {
			line += "  · " + isp
		}
		fmt.Fprintln(w, line)
		// 缓存老于 TTL 给一个温和提示；否则不打扰
		if snap.egressAge > ipinfoTTL {
			dimC.Fprintf(w, "      （%s 前的数据，后台正在刷新）\n", humanDuration(snap.egressAge))
		}
	case snap.egressPending:
		dimC.Fprintln(w, "    ↳ 真实出口  查询中…")
	case snap.egressErr != "":
		dimC.Fprintf(w, "    ↳ 真实出口  查询失败（%s）\n", truncateErr(snap.egressErr))
	}
}

// egressLocation 把 ipinfo 的 City/Region/Country 拼成一行可读位置。字段缺失
// 时尽量给出能看的内容，全空就返回空串。
func egressLocation(info *ipinfo.Info) string {
	parts := []string{}
	if info.City != "" {
		parts = append(parts, info.City)
	}
	// Region 和 City 重复的机会不小（例如 "Hong Kong, Hong Kong"），去重
	if info.Region != "" && info.Region != info.City {
		parts = append(parts, info.Region)
	}
	if info.Country != "" {
		parts = append(parts, info.Country)
	}
	return strings.Join(parts, ", ")
}

// humanDuration 把 duration 格式化成人读串，短到秒，长到分。仪表盘只用来
// 显示「N 秒/分前」，不需要小时级精度。
func humanDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}

// truncateErr 把可能很长的 error string 截到一行仪表盘能装下。
func truncateErr(s string) string {
	const max = 60
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func hopFlag(h proxyHop) string {
	if h.flag != "" {
		return h.flag
	}
	return dimC.Sprint("  ")
}

func hopDesc(h proxyHop) string {
	if h.hint == "" {
		return h.name
	}
	return fmt.Sprintf("%s  %s", h.name, dimC.Sprint(h.hint))
}

func nonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// humanBytes 把字节/速率格式化成 1.2 KB / 3.4 MB / 2.3 GB。仪表盘足够用，
// 不用 SI vs IEC 纠结，统一 1024。
func humanBytes(n float64) string {
	if n < 1024 {
		return fmt.Sprintf("%4.0f  B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%5.1f KB", n/1024)
	}
	if n < 1024*1024*1024 {
		return fmt.Sprintf("%5.1f MB", n/(1024*1024))
	}
	return fmt.Sprintf("%5.2f GB", n/(1024*1024*1024))
}

// fakeIPNet 是 mihomo TUN fake-ip 默认使用的 198.18.0.0/15（benchmark 专用段）。
// TUN 模式下本机自己访问被伪造到这段地址，不能算进「接入设备」表。
var fakeIPNet = func() *net.IPNet { _, n, _ := net.ParseCIDR("198.18.0.0/15"); return n }()

// isLocalhost 判定 sourceIP 是不是不该上设备表的那些：127.x / ::1（本机环回）
// 和 198.18.0.0/15（TUN fake-ip）。LAN 真机一律是 10/172.16/192.168 段，不受影响。
func isLocalhost(ip string) bool {
	p := net.ParseIP(ip)
	if p == nil {
		return false
	}
	if p.IsLoopback() {
		return true
	}
	if fakeIPNet != nil && fakeIPNet.Contains(p) {
		return true
	}
	return false
}

// screenDeviceLabels 管理 IP → 设备名映射。A 添加 / D 删除 / 0 返回。
// 变动立刻写回 gateway.yaml（不重启 mihomo；仪表盘下一帧就会看到新名字）。
func (c *consoleUI) screenDeviceLabels() {
	for {
		c.banner("给 IP 起名字 · 设备标签")
		dimC.Fprintln(c.out, "  路由器不报 hostname 的设备（PS5、智能电视、老 Android）可以手动打标签。")
		dimC.Fprintln(c.out, "  打过标签的会覆盖反向 DNS，优先显示。")
		fmt.Fprintln(c.out)

		labels := c.app.Cfg.Gateway.DeviceLabels
		// 排成表，序号用来删除
		ips := make([]string, 0, len(labels))
		for ip := range labels {
			ips = append(ips, ip)
		}
		sort.Strings(ips)
		if len(ips) == 0 {
			dimC.Fprintln(c.out, "  （还没有标签）")
		} else {
			titleC.Fprintln(c.out, "  #   IP              名字")
			for i, ip := range ips {
				fmt.Fprintf(c.out, "  %2d  %-15s %s\n", i+1, ip, labels[ip])
			}
		}

		fmt.Fprintln(c.out)
		titleC.Fprintln(c.out, "  ── 操作 ── A 添加   D <编号> 删除   0 返回（或按 Q）")
		input := strings.ToLower(strings.TrimSpace(c.prompt("选择：> ")))
		switch {
		case input == "" || input == "0" || input == "q":
			return
		case input == "a":
			c.addDeviceLabel()
		case strings.HasPrefix(input, "d"):
			numStr := strings.TrimSpace(strings.TrimPrefix(input, "d"))
			idx, err := strconv.Atoi(numStr)
			if err != nil || idx < 1 || idx > len(ips) {
				warnC.Fprintln(c.out, "无效编号（格式: d 3 或 d3）")
				continue
			}
			delete(c.app.Cfg.Gateway.DeviceLabels, ips[idx-1])
			if err := c.app.Save(); err != nil {
				badC.Fprintln(c.out, err.Error())
			} else {
				okC.Fprintf(c.out, "  ✓ 已删 %s\n", ips[idx-1])
			}
		default:
			warnC.Fprintln(c.out, "无效操作")
		}
	}
}

// addDeviceLabel 引导式添加一条 IP → 名字。IP 做一次 net.ParseIP 校验；重复
// IP 会直接覆盖老名字（没有二次确认，用户意图就是「改名」）。
func (c *consoleUI) addDeviceLabel() {
	ip := strings.TrimSpace(c.ask("  设备 IP（例如 192.168.1.23）", ""))
	if ip == "" {
		return
	}
	if net.ParseIP(ip) == nil {
		warnC.Fprintln(c.out, "  不是合法 IP，取消")
		return
	}
	name := strings.TrimSpace(c.ask("  起个名字（如 Switch / PS5 / iPhone）", ""))
	if name == "" {
		warnC.Fprintln(c.out, "  名字为空，取消")
		return
	}
	if c.app.Cfg.Gateway.DeviceLabels == nil {
		c.app.Cfg.Gateway.DeviceLabels = map[string]string{}
	}
	c.app.Cfg.Gateway.DeviceLabels[ip] = name
	if err := c.app.Save(); err != nil {
		badC.Fprintln(c.out, err.Error())
		return
	}
	okC.Fprintf(c.out, "  ✓ %s → %s\n", ip, name)
}

