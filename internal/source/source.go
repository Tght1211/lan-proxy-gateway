// Package source implements the "extension" feature: turning a user's proxy
// source (local port / subscription / file / single remote / none) into the
// mihomo YAML fragment that defines `proxies:`, `proxy-providers:` and
// `proxy-groups:` — keyed as the `Proxy` group so traffic rules can target it.
package source

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

// Fragment is the materialized YAML block inserted into mihomo's config.
type Fragment struct {
	YAML    string   // starts with "proxies:" or "proxy-providers:" etc.
	Rules   []string // user-supplied rules (订阅/文件 inline 出来的，engine 会 prepend 到 base rules 前)
	Summary string   // human-readable label ("订阅 · 96 nodes" / "本机 127.0.0.1:7890")
}

// MaterializeOptions tweaks runtime-only behavior without changing persisted config.
type MaterializeOptions struct {
	SubscriptionProxyURL string
}

// Materialize produces the Fragment for this source config.
// It may perform IO (HTTP fetch) — caller should pass a context with timeout.
func Materialize(ctx context.Context, src config.SourceConfig, workDir string) (Fragment, error) {
	return MaterializeWithOptions(ctx, src, workDir, MaterializeOptions{})
}

// MaterializeWithOptions is the runtime-aware variant used by engine reload paths.
func MaterializeWithOptions(ctx context.Context, src config.SourceConfig, workDir string, opts MaterializeOptions) (Fragment, error) {
	switch src.Type {
	case config.SourceTypeExternal:
		return materializeExternal(src.External), nil
	case config.SourceTypeSubscription:
		proxyURL := opts.SubscriptionProxyURL
		if proxyURL == "" {
			proxyURL = firstUpstreamProxyURL(src)
		}
		return materializeSubscription(ctx, src.Subscription, workDir, proxyURL)
	case config.SourceTypeFile:
		return materializeFile(src.File, workDir)
	case config.SourceTypeRemote:
		return materializeRemote(src.Remote), nil
	case config.SourceTypeNone:
		return materializeNone(), nil
	default:
		return Fragment{}, fmt.Errorf("unsupported source type: %q", src.Type)
	}
}

// --- external / remote: single-proxy shapes ---

func materializeExternal(e config.ExternalProxy) Fragment {
	kind := strings.ToLower(e.Kind)
	proxy := formatSingleProxy("Upstream", kind, e.Server, e.Port, "", "")
	return Fragment{
		YAML: singleProxyFragment(proxy),
		Summary: fmt.Sprintf("本机已有代理 %s:%d (%s)",
			e.Server, e.Port, strings.ToUpper(e.Kind)),
	}
}

func materializeRemote(r config.RemoteProxy) Fragment {
	kind := strings.ToLower(r.Kind)
	proxy := formatSingleProxy(r.Name, kind, r.Server, r.Port, r.Username, r.Password)
	return Fragment{
		YAML: singleProxyFragment(proxy),
		Summary: fmt.Sprintf("远程代理 %s:%d (%s)",
			r.Server, r.Port, strings.ToUpper(r.Kind)),
	}
}

func formatSingleProxy(name, kind, server string, port int, user, pass string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("  - name: %q\n", name))
	switch kind {
	case "socks5":
		b.WriteString("    type: socks5\n")
	default:
		b.WriteString("    type: http\n")
	}
	b.WriteString(fmt.Sprintf("    server: %s\n    port: %d\n", server, port))
	if user != "" {
		b.WriteString(fmt.Sprintf("    username: %q\n    password: %q\n", user, pass))
	}
	b.WriteString("    udp: true\n")
	return b.String()
}

func singleProxyFragment(proxyYaml string) string {
	var b strings.Builder
	b.WriteString("proxies:\n")
	b.WriteString(proxyYaml)
	b.WriteString(`proxy-groups:
  - name: Proxy
    type: select
    proxies:
      - Upstream
      - DIRECT
  - name: Auto
    type: fallback
    proxies:
      - Upstream
    url: http://www.gstatic.com/generate_204
    interval: 300
`)
	// If the single node isn't called "Upstream" (remote case), fix the group reference.
	// We'll only run the replacer if it's not already "Upstream".
	if !strings.Contains(proxyYaml, `name: "Upstream"`) {
		// Extract name from proxyYaml's first `- name: "…"` line.
		first := strings.Split(proxyYaml, "\n")[0]
		if idx := strings.Index(first, `name: `); idx >= 0 {
			name := strings.Trim(first[idx+len("name: "):], `" `)
			return strings.ReplaceAll(b.String(), "Upstream", name)
		}
	}
	return b.String()
}

// --- subscription: fetch and inline ---
//
// 把订阅 yaml 下载到 workdir 做备份（方便调试 / 下次启动离线用），
// 但真正给 mihomo 的是 inline 的 proxies + proxy-groups + rules。
func materializeSubscription(ctx context.Context, s config.SubscriptionSource, workDir string, proxyURL string) (Fragment, error) {
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return Fragment{}, err
	}
	dst := filepath.Join(workDir, "subscription.yaml")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return Fragment{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "clash-meta/1.18")
	client := newSubscriptionClient(proxyURL)
	resp, err := client.Do(req)
	if err != nil {
		return Fragment{}, fmt.Errorf("fetch subscription: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return Fragment{}, fmt.Errorf("subscription HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		return Fragment{}, fmt.Errorf("read subscription: %w", err)
	}
	normalized, err := normalizeSubscriptionContent(data)
	if err != nil {
		return Fragment{}, fmt.Errorf("normalize subscription: %w", err)
	}
	_ = os.WriteFile(dst, normalized, 0o600)

	frag, err := inlineUserYAML(normalized)
	if err != nil {
		return Fragment{}, fmt.Errorf("解析订阅 yaml: %w", err)
	}
	frag.Summary = fmt.Sprintf("订阅 · %s", s.Name)
	return frag, nil
}

// --- file: load local Clash/mihomo YAML ---
//
// 以前用 proxy-provider 加载，但 mihomo 的 provider 只读 proxies: 字段，
// 用户 yaml 里自己的 proxy-groups 和 rules 会被整体扔掉。所以这里改成
// inline：读 yaml → 提 proxies + proxy-groups + rules → 直接嵌进最终
// mihomo config.yaml。script enhancer 和「切换节点」菜单都能看到完整内容。
func materializeFile(f config.FileSource, workDir string) (Fragment, error) {
	info, err := os.Stat(f.Path)
	if err != nil {
		return Fragment{}, fmt.Errorf("本地配置文件 %s: %w", f.Path, err)
	}
	if info.IsDir() {
		return Fragment{}, fmt.Errorf("本地配置文件必须是文件，不是目录: %s", f.Path)
	}
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return Fragment{}, fmt.Errorf("读不了 %s: %w", f.Path, err)
	}
	normalized, err := normalizeSubscriptionContent(data)
	if err != nil {
		return Fragment{}, fmt.Errorf("标准化 %s 失败: %w", f.Path, err)
	}
	frag, err := inlineUserYAML(normalized)
	if err != nil {
		return Fragment{}, fmt.Errorf("解析 %s: %w", f.Path, err)
	}
	frag.Summary = fmt.Sprintf("本地文件 · %s", f.Path)
	return frag, nil
}

// inlineUserYAML 从 Clash/mihomo 订阅 yaml 抽出 proxies / proxy-groups / rules
// 三块，其他字段（mode / dns / tun / port / external-controller 之类顶层配置）
// 一律丢弃 —— 它们由 lan-proxy-gateway 的 base template 自己渲染。
//
// 如果用户 yaml 里没有「Proxy」组，补一个 select 组指向 DIRECT，让 base rules
// 里的 `MATCH,Proxy`（rule 模式兜底）不会挂。
func inlineUserYAML(data []byte) (Fragment, error) {
	var doc struct {
		Proxies     []yaml.Node `yaml:"proxies"`
		ProxyGroups []yaml.Node `yaml:"proxy-groups"`
		Rules       []string    `yaml:"rules"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Fragment{}, err
	}

	// 检查用户是否有 Proxy 组
	hasProxyGroup := false
	for _, g := range doc.ProxyGroups {
		name := groupNameFromNode(g)
		if name == "Proxy" {
			hasProxyGroup = true
			break
		}
	}

	// 没 Proxy 组就先补到结构体里，再统一 yaml.Marshal，避免手拼缩进把
	// `- name: Proxy` 掉出 `proxy-groups:` 列表，后续增强脚本再 yaml.Unmarshal
	// 整份 config 时就会炸（用户现场报的 did not find expected key 就是这个）。
	if !hasProxyGroup {
		fallback := firstGroupName(doc.ProxyGroups)
		if fallback == "" {
			fallback = "DIRECT"
		}
		proxyGroupYAML := fmt.Sprintf(`name: Proxy
type: select
proxies:
  - %s
  - DIRECT
`, fallback)
		var proxyGroupNode yaml.Node
		if err := yaml.Unmarshal([]byte(proxyGroupYAML), &proxyGroupNode); err != nil {
			return Fragment{}, fmt.Errorf("构造兜底 Proxy 组失败: %w", err)
		}
		if len(proxyGroupNode.Content) > 0 {
			doc.ProxyGroups = append(doc.ProxyGroups, *proxyGroupNode.Content[0])
		}
	}

	extract := map[string]interface{}{}
	if len(doc.Proxies) > 0 {
		extract["proxies"] = doc.Proxies
	}
	if len(doc.ProxyGroups) > 0 {
		extract["proxy-groups"] = doc.ProxyGroups
	}
	out, err := yaml.Marshal(extract)
	if err != nil {
		return Fragment{}, err
	}

	return Fragment{
		YAML:  string(out),
		Rules: doc.Rules,
	}, nil
}

// groupNameFromNode 从 yaml.Node（映射）里抽出 name 字段，失败返回 ""。
func normalizeSubscriptionContent(data []byte) ([]byte, error) {
	// 1) 去 UTF-8 BOM
	text := strings.TrimPrefix(string(data), "\ufeff")

	// 2) 有些 provider 会在 YAML 头塞 #!MANAGED-CONFIG / 更新时间注释，
	// yaml 本身能吃注释，但我们统一做一次 trim，顺手干掉前导空白。
	text = strings.TrimLeft(text, "\r\n \t")

	// 3) 一些订阅接口返回的不是 YAML，而是 base64(Clash YAML)。Clash Verge 等
	// 客户端会自动尝试解；这里也跟上。只在"看起来像 base64 且解出后像 YAML"
	// 时才替换，避免误伤正常明文 YAML。
	if maybe, ok := tryDecodeBase64YAML(text); ok {
		text = maybe
	}

	return []byte(text), nil
}

func tryDecodeBase64YAML(text string) (string, bool) {
	compact := strings.Map(func(r rune) rune {
		switch r {
		case '\r', '\n', '\t', ' ':
			return -1
		default:
			return r
		}
	}, text)
	if compact == "" {
		return "", false
	}
	// 明显是 YAML 顶层键就别误判成 base64 了。
	if looksLikeYAML(text) {
		return "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(compact)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(compact)
		if err != nil {
			return "", false
		}
	}
	decodedText := strings.TrimPrefix(string(decoded), "\ufeff")
	decodedText = strings.TrimLeft(decodedText, "\r\n \t")
	if !looksLikeYAML(decodedText) {
		return "", false
	}
	return decodedText, true
}

func looksLikeYAML(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for _, key := range []string{"proxies:", "proxy-groups:", "rules:", "port:", "mixed-port:", "mode:", "dns:"} {
			if strings.HasPrefix(line, key) {
				return true
			}
		}
		return false
	}
	return false
}

// groupNameFromNode 从 yaml.Node（映射）里抽出 name 字段，失败返回 ""。
func groupNameFromNode(n yaml.Node) string {
	if n.Kind != yaml.MappingNode {
		return ""
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		v := n.Content[i+1]
		if k.Value == "name" && v.Kind == yaml.ScalarNode {
			return v.Value
		}
	}
	return ""
}

func firstGroupName(groups []yaml.Node) string {
	for _, g := range groups {
		if n := groupNameFromNode(g); n != "" {
			return n
		}
	}
	return ""
}

// LocalMixedProxyURL formats the local mihomo mixed port as an HTTP proxy URL.
func LocalMixedProxyURL(port int) string {
	if port <= 0 {
		return ""
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func newSubscriptionClient(proxyURL string) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(u)
		}
	}
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}
}

func firstUpstreamProxyURL(upstreams ...config.SourceConfig) string {
	for _, src := range upstreams {
		switch src.Type {
		case config.SourceTypeExternal:
			if u := proxyURLForProxy(src.External.Kind, src.External.Server, src.External.Port, "", ""); u != "" {
				return u
			}
		case config.SourceTypeRemote:
			if u := proxyURLForProxy(src.Remote.Kind, src.Remote.Server, src.Remote.Port, src.Remote.Username, src.Remote.Password); u != "" {
				return u
			}
		}
	}
	return ""
}

func proxyURLForProxy(kind, server string, port int, username, password string) string {
	if strings.TrimSpace(server) == "" || port <= 0 || port > 65535 {
		return ""
	}
	scheme := "http"
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "socks", "socks5", "socks5h":
		scheme = "socks5"
	}
	u := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(server, strconv.Itoa(port)),
	}
	if username != "" {
		u.User = url.UserPassword(username, password)
	}
	return u.String()
}

// --- none: only DIRECT available ---

func materializeNone() Fragment {
	return Fragment{
		YAML: `proxies: []
proxy-groups:
  - name: Proxy
    type: select
    proxies: [DIRECT]
`,
		Summary: "未配置代理，所有流量直连",
	}
}
