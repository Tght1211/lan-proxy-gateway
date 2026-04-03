package cmd

import (
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/egress"
	"github.com/tght/lan-proxy-gateway/internal/mihomo"
	"github.com/tght/lan-proxy-gateway/internal/platform"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

type alertMessage struct {
	level string
	title string
	body  string
}

type subscriptionSnapshot struct {
	Name          string
	Source        string
	URL           string
	SourceHost    string
	RemainingText string
	ExpiryText    string
	UsageText     string
	UsagePercent  int
}

type nodeSnapshot struct {
	PolicyMode string
	Group      string
	Strategy   string
	Node       string
}

type networkSnapshot struct {
	TunEnabled   bool
	TunReady     bool
	TunInterface string
	BypassLocal  bool
	LanSharing   bool
	Warning      string
}

type trafficSnapshot struct {
	UploadTotal       int64
	DownloadTotal     int64
	UploadSpeed       int64
	DownloadSpeed     int64
	ActiveConnections int
	KernelUsage       string
	UploadTrend       []int64
	DownloadTrend     []int64
}

type addressSnapshot struct {
	Current string
	Entry   string
	Exit    string
}

type latencySnapshot struct {
	Sites map[string]string
}

type subscriptionUsage struct {
	Upload    int64
	Download  int64
	Total     int64
	ExpiresAt int64
}

type siteProbeTarget struct {
	Name string
	URL  string
}

var homeSiteTargets = []siteProbeTarget{
	{Name: "YouTube", URL: "https://www.youtube.com/generate_204"},
	{Name: "Google", URL: "http://www.gstatic.com/generate_204"},
	{Name: "GitHub", URL: "https://github.com"},
	{Name: "Apple", URL: "https://www.apple.com/library/test/success.html"},
}

func (m *runtimeConsoleModel) refreshDashboardSnapshot() {
	m.cfg = loadConfigOrDefault()
	m.client = newConsoleClient(m.cfg)

	now := time.Now()
	cfg := m.cfg
	client := m.client
	p := platform.New()

	report := egress.Collect(cfg, m.dataDir, client)
	activeProfile := cfg.Proxy.ActiveProfile()
	subscription := m.collectSubscriptionSnapshot(activeProfile, client)
	node := m.collectNodeSnapshot(client, cfg)
	network := m.collectNetworkSnapshot(p, cfg)
	traffic := m.collectTrafficSnapshot(p)
	addresses := buildAddressSnapshot(cfg, report)
	latency := m.collectLatencySnapshot(client)

	alerts := buildConsoleAlerts(cfg, network)
	modeSummary := compactModeSummary(cfg)
	egressSummary := compactEgressSummary(cfg, report)

	m.snapshot = snapshot{
		modeSummary:     modeSummary,
		egressSummary:   egressSummary,
		panelURL:        fmt.Sprintf("http://%s:%d/ui", m.ip, cfg.Runtime.Ports.API),
		configPath:      displayConfigPath(),
		iface:           m.iface,
		currentNode:     node.Node,
		shareEntry:      m.ip,
		refreshedAt:     now.Format("15:04:05"),
		activeProfile:   activeProfile,
		subscription:    subscription,
		node:            node,
		network:         network,
		traffic:         traffic,
		addresses:       addresses,
		latency:         latency,
		alerts:          alerts,
		overviewSummary: buildOverviewSummary(subscription, node, network, traffic),
	}
}

func (m *runtimeConsoleModel) collectSubscriptionSnapshot(profile config.ProxyProfile, client *mihomo.Client) subscriptionSnapshot {
	snap := subscriptionSnapshot{
		Name:       profile.Name,
		Source:     profile.Source,
		URL:        profile.SubscriptionURL,
		SourceHost: providerSourceHost(m.cfg),
	}

	if provider := loadActiveProvider(client, m.cfg); provider != nil {
		if remaining, expiry, website := parseProviderHints(provider); remaining != "" || expiry != "" || website != "" {
			if remaining != "" {
				snap.RemainingText = remaining
			}
			if expiry != "" {
				snap.ExpiryText = expiry
			}
			if website != "" {
				snap.SourceHost = website
			}
		}
	}

	if usage, err := fetchSubscriptionUsage(profile.SubscriptionURL); err == nil && usage.Total > 0 {
		used := usage.Upload + usage.Download
		remaining := usage.Total - used
		if remaining < 0 {
			remaining = 0
		}
		snap.RemainingText = ui.FormatBytes(remaining)
		if usage.ExpiresAt > 0 {
			snap.ExpiryText = time.Unix(usage.ExpiresAt, 0).In(time.Local).Format("2006-01-02 15:04")
		}
		snap.UsagePercent = clampPercent(int(float64(used) * 100 / float64(usage.Total)))
		snap.UsageText = fmt.Sprintf("%s / %s", ui.FormatBytes(used), ui.FormatBytes(usage.Total))
	}

	if snap.RemainingText == "" {
		snap.RemainingText = "未获取"
	}
	if snap.ExpiryText == "" {
		snap.ExpiryText = "未获取"
	}
	if snap.UsageText == "" {
		snap.UsageText = "总量未知"
	}
	return snap
}

func loadActiveProvider(client *mihomo.Client, cfg *config.Config) *mihomo.ProxyProvider {
	if client == nil || !client.IsAvailable() {
		return nil
	}
	candidates := []string{
		strings.TrimSpace(cfg.Proxy.SubscriptionName),
		"Proxy",
		"default",
	}
	for _, name := range candidates {
		if name == "" {
			continue
		}
		provider, err := client.GetProxyProvider(name)
		if err == nil && provider != nil && len(provider.Proxies) > 0 {
			return provider
		}
	}
	return nil
}

func parseProviderHints(provider *mihomo.ProxyProvider) (remaining, expiry, website string) {
	if provider == nil {
		return "", "", ""
	}
	for _, proxy := range provider.Proxies {
		for _, item := range proxy.All {
			switch {
			case strings.HasPrefix(item, "剩余流量："):
				remaining = strings.TrimPrefix(item, "剩余流量：")
			case strings.HasPrefix(item, "套餐到期："):
				expiry = strings.TrimPrefix(item, "套餐到期：")
			case strings.HasPrefix(item, "最新网址："):
				website = strings.TrimPrefix(item, "最新网址：")
			}
		}
	}
	return remaining, expiry, website
}

func fetchSubscriptionUsage(raw string) (*subscriptionUsage, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty subscription url")
	}
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1))
	info := strings.TrimSpace(resp.Header.Get("subscription-userinfo"))
	if info == "" {
		return nil, fmt.Errorf("subscription-userinfo missing")
	}
	return parseSubscriptionUsage(info)
}

func parseSubscriptionUsage(info string) (*subscriptionUsage, error) {
	usage := &subscriptionUsage{}
	for _, part := range strings.Split(info, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		value, err := strconv.ParseInt(strings.TrimSpace(kv[1]), 10, 64)
		if err != nil {
			continue
		}
		switch strings.TrimSpace(kv[0]) {
		case "upload":
			usage.Upload = value
		case "download":
			usage.Download = value
		case "total":
			usage.Total = value
		case "expire":
			usage.ExpiresAt = value
		}
	}
	if usage.Total == 0 && usage.ExpiresAt == 0 && usage.Upload == 0 && usage.Download == 0 {
		return nil, fmt.Errorf("invalid subscription-userinfo")
	}
	return usage, nil
}

func (m *runtimeConsoleModel) collectNodeSnapshot(client *mihomo.Client, cfg *config.Config) nodeSnapshot {
	snap := nodeSnapshot{
		PolicyMode: "规则",
		Group:      "Proxy",
		Strategy:   "未识别",
		Node:       "未识别",
	}
	if client == nil || !client.IsAvailable() {
		return snap
	}
	proxyGroup, err := client.GetProxyGroup("Proxy")
	if err != nil {
		return snap
	}
	now := strings.TrimSpace(proxyGroup.Now)
	if now == "" {
		now = "未识别"
	}
	snap.Strategy = now
	snap.Node = now
	if now == "DIRECT" {
		snap.PolicyMode = "直连"
		snap.Group = "-"
		return snap
	}

	if cfg.Extension.Mode == "chains" && cfg.Extension.ResidentialChain != nil && cfg.Extension.ResidentialChain.Mode == "global" {
		snap.PolicyMode = "全局"
	}

	if now == "Auto" || now == "Fallback" {
		if strategy, err := client.GetProxyGroup(now); err == nil {
			if strings.TrimSpace(strategy.Now) != "" {
				snap.Node = strategy.Now
			}
			snap.Group = strategy.Name
		}
	} else {
		snap.Group = "Proxy"
	}
	return snap
}

func (m *runtimeConsoleModel) collectNetworkSnapshot(p platform.Platform, cfg *config.Config) networkSnapshot {
	tunIf, _ := p.DetectTUNInterface()
	snap := networkSnapshot{
		TunEnabled:   cfg.Runtime.Tun.Enabled,
		TunReady:     tunIf != "",
		TunInterface: tunIf,
		BypassLocal:  cfg.Runtime.Tun.BypassLocal,
		LanSharing:   cfg.Runtime.Tun.Enabled && tunIf != "",
	}
	switch {
	case !cfg.Runtime.Tun.Enabled:
		snap.Warning = "未开启 TUN：当前只有本机可用，局域网共享功能不可用。"
	case tunIf == "":
		snap.Warning = "TUN 未就绪：局域网共享功能不可用，常见原因是另一套 Clash/mihomo 已占用 TUN。"
	}
	return snap
}

func (m *runtimeConsoleModel) collectTrafficSnapshot(p platform.Platform) trafficSnapshot {
	snap := m.snapshot.traffic
	if m.client == nil || !m.client.IsAvailable() {
		return snap
	}
	info, err := m.client.GetConnections()
	if err != nil {
		return snap
	}

	now := time.Now()
	snap.UploadTotal = info.UploadTotal
	snap.DownloadTotal = info.DownloadTotal
	snap.ActiveConnections = len(info.Connections)
	if running, pid, _ := p.IsRunning(); running {
		snap.KernelUsage = processUsageSummary(pid)
	}

	if !m.lastPolled.IsZero() {
		seconds := now.Sub(m.lastPolled).Seconds()
		if seconds > 0 {
			upDelta := info.UploadTotal - m.lastUp
			downDelta := info.DownloadTotal - m.lastDown
			if upDelta >= 0 {
				snap.UploadSpeed = int64(float64(upDelta) / seconds)
			}
			if downDelta >= 0 {
				snap.DownloadSpeed = int64(float64(downDelta) / seconds)
			}
		}
	}
	snap.UploadTrend = appendTrendPoint(snap.UploadTrend, snap.UploadSpeed)
	snap.DownloadTrend = appendTrendPoint(snap.DownloadTrend, snap.DownloadSpeed)
	m.lastPolled = now
	m.lastUp = info.UploadTotal
	m.lastDown = info.DownloadTotal
	return snap
}

func appendTrendPoint(history []int64, value int64) []int64 {
	history = append(history, value)
	if len(history) > 10 {
		history = history[len(history)-10:]
	}
	return history
}

func processUsageSummary(pid int) string {
	if pid <= 0 {
		return "未获取"
	}
	switch runtime.GOOS {
	case "windows":
		out, err := exec.Command("powershell", "-NoProfile", "-Command", fmt.Sprintf("(Get-Process -Id %d | Select-Object -ExpandProperty WorkingSet64)", pid)).Output()
		if err != nil {
			return "未获取"
		}
		value, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		if err != nil || value <= 0 {
			return "未获取"
		}
		return "RSS " + ui.FormatBytes(value)
	default:
		out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "rss=,%cpu=").Output()
		if err != nil {
			return "未获取"
		}
		fields := strings.Fields(string(out))
		if len(fields) < 2 {
			return "未获取"
		}
		rssKB, _ := strconv.ParseInt(fields[0], 10, 64)
		return fmt.Sprintf("RSS %s · CPU %s%%", ui.FormatBytes(rssKB*1024), fields[1])
	}
}

func buildAddressSnapshot(cfg *config.Config, report *egress.Report) addressSnapshot {
	if report == nil {
		return addressSnapshot{Current: "未获取", Entry: "未获取", Exit: "未获取"}
	}
	current := "未获取"
	exit := "未获取"
	if cfg.Extension.Mode == "chains" && cfg.Extension.ResidentialChain != nil && cfg.Extension.ResidentialChain.Mode == "global" {
		if report.ResidentialExit != nil {
			current = report.ResidentialExit.IP
			exit = report.ResidentialExit.IP
		}
	} else {
		if report.ProxyExit != nil {
			current = report.ProxyExit.IP
			exit = report.ProxyExit.IP
		}
		if report.ResidentialExit != nil && exit == "未获取" {
			exit = report.ResidentialExit.IP
		}
	}

	entry := "未获取"
	if report.AirportNode != nil {
		switch {
		case strings.TrimSpace(report.AirportNode.Resolved) != "":
			entry = report.AirportNode.Resolved
		case report.AirportNode.Location != nil && report.AirportNode.Location.IP != "":
			entry = report.AirportNode.Location.IP
		case strings.TrimSpace(report.AirportNode.Server) != "":
			entry = report.AirportNode.Server
		}
	}
	return addressSnapshot{Current: current, Entry: entry, Exit: exit}
}

func (m *runtimeConsoleModel) collectLatencySnapshot(client *mihomo.Client) latencySnapshot {
	sites := make(map[string]string, len(homeSiteTargets))
	if client == nil || !client.IsAvailable() {
		for _, target := range homeSiteTargets {
			sites[target.Name] = "未连接"
		}
		return latencySnapshot{Sites: sites}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, target := range homeSiteTargets {
		target := target
		wg.Add(1)
		go func() {
			defer wg.Done()
			delay, err := client.GetProxyDelay("Proxy", target.URL, 2*time.Second)
			value := "失败"
			if err == nil && delay > 0 {
				value = fmt.Sprintf("%dms", delay)
			}
			mu.Lock()
			sites[target.Name] = value
			mu.Unlock()
		}()
	}
	wg.Wait()
	return latencySnapshot{Sites: sites}
}

func buildConsoleAlerts(cfg *config.Config, network networkSnapshot) []alertMessage {
	alerts := make([]alertMessage, 0, 2)
	if network.Warning != "" {
		title := "TUN 未就绪"
		if !cfg.Runtime.Tun.Enabled {
			title = "局域网共享未开启"
		}
		alerts = append(alerts, alertMessage{
			level: "error",
			title: title,
			body:  network.Warning,
		})
	}
	return alerts
}

func buildOverviewSummary(subscription subscriptionSnapshot, node nodeSnapshot, network networkSnapshot, traffic trafficSnapshot) string {
	return strings.Join([]string{
		"订阅 " + fallbackText(subscription.Name, "未设置"),
		"策略 " + fallbackText(node.PolicyMode, "规则"),
		"节点 " + fallbackText(node.Node, "未识别"),
		"TUN " + onOff(network.TunEnabled),
		fmt.Sprintf("连接 %d", traffic.ActiveConnections),
	}, "  ·  ")
}

func renderHomeDashboardLines(s snapshot) []string {
	lines := []string{
		renderSectionTitle("订阅情况"),
		"  当前订阅: " + fallbackText(s.subscription.Name, "未设置"),
		"  来源: " + fallbackText(s.subscription.Source, "未设置"),
		"  来源站点: " + fallbackText(s.subscription.SourceHost, "未获取"),
		"  到期时间: " + fallbackText(s.subscription.ExpiryText, "未获取"),
		"  剩余流量: " + fallbackText(s.subscription.RemainingText, "未获取"),
		"  流量进度: " + renderUsageBar(s.subscription.UsagePercent) + "  " + fallbackText(s.subscription.UsageText, "总量未知"),
		"",
		renderSectionTitle("当前节点"),
		"  工作模式: " + fallbackText(s.node.PolicyMode, "规则"),
		"  当前分组: " + fallbackText(s.node.Group, "-"),
		"  策略组: " + fallbackText(s.node.Strategy, "未识别"),
		"  当前节点: " + fallbackText(s.node.Node, "未识别"),
		"",
		renderSectionTitle("网络设置"),
		"  TUN: " + tuiOnOff(s.network.TunEnabled),
		"  TUN 接口: " + fallbackText(s.network.TunInterface, "未就绪"),
		"  局域网共享: " + tuiState(s.network.LanSharing, "可用", "不可用"),
		"  本机绕过代理: " + tuiOnOff(s.network.BypassLocal),
	}
	if s.network.Warning != "" {
		lines = append(lines, "  警告: "+s.network.Warning)
	}

	lines = append(lines,
		"",
		renderSectionTitle("流量统计"),
		fmt.Sprintf("  上行速度: %s/s", ui.FormatBytes(s.traffic.UploadSpeed)),
		fmt.Sprintf("  下行速度: %s/s", ui.FormatBytes(s.traffic.DownloadSpeed)),
		fmt.Sprintf("  活跃连接: %d", s.traffic.ActiveConnections),
		"  上传总量: "+ui.FormatBytes(s.traffic.UploadTotal),
		"  下载总量: "+ui.FormatBytes(s.traffic.DownloadTotal),
		"  内核占用: "+fallbackText(s.traffic.KernelUsage, "未获取"),
		"  上行趋势: "+renderTrendSparkline(s.traffic.UploadTrend),
		"  下行趋势: "+renderTrendSparkline(s.traffic.DownloadTrend),
		"",
		renderSectionTitle("IP 链路"),
		"  当前 IP: "+fallbackText(s.addresses.Current, "未获取"),
		"  入口 IP: "+fallbackText(s.addresses.Entry, "未获取"),
		"  出口 IP: "+fallbackText(s.addresses.Exit, "未获取"),
		"",
		renderSectionTitle("常用站点延迟"),
	)
	for _, target := range homeSiteTargets {
		lines = append(lines, fmt.Sprintf("  %-8s %s", target.Name+":", fallbackText(s.latency.Sites[target.Name], "未测")))
	}
	return lines
}

func renderTrafficDetailLines(s snapshot) []string {
	return []string{
		renderSectionTitle("连接与流量"),
		fmt.Sprintf("  活跃连接: %d", s.traffic.ActiveConnections),
		fmt.Sprintf("  上行速度: %s/s", ui.FormatBytes(s.traffic.UploadSpeed)),
		fmt.Sprintf("  下行速度: %s/s", ui.FormatBytes(s.traffic.DownloadSpeed)),
		"  上传总量: " + ui.FormatBytes(s.traffic.UploadTotal),
		"  下载总量: " + ui.FormatBytes(s.traffic.DownloadTotal),
		"  内核占用: " + fallbackText(s.traffic.KernelUsage, "未获取"),
		"",
		renderSectionTitle("最近 10 次刷新"),
		"  上行趋势: " + renderTrendSparkline(s.traffic.UploadTrend),
		"  下行趋势: " + renderTrendSparkline(s.traffic.DownloadTrend),
		"",
		noteLine("按 R 刷新可以重新拉取当前连接、速度和趋势数据。"),
	}
}

func renderLatencyDetailLines(s snapshot) []string {
	lines := []string{
		renderSectionTitle("IP 与延迟"),
		"  当前 IP: " + fallbackText(s.addresses.Current, "未获取"),
		"  入口 IP: " + fallbackText(s.addresses.Entry, "未获取"),
		"  出口 IP: " + fallbackText(s.addresses.Exit, "未获取"),
		"",
		renderSectionTitle("常用站点"),
	}
	for _, target := range homeSiteTargets {
		lines = append(lines, fmt.Sprintf("  %-8s %s", target.Name+":", fallbackText(s.latency.Sites[target.Name], "未测")))
	}
	lines = append(lines,
		"",
		noteLine("这些延迟会跟随当前代理链路变化；切节点后按 R 刷新最直观。"),
	)
	return lines
}

func (m *runtimeConsoleModel) renderEgressStatusLines() []string {
	report := egress.Collect(m.cfg, m.dataDir, m.client)
	lines := []string{
		renderSectionTitle("出口网络"),
		"  当前出口: " + fallbackText(m.snapshot.addresses.Current, "未获取"),
		"  入口 IP: " + fallbackText(m.snapshot.addresses.Entry, "未获取"),
		"  出口 IP: " + fallbackText(m.snapshot.addresses.Exit, "未获取"),
		"",
		renderSectionTitle("链路详情"),
	}
	lines = append(lines, renderEgressDetailLines(m.cfg, report)...)
	lines = append(lines,
		"",
		noteLine("如果刚切完节点或 chains 模式，按 R 再刷一次，这里最直观。"),
	)
	return lines
}

func renderUsageBar(percent int) string {
	if percent <= 0 {
		return "[??????????]"
	}
	filled := clampPercent(percent) / 10
	if filled > 10 {
		filled = 10
	}
	return "[" + strings.Repeat("=", filled) + strings.Repeat(".", 10-filled) + fmt.Sprintf("] %d%%", clampPercent(percent))
}

func clampPercent(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func renderTrendSparkline(values []int64) string {
	if len(values) == 0 {
		return "暂无样本"
	}
	const bars = "▁▂▃▄▅▆▇█"
	var maxValue int64
	for _, value := range values {
		if value > maxValue {
			maxValue = value
		}
	}
	if maxValue <= 0 {
		return strings.Repeat("▁", len(values))
	}
	var b strings.Builder
	for _, value := range values {
		idx := int(float64(value) / float64(maxValue) * float64(len([]rune(bars))-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len([]rune(bars)) {
			idx = len([]rune(bars)) - 1
		}
		b.WriteRune([]rune(bars)[idx])
	}
	return b.String()
}

func shortHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := neturl.Parse(raw)
	if err != nil {
		return raw
	}
	return parsed.Hostname()
}
