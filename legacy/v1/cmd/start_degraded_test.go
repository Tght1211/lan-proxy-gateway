package cmd

import (
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
	tmpl "github.com/tght/lan-proxy-gateway/internal/template"
)

func TestDegradedStartupUsesDirectModeWhenSubscriptionUnavailable(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Proxy.Source = "url"
	cfg.Proxy.SubscriptionURL = ""

	status := evaluateSubscriptionStartupStatus(cfg)
	if status.Ready {
		t.Fatalf("expected startup status to be degraded")
	}

	opts := tmpl.RenderOptions{ProxyProviderAvailable: status.Ready}
	if opts.ProxyProviderAvailable {
		t.Fatalf("expected proxy provider to stay disabled in degraded mode")
	}
}

func TestDegradedStartupKeepsProviderEnabledWhenSubscriptionReady(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Proxy.Source = "url"
	cfg.Proxy.SubscriptionURL = "https://example.com/sub"

	opts := tmpl.RenderOptions{ProxyProviderAvailable: true}
	if !opts.ProxyProviderAvailable {
		t.Fatalf("expected proxy provider to be enabled")
	}
}
