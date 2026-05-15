package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestRenderExternalSource(t *testing.T) {
	cfg := config.Default()
	cfg.Source.Type = config.SourceTypeExternal
	cfg.Source.External.Server = "192.0.2.10"
	cfg.Source.External.Port = 7890
	cfg.Source.External.Kind = "http"

	out, err := Render(context.Background(), cfg, t.TempDir())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "mixed-port: 17890") {
		t.Errorf("missing mixed port substitution (默认 17890)")
	}
	if !strings.Contains(s, "mode: rule") {
		t.Errorf("mihomo mode not substituted")
	}
	if !strings.Contains(s, "MATCH,Proxy") {
		t.Errorf("rules not rendered")
	}
	if !strings.Contains(s, "192.0.2.10") {
		t.Errorf("upstream server not rendered")
	}
	if !strings.Contains(s, "tun:") || !strings.Contains(s, "enable: true") {
		t.Errorf("tun block not rendered:\n%s", s)
	}
}

func TestRenderIPv6Enabled(t *testing.T) {
	// v3.0 误从 v1 模板继承了 ipv6: false，导致双栈目标在 IPv6 优先客户端上
	// 解析失败；v3.3.2 恢复 v2 行为（全局 IPv6 + DNS AAAA 都开）。
	// 有用户反馈「2.x 支持 IPv6，3.x 之后不支持」就是这个回归。
	cfg := config.Default()
	cfg.Source.Type = config.SourceTypeExternal
	cfg.Source.External.Server = "127.0.0.1"
	cfg.Source.External.Port = 7890
	cfg.Source.External.Kind = "http"

	out, err := Render(context.Background(), cfg, t.TempDir())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "\nipv6: true\n") {
		t.Errorf("global ipv6 must be enabled; got snippet:\n%s",
			contextAround(s, "ipv6", 80))
	}
	if !strings.Contains(s, "  ipv6: true\n") {
		t.Errorf("dns.ipv6 must be enabled; got snippet:\n%s",
			contextAround(s, "ipv6", 80))
	}
	if strings.Contains(s, "ipv6: false") {
		t.Errorf("no ipv6: false should remain after v3.3.2 fix")
	}
}

// contextAround 返回 needle 附近的 pad 字节片段，失败时方便看上下文。
func contextAround(s, needle string, pad int) string {
	idx := strings.Index(s, needle)
	if idx < 0 {
		return "<" + needle + " not found>"
	}
	start := idx - pad
	if start < 0 {
		start = 0
	}
	end := idx + len(needle) + pad
	if end > len(s) {
		end = len(s)
	}
	return s[start:end]
}

func TestRenderDirectMode(t *testing.T) {
	cfg := config.Default()
	cfg.Source.Type = config.SourceTypeNone
	cfg.Traffic.Mode = config.ModeDirect

	out, err := Render(context.Background(), cfg, t.TempDir())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "mode: direct") {
		t.Errorf("expected mode: direct")
	}
	if strings.Contains(s, "MATCH,Proxy") {
		t.Errorf("direct mode should not emit MATCH,Proxy: %s", s)
	}
}

func TestRenderTUNOff(t *testing.T) {
	cfg := config.Default()
	cfg.Source.Type = config.SourceTypeNone
	cfg.Gateway.TUN.Enabled = false
	out, err := Render(context.Background(), cfg, t.TempDir())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(out), "enable: false") {
		t.Errorf("TUN off not rendered")
	}
}

func TestRenderLocalExternalProxyKeepsGatewayWithLocalBypass(t *testing.T) {
	cfg := config.Default()
	cfg.Gateway.Enabled = true
	cfg.Gateway.TUN.Enabled = false
	cfg.Gateway.TUN.BypassLocal = false
	cfg.Gateway.DNS.Enabled = false
	cfg.Source.Type = config.SourceTypeExternal
	cfg.Source.External.Server = "127.0.0.1"
	cfg.Source.External.Port = 6578
	cfg.Source.External.Kind = "socks5"

	out, err := Render(context.Background(), cfg, t.TempDir())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "tun:\n  enable: true\n") {
		t.Fatalf("local external proxy should keep TUN enabled:\n%s", contextAround(s, "tun:", 220))
	}
	if !strings.Contains(s, "  strict-route: false\n") {
		t.Fatalf("local external proxy should force strict-route false:\n%s", contextAround(s, "tun:", 260))
	}
	if !strings.Contains(s, "dns:\n  enable: true\n") {
		t.Fatalf("local external proxy should keep DNS listener enabled:\n%s", contextAround(s, "dns:", 220))
	}
}
