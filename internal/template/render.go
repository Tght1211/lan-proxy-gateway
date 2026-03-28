package template

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	embed "github.com/tght/lan-proxy-gateway/embed"
	"github.com/tght/lan-proxy-gateway/internal/config"
)

// RenderTemplate replaces {{VARIABLE}} placeholders with actual values
// and writes the result to outputPath.
func RenderTemplate(cfg *config.Config, iface, ip, outputPath string) error {
	result := embed.TemplateContent

	replacements := map[string]string{
		"{{MIXED_PORT}}":        strconv.Itoa(cfg.Ports.Mixed),
		"{{REDIR_PORT}}":        strconv.Itoa(cfg.Ports.Redir),
		"{{API_PORT}}":          strconv.Itoa(cfg.Ports.API),
		"{{API_SECRET}}":        cfg.APISecret,
		"{{DNS_LISTEN_PORT}}":   strconv.Itoa(cfg.Ports.DNS),
		"{{SUBSCRIPTION_URL}}":  cfg.SubscriptionURL,
		"{{SUBSCRIPTION_NAME}}": cfg.SubscriptionName,
		"{{LAN_INTERFACE}}":     iface,
		"{{LAN_IP}}":            ip,
	}

	// 处理链式代理配置
	proxySection, groupSection, rulesSection := buildChainProxyConfig(cfg)
	replacements["{{CHAIN_PROXY_SECTION}}"] = proxySection
	replacements["{{CHAIN_PROXY_GROUP}}"] = groupSection
	replacements["{{CHAIN_PROXY_RULES}}"] = rulesSection

	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}

	// For file mode: patch proxy-providers from http to file type
	if cfg.ProxySource == "file" {
		result = patchForFileMode(result)
	}

	return os.WriteFile(outputPath, []byte(result), 0644)
}

// buildChainProxyConfig 构建链式代理的配置片段
func buildChainProxyConfig(cfg *config.Config) (proxySection, groupSection, rulesSection string) {
	if cfg.ChainProxy == nil || !cfg.ChainProxy.Enabled {
		proxySection = "proxies: []"
		groupSection = ""
		rulesSection = ""
		return
	}

	cp := cfg.ChainProxy
	proxySection = fmt.Sprintf(`proxies:
  - name: %s
    type: %s
    server: %s
    port: %d`, cp.Name, cp.Type, cp.Server, cp.Port)
	if cp.Username != "" {
		proxySection += fmt.Sprintf("\n    username: %s", cp.Username)
	}
	if cp.Password != "" {
		proxySection += fmt.Sprintf("\n    password: %s", cp.Password)
	}
	proxySection += fmt.Sprintf("\n    udp: %t", cp.UDP)
	proxySection += "\n    dialer-proxy: Proxy"

	groupSection = fmt.Sprintf(`  - name: AI + Foreign
    type: select
    proxies:
      - %s
      - Proxy
      - DIRECT

`, cp.Name)

	rulesSection = `  # --- AI + Foreign（链式代理 → 住宅 IP）---
  - DOMAIN-SUFFIX,anthropic.com,AI + Foreign
  - DOMAIN-SUFFIX,claude.ai,AI + Foreign
  - DOMAIN-SUFFIX,claudeusercontent.com,AI + Foreign
  - DOMAIN-SUFFIX,openai.com,AI + Foreign
  - DOMAIN-SUFFIX,chatgpt.com,AI + Foreign
  - DOMAIN-SUFFIX,oaiusercontent.com,AI + Foreign
  - DOMAIN-SUFFIX,oaistatic.com,AI + Foreign
  - DOMAIN,gemini.google.com,AI + Foreign
  - DOMAIN,generativelanguage.googleapis.com,AI + Foreign
  - DOMAIN,ping0.cc,AI + Foreign
  - DOMAIN,ipinfo.io,AI + Foreign
`
	return
}

// patchForFileMode modifies the generated config to use local file
// instead of HTTP subscription.
func patchForFileMode(content string) string {
	lines := strings.Split(content, "\n")
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Change type: http to type: file
		if trimmed == "type: http" {
			line = strings.Replace(line, "type: http", "type: file", 1)
		}
		// Remove url: line within proxy-providers
		if strings.HasPrefix(trimmed, "url: \"") {
			continue
		}
		// Remove interval: 3600 line
		if trimmed == "interval: 3600" {
			continue
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}
