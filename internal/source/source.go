// Package source implements the "extension" feature: turning a user's proxy
// source (local port / subscription / file / single remote / none) into the
// mihomo YAML fragment that defines `proxies:`, `proxy-providers:` and
// `proxy-groups:` — keyed as the `Proxy` group so traffic rules can target it.
package source

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

// Fragment is the materialized YAML block inserted into mihomo's config.
type Fragment struct {
	YAML    string // starts with "proxies:" or "proxy-providers:" etc.
	Summary string // human-readable label ("订阅 · 96 nodes" / "本机 127.0.0.1:7890")
}

// Materialize produces the Fragment for this source config.
// It may perform IO (HTTP fetch) — caller should pass a context with timeout.
func Materialize(ctx context.Context, src config.SourceConfig, workDir string) (Fragment, error) {
	switch src.Type {
	case config.SourceTypeExternal:
		return materializeExternal(src.External), nil
	case config.SourceTypeSubscription:
		return materializeSubscription(ctx, src.Subscription, workDir)
	case config.SourceTypeFile:
		return materializeFile(src.File)
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

func materializeSubscription(ctx context.Context, s config.SubscriptionSource, workDir string) (Fragment, error) {
	// Download once per start. We don't poll; user can click "refresh" in the TUI.
	providerFile := "subscription.yaml"
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return Fragment{}, err
	}
	dst := workDir + "/" + providerFile
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return Fragment{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "clash-meta/1.18")
	client := &http.Client{Timeout: 30 * time.Second}
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
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		return Fragment{}, err
	}
	return Fragment{
		YAML:    renderProviderBlock(dst, "http", s.URL),
		Summary: fmt.Sprintf("订阅 · %s", s.Name),
	}, nil
}

// --- file: load local Clash config ---

func materializeFile(f config.FileSource) (Fragment, error) {
	info, err := os.Stat(f.Path)
	if err != nil {
		return Fragment{}, fmt.Errorf("file source %s: %w", f.Path, err)
	}
	if info.IsDir() {
		return Fragment{}, fmt.Errorf("file source must be a file: %s", f.Path)
	}
	return Fragment{
		YAML:    renderProviderBlock(f.Path, "file", ""),
		Summary: fmt.Sprintf("本地文件 · %s", f.Path),
	}, nil
}

func renderProviderBlock(path, kind, url string) string {
	var b strings.Builder
	b.WriteString("proxy-providers:\n")
	b.WriteString("  subscription:\n")
	if kind == "http" {
		b.WriteString(fmt.Sprintf("    type: http\n    url: %q\n    interval: 3600\n    path: %q\n", url, path))
	} else {
		b.WriteString(fmt.Sprintf("    type: file\n    path: %q\n", path))
	}
	b.WriteString(`    health-check:
      enable: true
      url: http://www.gstatic.com/generate_204
      interval: 300
proxy-groups:
  - name: Proxy
    type: select
    use: [subscription]
    proxies: [Auto, DIRECT]
  - name: Auto
    type: url-test
    use: [subscription]
    url: http://www.gstatic.com/generate_204
    interval: 300
    tolerance: 50
`)
	return b.String()
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
