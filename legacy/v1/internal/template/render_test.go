package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestRenderTemplateIncludesInterfaceScopedTunWhenBypassLocalEnabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Tun.Enabled = true
	cfg.Runtime.Tun.BypassLocal = true
	cfg.Proxy.SubscriptionURL = "https://example.com/sub"

	outPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := RenderTemplate(cfg, "en0", "192.168.12.100", outPath); err != nil {
		t.Fatalf("RenderTemplate() error = %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	rendered := string(data)

	if !strings.Contains(rendered, "include-interface:\n    - en0") {
		t.Fatalf("rendered config missing include-interface block:\n%s", rendered)
	}
}

func TestRenderTemplateOmitsInterfaceScopedTunWhenBypassLocalDisabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Tun.Enabled = true
	cfg.Runtime.Tun.BypassLocal = false
	cfg.Proxy.SubscriptionURL = "https://example.com/sub"

	outPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := RenderTemplate(cfg, "en0", "192.168.12.100", outPath); err != nil {
		t.Fatalf("RenderTemplate() error = %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	rendered := string(data)

	if strings.Contains(rendered, "include-interface:") {
		t.Fatalf("rendered config should not contain include-interface block:\n%s", rendered)
	}
}

func TestRenderTemplateDegradedModeOmitsProxyProviders(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Proxy.Source = "url"
	cfg.Proxy.SubscriptionURL = ""

	outPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := RenderTemplateWithOptions(cfg, "en0", "192.168.12.100", outPath, RenderOptions{ProxyProviderAvailable: false}); err != nil {
		t.Fatalf("RenderTemplateWithOptions() error = %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	rendered := string(data)

	if strings.Contains(rendered, "proxy-providers:") {
		t.Fatalf("degraded config should not contain proxy-providers block:\n%s", rendered)
	}
	if !strings.Contains(rendered, "- name: Proxy") || !strings.Contains(rendered, "- name: Auto") || !strings.Contains(rendered, "- name: Fallback") {
		t.Fatalf("degraded config should keep proxy group names:\n%s", rendered)
	}
	if strings.Contains(rendered, "use:") {
		t.Fatalf("degraded config should not reference provider use blocks:\n%s", rendered)
	}
}

func TestRenderTemplateNormalModeIncludesProxyProviders(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Proxy.Source = "url"
	cfg.Proxy.SubscriptionURL = "https://example.com/sub"
	cfg.Proxy.SubscriptionName = "demo"

	outPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := RenderTemplateWithOptions(cfg, "en0", "192.168.12.100", outPath, RenderOptions{ProxyProviderAvailable: true}); err != nil {
		t.Fatalf("RenderTemplateWithOptions() error = %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	rendered := string(data)

	if !strings.Contains(rendered, "proxy-providers:") {
		t.Fatalf("normal config should contain proxy-providers block:\n%s", rendered)
	}
	if !strings.Contains(rendered, "url: \"https://example.com/sub\"") {
		t.Fatalf("normal config should contain subscription URL:\n%s", rendered)
	}
	if !strings.Contains(rendered, "use:\n      - demo") {
		t.Fatalf("normal config should reference provider group:\n%s", rendered)
	}
}
