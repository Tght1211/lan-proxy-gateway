package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/engine"
	"github.com/tght/lan-proxy-gateway/internal/gateway"
	"github.com/tght/lan-proxy-gateway/internal/traffic/rulesets"
	"github.com/tght/lan-proxy-gateway/internal/webui"
)

// WebUIController 把 *App 适配成 webui.Controller，让 internal/webui 不需要
// import internal/app（避免 webui → app → webui 循环）。
//
// 设计上跟 cmd/start.go 用的是同一个 App 实例，所以 webui 改的配置和 CLI 菜单
// 改的是同一个 gateway.yaml；mihomo Reload / Restart 也都走同一条路径。
//
// 并发模型：HTTP server 用多个 goroutine 同时调本类方法，必须保证：
//  1. 同一时刻只有一个写者修改 `c.app.Cfg`（避免字段撕裂）
//  2. 读者（Snapshot）拿到一致快照（不能读一半被另一个写者改了）
//
// 用 sync.RWMutex：所有 Set* 上 Lock，Snapshot 上 RLock。
type WebUIController struct {
	app *App
	mu  sync.RWMutex
}

// NewWebUIController 包一个适配器；调用方一般是 cmd/start.go。
func NewWebUIController(a *App) *WebUIController {
	return &WebUIController{app: a}
}

// Snapshot 把 app 的状态打成 webui.Snapshot，给前端 GET /api/status 用。
// 前端默认 8s 轮询 + 改东西后立即 GET，并发频次较高。RLock 保证读到的 Cfg 字段
// 一致（不被另一个写 goroutine 改一半）。
func (c *WebUIController) Snapshot() webui.Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cfg := c.app.Cfg
	effective := config.EffectiveRuntimeConfig(cfg)
	gs := gateway.Status{}
	if c.app.Gateway != nil {
		gs, _ = c.app.Gateway.Status()
	}

	return webui.Snapshot{
		Version:            versionString(),
		Platform:           runtime.GOOS,
		Running:            c.app.Engine != nil && c.app.Engine.Running(),
		LocalIP:            gs.LocalIP,
		Router:             gs.Router,
		GatewayMode:        effective.Gateway.Mode,
		GatewayModeLabel:   accessCapabilityLabel(effective),
		TUNEnabled:         effective.Gateway.TUN.Enabled,
		DNSEnabled:         effective.Gateway.DNS.Enabled,
		DNSPort:            effective.Gateway.DNS.Port,
		TrafficMode:        effective.Traffic.Mode,
		Adblock:            effective.Traffic.Adblock,
		AutoGroups:         effective.Traffic.AutoGroups,
		Rulesets:           rulesetsToPayload(effective.Traffic.Rulesets),
		RulesetDescriptors: buildRulesetDescriptors(effective.Traffic.Rulesets),
		Rules:              rulesToPayload(effective.Traffic.Extras),
		SourceType:         cfg.Source.Type,
		Source:             sourceToPayload(cfg.Source),
		SourceRuntime:      c.sourceRuntimeSnapshot(cfg.Source),
		SourceProfiles:     sourceProfilesToPayloads(cfg.Source),
		Script:             scriptToPayload(cfg.Source),
		ProxyService:       proxyServiceToPayload(effective.Runtime.ProxyService),
		MixedPort:          effective.Runtime.Ports.Mixed,
		RedirPort:          effective.Runtime.Ports.Redir,
		MihomoAPIPort:      effective.Runtime.Ports.API,
		WebUIPort:          effective.Runtime.Ports.WebUI,
		MixedPortDown:      false,
		Connectivity:       buildConnectivity(gs.LocalIP, gs.Router, effective),
	}
}

func (c *WebUIController) sourceRuntimeSnapshot(src config.SourceConfig) webui.SourceRuntime {
	rt := webui.SourceRuntime{
		Label:      sourceLabel(src),
		Detail:     sourceDetail(src),
		Status:     "unknown",
		StatusText: "等待检测",
	}
	if src.Type == config.SourceTypeNone || src.Type == "" {
		rt.Status = "ok"
		rt.StatusText = "直连"
	}
	health := c.app.Health()
	if !health.CheckedAt.IsZero() {
		rt.CheckedAt = health.CheckedAt.Format(time.RFC3339)
		rt.FallbackActive = health.FallbackActive
		rt.LastError = health.LastError
		if health.Healthy {
			rt.Status = "ok"
			rt.StatusText = "源可用"
		} else if health.FallbackActive {
			rt.Status = "bad"
			rt.StatusText = "源不可用，已临时直连"
		} else {
			rt.Status = "warn"
			rt.StatusText = "源异常"
		}
	}

	if c.app.Engine == nil || !c.app.Engine.Running() {
		if rt.Status == "unknown" {
			rt.StatusText = "服务未运行"
		}
		return rt
	}
	ctx, cancel := context.WithTimeout(context.Background(), 900*time.Millisecond)
	defer cancel()
	groups, err := c.app.Engine.API().ListProxyGroups(ctx)
	if err != nil {
		if rt.Status == "unknown" {
			rt.StatusText = "无法读取当前节点"
		}
		return rt
	}
	group := pickActiveProxyGroup(groups)
	if group.Name == "" {
		return rt
	}
	rt.ActiveGroup = group.Name
	rt.ActiveNode = group.Now
	if group.Now != "" {
		rt.ActivePath = group.Name + " -> " + group.Now
	}
	rt.AIRoute = aiRouteSummary(groups)
	return rt
}

func (c *WebUIController) SetGatewayMode(ctx context.Context, mode string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.app.SetGatewayMode(ctx, mode)
}

func (c *WebUIController) SetTUNEnabled(ctx context.Context, enabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.app.Cfg.Gateway.TUN.Enabled = enabled
	if enabled && c.app.Cfg.Gateway.Mode == "" {
		c.app.Cfg.Gateway.Mode = config.GatewayModeTUN
	}
	return c.saveAndReload(ctx)
}

func (c *WebUIController) SetTrafficMode(ctx context.Context, mode string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.app.SetMode(ctx, mode)
}

func (c *WebUIController) SetAdblock(ctx context.Context, enabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.app.Cfg.Traffic.Adblock = enabled
	return c.saveAndReload(ctx)
}

func (c *WebUIController) SetDNSEnabled(ctx context.Context, enabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.app.Cfg.Gateway.DNS.Enabled = enabled
	return c.saveAndReload(ctx)
}

func (c *WebUIController) SetAutoGroups(ctx context.Context, enabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.app.Cfg.Traffic.AutoGroups = enabled
	return c.saveAndReload(ctx)
}

func (c *WebUIController) SetRulesets(ctx context.Context, rs webui.Rulesets) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.app.Cfg.Traffic.Rulesets = config.RulesetToggles{
		ChinaDirect: rs.ChinaDirect,
		Apple:       rs.Apple,
		Nintendo:    rs.Nintendo,
		Global:      rs.Global,
		LANDirect:   rs.LANDirect,
	}
	return c.saveAndReload(ctx)
}

func (c *WebUIController) SetCustomRules(ctx context.Context, rs webui.CustomRules) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.app.Cfg.Traffic.Extras = config.ExtraRules{
		Direct: rs.Direct,
		Proxy:  rs.Proxy,
		Reject: rs.Reject,
		Groups: normalizeTargetedRules(rs.Groups),
	}
	return c.saveAndReload(ctx)
}

// SetScript 把"增强脚本"卡的三种模式落到 SourceConfig 里：
//
//	none    → 清掉 ScriptPath 和 ChainResidential
//	custom  → 写 ScriptPath；清掉 ChainResidential（避免它覆盖 ScriptPath）
//	preset  → 写 ChainResidential；ScriptPath 留空，render 阶段会自动生成预设
//	          脚本并指向那个文件（参考 internal/script/presets 包）
//
// 改完触发 hot-reload，让 mihomo 重新读取 config.yaml。
func (c *WebUIController) SetScript(ctx context.Context, p webui.ScriptPayload) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	mode := p.Mode
	if mode == "" {
		mode = "none"
	}
	switch mode {
	case "none":
		c.app.Cfg.Source.ScriptPath = ""
		c.app.Cfg.Source.ChainResidential = nil
	case "custom":
		// custom_path 来自 LAN 上的 HTTP 调用，必须强校验，防路径穿越 + 不存在文件
		// + 非 .js 后缀（goja 引擎只 eval JavaScript）：
		//   1) 必须是绝对路径（filepath.IsAbs）—— 阻断 "../../etc/passwd" 这类
		//   2) Clean 后比原 path 不能少（防止 "./a/../b" 这种弯绕)
		//   3) 必须存在且是常规文件（不是目录 / 软链 / device）
		//   4) 后缀必须 .js
		cp := strings.TrimSpace(p.CustomPath)
		if !filepath.IsAbs(cp) {
			return fmt.Errorf("script.custom_path 必须是绝对路径，得到 %q", cp)
		}
		if filepath.Clean(cp) != cp {
			return fmt.Errorf("script.custom_path 含 .. 或冗余分隔符：%q", cp)
		}
		if !strings.HasSuffix(strings.ToLower(cp), ".js") {
			return fmt.Errorf("script.custom_path 必须是 .js 文件")
		}
		info, err := os.Stat(cp)
		if err != nil {
			return fmt.Errorf("script.custom_path 不存在或不可读：%w", err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("script.custom_path 不是常规文件：%s", info.Mode().String())
		}
		c.app.Cfg.Source.ScriptPath = cp
		c.app.Cfg.Source.ChainResidential = nil
	case "preset":
		if p.ChainResidential == nil {
			return fmt.Errorf("preset 模式缺少 chain_residential")
		}
		dialer := p.ChainResidential.DialerProxy
		if dialer == "" {
			dialer = "🛫 AI起飞节点"
		}
		c.app.Cfg.Source.ChainResidential = &config.ChainResidentialConfig{
			Name:        p.ChainResidential.Name,
			Kind:        p.ChainResidential.Kind,
			Server:      p.ChainResidential.Server,
			Port:        p.ChainResidential.Port,
			Username:    p.ChainResidential.Username,
			Password:    p.ChainResidential.Password,
			DialerProxy: dialer,
		}
		// ScriptPath 不动；预设的 .js 会在 render 阶段被生成并指向。
	default:
		return fmt.Errorf("未知 script mode: %s", mode)
	}
	return c.saveAndReload(ctx)
}

func (c *WebUIController) SetSource(ctx context.Context, p webui.SourcePayload) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	src, err := payloadToSource(p, c.app.Cfg.Source)
	if err != nil {
		return err
	}
	return c.app.SetSource(ctx, src)
}

// SetPorts 改端口需要完整重启，因为 mihomo 启动时才读端口、bind 监听 socket。
// 0 = 该项不动；其它正整数 = 改成这个端口。
//
// 校验：
//   - 任意端口必须在 1024..65535（避开 privileged 段，DNS 53 除外）
//   - **拒绝从 WebUI 改 WebUI 自己端口** —— 改了之后 server 不会跟着重启，
//     浏览器响应 OK 后下一秒 fetch 不通，体感是"卡死"。让用户 yaml 改 + 重启 gateway 进程
//   - 五个端口互不冲突
func (c *WebUIController) SetPorts(ctx context.Context, p webui.Ports) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if p.WebUI != 0 && p.WebUI != c.app.Cfg.Runtime.Ports.WebUI {
		return fmt.Errorf("WebUI 端口不能在 Web 界面里改（改了之后 HTTP server 自己也会重建，请求会断）。请编辑 gateway.yaml 后 gateway stop && start。")
	}
	// 端口范围校验：mihomo / mixed / redir / api 强制 1024..65535（避开特权段，
	// 普通用户也能 bind），DNS 单独放行 1..65535（默认 53）。
	type chk struct {
		name string
		val  int
		min  int
	}
	for _, x := range []chk{
		{"mixed", p.Mixed, 1024},
		{"redir", p.Redir, 1024},
		{"api", p.API, 1024},
		{"dns", p.DNS, 1},
	} {
		if x.val == 0 {
			continue
		}
		if x.val < x.min || x.val > 65535 {
			return fmt.Errorf("%s 端口 %d 越界（合法范围 %d-65535）", x.name, x.val, x.min)
		}
	}
	// 冲突检查：把"会落盘的最终端口"摆一起去重。
	final := map[string]int{
		"mixed": pick(p.Mixed, c.app.Cfg.Runtime.Ports.Mixed),
		"redir": pick(p.Redir, c.app.Cfg.Runtime.Ports.Redir),
		"api":   pick(p.API, c.app.Cfg.Runtime.Ports.API),
		"webui": c.app.Cfg.Runtime.Ports.WebUI,
		"dns":   pick(p.DNS, c.app.Cfg.Gateway.DNS.Port),
	}
	seen := map[int]string{}
	for k, v := range final {
		if v <= 0 {
			continue
		}
		if prev, ok := seen[v]; ok {
			return fmt.Errorf("端口冲突：%s 与 %s 都使用 %d", k, prev, v)
		}
		seen[v] = k
	}

	cfg := c.app.Cfg
	if p.Mixed > 0 {
		cfg.Runtime.Ports.Mixed = p.Mixed
	}
	if p.Redir > 0 {
		cfg.Runtime.Ports.Redir = p.Redir
	}
	if p.API > 0 {
		cfg.Runtime.Ports.API = p.API
	}
	if p.DNS > 0 {
		cfg.Gateway.DNS.Port = p.DNS
	}
	if err := c.app.Save(); err != nil {
		return err
	}
	if c.app.Engine != nil && c.app.Engine.Running() {
		if err := c.app.Stop(); err != nil {
			return fmt.Errorf("停止失败: %w", err)
		}
		return c.app.Start(ctx)
	}
	return nil
}

func (c *WebUIController) SetProxyService(ctx context.Context, p webui.ProxyServicePayload) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.app.Cfg.Runtime.ProxyService.Enabled = config.BoolPtr(p.Enabled)
	c.app.Cfg.Runtime.ProxyService.Username = strings.TrimSpace(p.Username)
	c.app.Cfg.Runtime.ProxyService.Password = p.Password
	if err := c.app.Save(); err != nil {
		return err
	}
	if c.app.Engine != nil && c.app.Engine.Running() {
		if err := c.app.Stop(); err != nil {
			return fmt.Errorf("停止失败: %w", err)
		}
		return c.app.Start(ctx)
	}
	return nil
}

// pick 在 newVal > 0 时用 newVal，否则用 oldVal（"0 = 不改"语义）。
func pick(newVal, oldVal int) int {
	if newVal > 0 {
		return newVal
	}
	return oldVal
}

func (c *WebUIController) Reload(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.app.Engine == nil || !c.app.Engine.Running() {
		return fmt.Errorf("mihomo 没在跑")
	}
	return c.app.Engine.Reload(ctx, config.EffectiveRuntimeConfig(c.app.Cfg))
}

func (c *WebUIController) Restart(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.app.Stop(); err != nil {
		return fmt.Errorf("停止失败: %w", err)
	}
	return c.app.Start(ctx)
}

func (c *WebUIController) CheckUpdate(ctx context.Context) (webui.UpdateInfo, error) {
	current := versionString()
	info := webui.UpdateInfo{Current: current}
	reqCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, "https://api.github.com/repos/Tght1211/lan-proxy-gateway/releases/latest", nil)
	if err != nil {
		return info, err
	}
	req.Header.Set("User-Agent", "lan-proxy-gateway-webui")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return info, fmt.Errorf("检查更新失败: HTTP %d", resp.StatusCode)
	}
	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return info, err
	}
	info.Latest = release.TagName
	info.URL = release.HTMLURL
	info.Available = release.TagName != "" && release.TagName != current
	return info, nil
}

func (c *WebUIController) RunUpdate(ctx context.Context) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, self, "update", "latest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

func (c *WebUIController) saveAndReload(ctx context.Context) error {
	if err := c.app.Save(); err != nil {
		return err
	}
	if c.app.Engine != nil && c.app.Engine.Running() {
		return c.app.Engine.Reload(ctx, config.EffectiveRuntimeConfig(c.app.Cfg))
	}
	return nil
}

// --- 转换辅助 ---

func accessCapabilityLabel(cfg *config.Config) string {
	var parts []string
	if cfg.Gateway.TUN.Enabled {
		parts = append(parts, "透明代理已启用")
	}
	if cfg.Runtime.ProxyService.IsEnabled() {
		parts = append(parts, "代理服务已启用")
	}
	if len(parts) == 0 {
		return "接入能力未启用"
	}
	return strings.Join(parts, " / ")
}

func sourceLabel(s config.SourceConfig) string {
	switch s.Type {
	case config.SourceTypeSubscription:
		return "订阅 · " + fallbackText(s.Subscription.Name, "subscription")
	case config.SourceTypeExternal:
		return "本机已有代理"
	case config.SourceTypeFile:
		return "本地配置文件"
	case config.SourceTypeRemote:
		return "单节点 · " + fallbackText(s.Remote.Name, "remote")
	default:
		return "直连"
	}
}

func sourceDetail(s config.SourceConfig) string {
	switch s.Type {
	case config.SourceTypeSubscription:
		if u, err := url.Parse(s.Subscription.URL); err == nil && u.Host != "" {
			return u.Host
		}
		if s.Subscription.URL != "" {
			return "订阅链接"
		}
	case config.SourceTypeExternal:
		return fmt.Sprintf("%s://%s:%d", fallbackText(s.External.Kind, "http"), fallbackText(s.External.Server, "127.0.0.1"), s.External.Port)
	case config.SourceTypeFile:
		return s.File.Path
	case config.SourceTypeRemote:
		return fmt.Sprintf("%s://%s:%d", fallbackText(s.Remote.Kind, "socks5"), s.Remote.Server, s.Remote.Port)
	case config.SourceTypeNone, "":
		return "未使用代理源"
	}
	return ""
}

func fallbackText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func pickActiveProxyGroup(groups []engine.ProxyGroup) engine.ProxyGroup {
	if len(groups) == 0 {
		return engine.ProxyGroup{}
	}
	byName := make(map[string]engine.ProxyGroup, len(groups))
	for _, g := range groups {
		byName[g.Name] = g
	}
	for _, name := range []string{"Proxy", "PROXY", "🚀 节点选择", "GLOBAL"} {
		if g, ok := byName[name]; ok && strings.TrimSpace(g.Now) != "" {
			return withResolvedNode(g, byName)
		}
	}
	sorted := append([]engine.ProxyGroup(nil), groups...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Type != sorted[j].Type {
			return sorted[i].Type == "Selector"
		}
		return sorted[i].Name < sorted[j].Name
	})
	for _, g := range sorted {
		if g.Type == "Selector" && len(g.All) > 1 && strings.TrimSpace(g.Now) != "" {
			return withResolvedNode(g, byName)
		}
	}
	for _, g := range sorted {
		if strings.TrimSpace(g.Now) != "" {
			return withResolvedNode(g, byName)
		}
	}
	return engine.ProxyGroup{}
}

func withResolvedNode(g engine.ProxyGroup, groups map[string]engine.ProxyGroup) engine.ProxyGroup {
	cur := g.Now
	for i := 0; i < 6; i++ {
		next, ok := groups[cur]
		if !ok || strings.TrimSpace(next.Now) == "" {
			break
		}
		cur = next.Now
	}
	g.Now = cur
	return g
}

func aiRouteSummary(groups []engine.ProxyGroup) string {
	byName := make(map[string]engine.ProxyGroup, len(groups))
	for _, g := range groups {
		byName[g.Name] = g
	}
	takeoff, okTakeoff := byName["🛫 AI起飞节点"]
	landing, okLanding := byName["🛬 AI落地节点"]
	if !okTakeoff && !okLanding {
		return ""
	}
	takeoffNode := strings.TrimSpace(takeoff.Now)
	landingNode := strings.TrimSpace(landing.Now)
	if takeoffNode == "" {
		takeoffNode = "未选择"
	}
	if landingNode == "" {
		landingNode = "未选择"
	}
	return "🛫 " + takeoffNode + " -> 🛬 " + landingNode
}

func sourceToPayload(s config.SourceConfig) webui.SourcePayload {
	p := webui.SourcePayload{Type: s.Type}
	if s.Subscription.URL != "" || s.Subscription.Name != "" {
		p.Subscription = &webui.SubscriptionField{URL: s.Subscription.URL, Name: s.Subscription.Name}
	}
	if s.External.Server != "" || s.External.Port != 0 {
		p.External = &webui.ExternalField{
			Name:   s.External.Name,
			Server: s.External.Server,
			Port:   s.External.Port,
			Kind:   s.External.Kind,
		}
	}
	if s.File.Path != "" {
		p.File = &webui.FileField{Path: s.File.Path}
	}
	if s.Remote.Server != "" || s.Remote.Port != 0 {
		p.Remote = &webui.RemoteField{
			Name:     s.Remote.Name,
			Kind:     s.Remote.Kind,
			Server:   s.Remote.Server,
			Port:     s.Remote.Port,
			Username: s.Remote.Username,
			Password: s.Remote.Password,
		}
	}
	return p
}

func sourceProfilesToPayloads(s config.SourceConfig) []webui.SourcePayload {
	out := make([]webui.SourcePayload, 0, len(s.Profiles)+1)
	seen := map[string]bool{}
	add := func(p webui.SourcePayload) {
		key := p.Type
		switch p.Type {
		case config.SourceTypeSubscription:
			if p.Subscription != nil {
				key += ":" + p.Subscription.Name + ":" + p.Subscription.URL
			}
		case config.SourceTypeFile:
			if p.File != nil {
				key += ":" + p.File.Path
			}
		default:
			key += ":" + sourcePayloadLabel(p)
		}
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, p)
	}
	add(sourceToPayload(s))
	for _, prof := range s.Profiles {
		p := webui.SourcePayload{Type: prof.Type}
		if prof.Subscription != nil {
			p.Subscription = &webui.SubscriptionField{URL: prof.Subscription.URL, Name: firstNonEmpty(prof.Subscription.Name, prof.Name)}
		}
		if prof.File != nil {
			p.File = &webui.FileField{Path: prof.File.Path}
		}
		if prof.External != nil {
			p.External = &webui.ExternalField{Name: prof.External.Name, Server: prof.External.Server, Port: prof.External.Port, Kind: prof.External.Kind}
		}
		if prof.Remote != nil {
			p.Remote = &webui.RemoteField{Name: prof.Remote.Name, Kind: prof.Remote.Kind, Server: prof.Remote.Server, Port: prof.Remote.Port, Username: prof.Remote.Username, Password: prof.Remote.Password}
		}
		add(p)
	}
	return out
}

func sourcePayloadLabel(p webui.SourcePayload) string {
	if p.Subscription != nil {
		return p.Subscription.Name + p.Subscription.URL
	}
	if p.File != nil {
		return p.File.Path
	}
	if p.External != nil {
		return p.External.Server
	}
	if p.Remote != nil {
		return p.Remote.Name + p.Remote.Server
	}
	return p.Type
}

func proxyServiceToPayload(p config.ProxyServiceConfig) webui.ProxyServicePayload {
	return webui.ProxyServicePayload{
		Enabled:  p.IsEnabled(),
		Username: p.Username,
		Password: p.Password,
	}
}

func payloadToSource(p webui.SourcePayload, current config.SourceConfig) (config.SourceConfig, error) {
	out := current
	out.Type = p.Type
	if p.Subscription != nil {
		out.Subscription = config.SubscriptionSource{URL: p.Subscription.URL, Name: p.Subscription.Name}
	}
	if p.External != nil {
		out.External = config.ExternalProxy{
			Name:   p.External.Name,
			Server: p.External.Server,
			Port:   p.External.Port,
			Kind:   p.External.Kind,
		}
	}
	if p.File != nil {
		out.File = config.FileSource{Path: p.File.Path}
	}
	if p.Remote != nil {
		out.Remote = config.RemoteProxy{
			Name:     p.Remote.Name,
			Kind:     p.Remote.Kind,
			Server:   p.Remote.Server,
			Port:     p.Remote.Port,
			Username: p.Remote.Username,
			Password: p.Remote.Password,
		}
	}
	return out, nil
}

// buildRulesetDescriptors 把每个内置规则集的中文名 / verdict / 规则原文打包给前端，
// 让用户能展开看到"这个开关到底匹配什么"并复制完整规则集。
func buildRulesetDescriptors(r config.RulesetToggles) []webui.RulesetDescriptor {
	const sampleN = 8
	pick := func(s []string) []string {
		if len(s) <= sampleN {
			return append([]string(nil), s...)
		}
		return append([]string(nil), s[:sampleN]...)
	}
	cd := rulesets.ChinaDirect()
	ap := rulesets.Apple()
	ni := rulesets.Nintendo()
	gl := rulesets.Global()
	ld := rulesets.LANDirect()
	return []webui.RulesetDescriptor{
		{
			Key: "china_direct", Label: "国内域名直连", Verdict: "DIRECT",
			Note:  "GeoSite CN · 国内域名走本地直连，避免出墙绕路",
			Count: len(cd), Sample: pick(cd), Rules: append([]string(nil), cd...), Enabled: r.ChinaDirect,
		},
		{
			Key: "lan_direct", Label: "局域网直连", Verdict: "DIRECT",
			Note:  "私有网段（10/8 · 172.16/12 · 192.168/16）直出，本机也归入",
			Count: len(ld), Sample: pick(ld), Rules: append([]string(nil), ld...), Enabled: r.LANDirect,
		},
		{
			Key: "apple", Label: "Apple 服务", Verdict: "DIRECT",
			Note:  "iCloud / FaceTime / TestFlight / Push 等 Apple 自家域名直连",
			Count: len(ap), Sample: pick(ap), Rules: append([]string(nil), ap...), Enabled: r.Apple,
		},
		{
			Key: "nintendo", Label: "Nintendo 联机", Verdict: "Proxy",
			Note:  "Switch 联机域名走代理，常用于 eShop / Splatoon 加速",
			Count: len(ni), Sample: pick(ni), Rules: append([]string(nil), ni...), Enabled: r.Nintendo,
		},
		{
			Key: "global", Label: "全球代理", Verdict: "Proxy",
			Note:  "GeoSite Geolocation-!cn · 国外常用域名（YouTube / Google / GitHub 等）",
			Count: len(gl), Sample: pick(gl), Rules: append([]string(nil), gl...), Enabled: r.Global,
		},
	}
}

// scriptToPayload 读 SourceConfig 反推 ScriptPayload。
// 优先级：ChainResidential 设置过 → preset；否则有 ScriptPath → custom；否则 none。
// 跟 CLI 菜单看到的"当前增强脚本是什么"保持一致。
func scriptToPayload(s config.SourceConfig) webui.ScriptPayload {
	if s.ChainResidential != nil {
		return webui.ScriptPayload{
			Mode: "preset",
			ChainResidential: &webui.ChainResidentialPayload{
				Name:        s.ChainResidential.Name,
				Kind:        s.ChainResidential.Kind,
				Server:      s.ChainResidential.Server,
				Port:        s.ChainResidential.Port,
				Username:    s.ChainResidential.Username,
				Password:    s.ChainResidential.Password,
				DialerProxy: s.ChainResidential.DialerProxy,
			},
		}
	}
	if s.ScriptPath != "" {
		return webui.ScriptPayload{Mode: "custom", CustomPath: s.ScriptPath}
	}
	return webui.ScriptPayload{Mode: "none"}
}

func rulesetsToPayload(r config.RulesetToggles) webui.Rulesets {
	return webui.Rulesets{
		ChinaDirect: r.ChinaDirect,
		Apple:       r.Apple,
		Nintendo:    r.Nintendo,
		Global:      r.Global,
		LANDirect:   r.LANDirect,
	}
}

func rulesToPayload(r config.ExtraRules) webui.CustomRules {
	return webui.CustomRules{
		Direct: append([]string(nil), r.Direct...),
		Proxy:  append([]string(nil), r.Proxy...),
		Reject: append([]string(nil), r.Reject...),
		Groups: normalizeTargetedRules(r.Groups),
	}
}

func normalizeTargetedRules(groups []config.TargetedRules) []config.TargetedRules {
	if len(groups) == 0 {
		return nil
	}
	merged := make(map[string][]string)
	order := make([]string, 0, len(groups))
	for _, group := range groups {
		target := strings.TrimSpace(group.Target)
		if target == "" {
			continue
		}
		if _, ok := merged[target]; !ok {
			order = append(order, target)
		}
		for _, rule := range group.Rules {
			rule = strings.TrimSpace(rule)
			if rule == "" {
				continue
			}
			merged[target] = append(merged[target], rule)
		}
	}
	out := make([]config.TargetedRules, 0, len(order))
	for _, target := range order {
		rules := merged[target]
		if len(rules) == 0 {
			continue
		}
		out = append(out, config.TargetedRules{
			Target: target,
			Rules:  append([]string(nil), rules...),
		})
	}
	return out
}

// buildConnectivity 把"设备接入指引"打成结构化数据。前端按场景渲染表格 / 卡片。
// 内容跟 internal/gateway/guide.go 的 DeviceGuide 一致，但不带 ASCII 制表符。
func buildConnectivity(localIP, router string, eff *config.Config) webui.Connectivity {
	if localIP == "" {
		localIP = "<本机局域网 IP>"
	}
	if router == "" {
		router = "<路由器 IP>"
	}
	mixed := eff.Runtime.Ports.Mixed
	dnsPort := eff.Gateway.DNS.Port

	conn := webui.Connectivity{
		LocalIP:   localIP,
		Router:    router,
		MixedPort: mixed,
		DNSPort:   dnsPort,
	}

	tunOn := eff.Gateway.TUN.Enabled

	switch runtime.GOOS {
	case "windows":
		conn.Methods = []webui.AccessMethod{
			{
				Scenario:    "手机 / 电脑 / 浏览器",
				Recommended: "填代理",
				Fields: []webui.AccessField{
					{Label: "主机", Value: localIP, Mono: true},
					{Label: "端口", Value: itoa(mixed), Mono: true},
					{Label: "类型", Value: "HTTP / SOCKS5"},
				},
			},
		}
		conn.Notes = []string{
			"Windows 不支持改默认网关把流量重定向到本机，游戏机 / 智能设备请使用 macOS、Linux 或软路由。",
		}

	case "darwin":
		conn.Methods = []webui.AccessMethod{
			{
				Scenario:    "手机 / 电脑 / 浏览器",
				Recommended: "填代理",
				Fields: []webui.AccessField{
					{Label: "主机", Value: localIP, Mono: true},
					{Label: "端口", Value: itoa(mixed), Mono: true},
					{Label: "类型", Value: "HTTP / SOCKS5"},
				},
			},
			{
				// Switch 系统设置里能填 HTTP 代理，但 eShop / Splatoon 联机加速
				// 走得通；YouTube App 走的不是标准 HTTP 流量，只能靠 TUN 改路由。
				Scenario:    "Switch · 联机加速",
				Recommended: "填代理（仅 HTTP 流量生效）",
				Fields: []webui.AccessField{
					{Label: "主机", Value: localIP, Mono: true},
					{Label: "端口", Value: itoa(mixed), Mono: true},
					{Label: "类型", Value: "HTTP"},
				},
			},
			{
				Scenario:    "Switch · 看 YouTube / Twitch",
				Recommended: "改网关 + 改 DNS（需 TUN 模式）",
				Fields: []webui.AccessField{
					{Label: "默认网关", Value: localIP, Mono: true},
					{Label: "DNS", Value: localIP, Mono: true},
					{Label: "子网掩码", Value: "255.255.255.0", Mono: true},
				},
			},
			{
				Scenario:    "投影仪 / 电视 / IoT",
				Recommended: "改网关 + 改 DNS（需 TUN 模式）",
				Fields: []webui.AccessField{
					{Label: "默认网关", Value: localIP, Mono: true},
					{Label: "DNS", Value: localIP, Mono: true},
					{Label: "子网掩码", Value: "255.255.255.0", Mono: true},
				},
			},
		}
		notes := []string{
			"Switch：填代理只对 eShop / 系统级 HTTP(S) 生效，YouTube / Twitch App 自定义协议，必须 TUN 旁路由才能解锁。",
			"TUN 模式：网关 + DNS 都必须指向本机；只改网关只是 NAT，不会走代理。",
			"代理服务：HTTP/SOCKS5 mixed-port 可与 TUN 同时提供，适合能手动填写代理的设备。",
		}
		if !tunOn {
			notes = append([]string{
				"当前透明代理未启用：投影仪 / 智能设备 / Switch 解锁流媒体需先打开 TUN 旁路由。",
			}, notes...)
		}
		conn.Notes = notes

	default: // linux 等
		conn.Methods = []webui.AccessMethod{
			{
				Scenario:    "游戏机 / 电视 / IoT",
				Recommended: "改默认网关",
				Fields: []webui.AccessField{
					{Label: "默认网关", Value: localIP, Mono: true},
					{Label: "DNS", Value: localIP, Mono: true},
					{Label: "子网掩码", Value: "255.255.255.0", Mono: true},
				},
			},
			{
				Scenario:    "手机 / 电脑 / 浏览器",
				Recommended: "填代理",
				Fields: []webui.AccessField{
					{Label: "主机", Value: localIP, Mono: true},
					{Label: "端口", Value: itoa(mixed), Mono: true},
					{Label: "类型", Value: "HTTP / SOCKS5"},
				},
			},
		}
		conn.Notes = []string{
			"Linux 上 forward 模式走 iptables REDIRECT，设备改网关即可透明走代理；宿主机本身不受影响。",
		}
	}
	return conn
}

func itoa(i int) string {
	if i <= 0 {
		return "—"
	}
	const digits = "0123456789"
	if i < 10 {
		return string(digits[i])
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = digits[i%10]
		i /= 10
	}
	return string(b[pos:])
}

// versionString 在 ldflags 没注入时给个占位。InjectWebUIVersion 由 cmd/start 调。
var injectedVersion = "dev"

func InjectWebUIVersion(v string) { injectedVersion = v }
func versionString() string       { return injectedVersion }

// RulesetDescriptorsForPreview / ConnectivityForPreview 是给 scripts/webui_preview.go
// 喂假数据用的导出帮手。生产路径不依赖这俩；保留导出仅为了 preview 脚本不重复实现一份。
func RulesetDescriptorsForPreview(r config.RulesetToggles) []webui.RulesetDescriptor {
	return buildRulesetDescriptors(r)
}
func ConnectivityForPreview() webui.Connectivity {
	cfg := config.Default()
	cfg.Runtime.Ports.Mixed = 17890
	cfg.Gateway.DNS.Port = 53
	return buildConnectivity("192.168.12.100", "192.168.12.1", cfg)
}
