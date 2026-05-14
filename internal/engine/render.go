// Package engine is the only code path that speaks directly to mihomo.
// It renders the final config.yaml, manages the mihomo child process,
// and exposes a thin REST client for status queries.
package engine

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/tght/lan-proxy-gateway/embed"
	configpkg "github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/script"
	"github.com/tght/lan-proxy-gateway/internal/script/presets"
	"github.com/tght/lan-proxy-gateway/internal/source"
	"github.com/tght/lan-proxy-gateway/internal/traffic"
)

type renderOptions struct {
	subscriptionProxyURL string
}

// Render builds the final mihomo YAML for the given config.
// Side effects: materialize 可能下订阅 / 读用户 yaml，也会把内嵌 Web UI
// 释放到 workdir/ui/，这样 Start 和 Reload 两条路径都能自动部署 UI。
func Render(ctx context.Context, cfg *configpkg.Config, workDir string) ([]byte, error) {
	cfg = configpkg.EffectiveRuntimeConfig(cfg)
	return renderWithOptions(ctx, cfg, workDir, renderOptions{})
}

func renderWithOptions(ctx context.Context, cfg *configpkg.Config, workDir string, opts renderOptions) ([]byte, error) {
	cfg = configpkg.EffectiveRuntimeConfig(cfg)
	// 先把 Web UI 释放好，mihomo external-ui 生效后浏览器访问 /ui 立即可用。
	// 失败只打 warning，别因为 UI 问题阻塞核心 render。
	if err := deployWebUI(workDir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: 部署 Web 控制台失败: %v\n", err)
	}

	frag, err := source.MaterializeWithOptions(ctx, cfg.Source, workDir, source.MaterializeOptions{
		SubscriptionProxyURL: opts.subscriptionProxyURL,
		AutoGroups:           cfg.Traffic.AutoGroups,
	})
	if err != nil {
		return nil, fmt.Errorf("materialize source: %w", err)
	}

	rules := traffic.Render(cfg.Traffic)
	// 用户源（订阅/本地文件）带了自己的 rules：把 base rules 末尾的
	// MATCH,Proxy 兜底去掉，换成用户 rules 做兜底（用户 yaml 里一般自己
	// 就有 MATCH）。这样用户订阅里的 GEOSITE/GEOIP/DOMAIN 规则链能生效，
	// 不会被我们的 MATCH,Proxy 截胡。
	if len(frag.Rules) > 0 {
		matchLine := "  - MATCH," + traffic.ProxyTag + "\n"
		rules = strings.TrimSuffix(rules, matchLine)
		var b strings.Builder
		b.WriteString(rules)
		for _, r := range frag.Rules {
			b.WriteString("  - ")
			b.WriteString(r)
			b.WriteString("\n")
		}
		rules = b.String()
	}

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

	// 增强脚本：先看是否有「链式代理预设」要实例化，再退化到用户自定义 ScriptPath。
	effectiveScript := cfg.Source.ScriptPath
	if cfg.Source.ChainResidential != nil {
		rendered, err := presets.RenderResidentialChain(cfg.Source.ChainResidential, workDir)
		if err != nil {
			return nil, fmt.Errorf("渲染链式代理预设失败: %w", err)
		}
		effectiveScript = rendered
	}
	if effectiveScript != "" {
		modified, err := script.Apply(effectiveScript, []byte(out))
		if err != nil {
			return nil, fmt.Errorf("执行增强脚本失败: %w", err)
		}
		return modified, nil
	}
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
