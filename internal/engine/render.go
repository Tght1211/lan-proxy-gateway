// Package engine is the only code path that speaks directly to mihomo.
// It renders the final config.yaml, manages the mihomo child process,
// and exposes a thin REST client for status queries.
package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/tght/lan-proxy-gateway/embed"
	configpkg "github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/source"
	"github.com/tght/lan-proxy-gateway/internal/traffic"
)

// Render builds the final mihomo YAML for the given config. Pure function
// (aside from the network call the source layer may make).
func Render(ctx context.Context, cfg *configpkg.Config, workDir string) ([]byte, error) {
	frag, err := source.Materialize(ctx, cfg.Source, workDir)
	if err != nil {
		return nil, fmt.Errorf("materialize source: %w", err)
	}

	rules := traffic.Render(cfg.Traffic)

	out := embed.Template
	out = strings.ReplaceAll(out, "{{MIXED_PORT}}", strconv.Itoa(cfg.Runtime.Ports.Mixed))
	out = strings.ReplaceAll(out, "{{REDIR_PORT}}", strconv.Itoa(cfg.Runtime.Ports.Redir))
	out = strings.ReplaceAll(out, "{{API_PORT}}", strconv.Itoa(cfg.Runtime.Ports.API))
	out = strings.ReplaceAll(out, "{{MIHOMO_MODE}}", cfg.Traffic.Mode)
	out = strings.ReplaceAll(out, "{{LOG_LEVEL}}", cfg.Runtime.LogLevel)
	out = strings.ReplaceAll(out, "{{TUN_CONFIG}}", renderTUNBlock(cfg))
	out = strings.ReplaceAll(out, "{{DNS_ENABLED}}", boolStr(cfg.Gateway.DNS.Enabled))
	out = strings.ReplaceAll(out, "{{DNS_PORT}}", strconv.Itoa(cfg.Gateway.DNS.Port))
	out = strings.ReplaceAll(out, "{{PROXY_BLOCK}}", frag.YAML)
	out = strings.ReplaceAll(out, "{{RULES_BLOCK}}", rules)
	return []byte(out), nil
}

func renderTUNBlock(cfg *configpkg.Config) string {
	if !cfg.Gateway.TUN.Enabled {
		return "tun:\n  enable: false\n"
	}
	var b strings.Builder
	b.WriteString("tun:\n")
	b.WriteString("  enable: true\n")
	b.WriteString("  stack: system\n")
	b.WriteString("  dns-hijack:\n    - any:53\n")
	b.WriteString("  auto-route: true\n")
	b.WriteString("  auto-detect-interface: true\n")
	b.WriteString("  mtu: 1500\n")
	if cfg.Gateway.TUN.BypassLocal {
		b.WriteString("  strict-route: false\n")
	} else {
		b.WriteString("  strict-route: true\n")
	}
	return b.String()
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
