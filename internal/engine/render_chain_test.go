package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/tght/lan-proxy-gateway/internal/config"
)

func TestRender_SubscriptionWithResidentialChainScript(t *testing.T) {
	workDir := t.TempDir()
	subscription := `#!MANAGED-CONFIG https://example.com/sub?clash=1

proxies:
  - name: "香港 01"
    type: socks5
    server: hk.example.com
    port: 443
proxy-groups:
  - name: "🔰国外流量"
    type: select
    proxies:
      - "香港 01"
rules:
  - MATCH,🔰国外流量
`
	filePath := filepath.Join(workDir, "subscription.yaml")
	if err := os.WriteFile(filePath, []byte(subscription), 0o600); err != nil {
		t.Fatalf("write subscription: %v", err)
	}

	cfg := configpkg.Default()
	cfg.Source.Type = configpkg.SourceTypeFile
	cfg.Source.File.Path = filePath
	cfg.Source.ChainResidential = &configpkg.ChainResidentialConfig{
		Name:        "🏠 住宅IP",
		Kind:        "socks5",
		Server:      "206.40.215.135",
		Port:        443,
		Username:    "u",
		Password:    "p",
		DialerProxy: "🛫 AI起飞节点",
	}

	out, err := Render(context.Background(), cfg, workDir)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"AI起飞节点",
		"AI落地节点",
		"dialer-proxy:",
		"anthropic.com",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("rendered config missing %q", want)
		}
	}
}
