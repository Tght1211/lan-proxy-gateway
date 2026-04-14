package template

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	embed "github.com/tght/lan-proxy-gateway/embed"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/rules"
	"github.com/tght/lan-proxy-gateway/internal/script"
)

// RenderTemplate replaces {{VARIABLE}} placeholders with actual values
// and writes the result to outputPath.
func RenderTemplate(cfg *config.Config, iface, ip, outputPath string) error {
	result := embed.TemplateContent

	tunConfig := "tun:\n  enable: false"
	if cfg.Runtime.Tun.Enabled {
		tunConfig = "tun:\n  enable: true\n  stack: mixed\n  auto-route: true\n  auto-detect-interface: true\n  mtu: 1500"
		if cfg.Runtime.Tun.BypassLocal && iface != "" {
			tunConfig += "\n  include-interface:\n    - " + iface
		}
	}

	replacements := map[string]string{
		"{{MIXED_PORT}}":      strconv.Itoa(cfg.Runtime.Ports.Mixed),
		"{{REDIR_PORT}}":      strconv.Itoa(cfg.Runtime.Ports.Redir),
		"{{API_PORT}}":        strconv.Itoa(cfg.Runtime.Ports.API),
		"{{API_SECRET}}":      cfg.Runtime.APISecret,
		"{{DNS_LISTEN_PORT}}": strconv.Itoa(cfg.Runtime.Ports.DNS),
		"{{LAN_INTERFACE}}":   iface,
		"{{LAN_IP}}":          ip,
		"{{TUN_CONFIG}}":      tunConfig,
		"{{PROXY_BLOCK}}":     renderProxyBlock(cfg),
		"{{RULES_BLOCK}}":     rules.Render(cfg),
	}

	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}

	output := []byte(result)

	switch cfg.Extension.Mode {
	case "chains":
		if cfg.Extension.ResidentialChain == nil {
			return fmt.Errorf("extension.mode 为 chains 但未配置 extension.residential_chain")
		}
		// chains 模式不支持 proxy 直接代理来源
		if cfg.Proxy.Source == "proxy" {
			return fmt.Errorf("chains 链式代理模式需要订阅链接或本地配置文件作为出口节点来源，不支持直接代理模式")
		}
		modified, err := script.ApplyChains(cfg.Extension.ResidentialChain, output)
		if err != nil {
			return fmt.Errorf("链式代理注入失败: %w", err)
		}
		output = modified
	case "script":
		if cfg.Extension.ScriptPath == "" {
			return fmt.Errorf("extension.mode 为 script 但未配置 extension.script_path")
		}
		modified, err := script.Apply(cfg.Extension.ScriptPath, output)
		if err != nil {
			return fmt.Errorf("扩展脚本执行失败: %w", err)
		}
		output = modified
	}

	return os.WriteFile(outputPath, output, 0644)
}

// renderProxyBlock 根据代理来源类型生成对应的 proxy-providers/proxies + proxy-groups 配置块
func renderProxyBlock(cfg *config.Config) string {
	switch cfg.Proxy.Source {
	case "proxy":
		return renderDirectProxyBlock(cfg.Proxy.DirectProxy)
	case "file":
		return renderSubscriptionProxyBlock(cfg.Proxy.SubscriptionName, cfg.Proxy.SubscriptionURL, true)
	default:
		return renderSubscriptionProxyBlock(cfg.Proxy.SubscriptionName, cfg.Proxy.SubscriptionURL, false)
	}
}

// renderSubscriptionProxyBlock 生成订阅/文件模式的代理配置块
func renderSubscriptionProxyBlock(name, url string, fileMode bool) string {
	var sb strings.Builder

	if fileMode {
		sb.WriteString("proxy-providers:\n")
		sb.WriteString("  " + name + ":\n")
		sb.WriteString("    type: file\n")
		sb.WriteString("    path: ./proxy_provider/" + name + ".yaml\n")
		sb.WriteString("    health-check:\n")
		sb.WriteString("      enable: true\n")
		sb.WriteString("      interval: 120\n")
		sb.WriteString("      url: http://www.gstatic.com/generate_204\n")
	} else {
		sb.WriteString("proxy-providers:\n")
		sb.WriteString("  " + name + ":\n")
		sb.WriteString("    type: http\n")
		sb.WriteString("    url: \"" + url + "\"\n")
		sb.WriteString("    interval: 1800\n")
		sb.WriteString("    path: ./proxy_provider/" + name + ".yaml\n")
		sb.WriteString("    health-check:\n")
		sb.WriteString("      enable: true\n")
		sb.WriteString("      interval: 120\n")
		sb.WriteString("      url: http://www.gstatic.com/generate_204\n")
	}

	sb.WriteString("\nproxy-groups:\n")
	sb.WriteString("  - name: Proxy\n")
	sb.WriteString("    type: select\n")
	sb.WriteString("    use:\n")
	sb.WriteString("      - " + name + "\n")
	sb.WriteString("    proxies:\n")
	sb.WriteString("      - Auto\n")
	sb.WriteString("      - Fallback\n")
	sb.WriteString("      - DIRECT\n")
	sb.WriteString("\n  - name: Auto\n")
	sb.WriteString("    type: url-test\n")
	sb.WriteString("    use:\n")
	sb.WriteString("      - " + name + "\n")
	sb.WriteString("    url: http://www.gstatic.com/generate_204\n")
	sb.WriteString("    interval: 120\n")
	sb.WriteString("    tolerance: 100\n")
	sb.WriteString("\n  - name: Fallback\n")
	sb.WriteString("    type: fallback\n")
	sb.WriteString("    use:\n")
	sb.WriteString("      - " + name + "\n")
	sb.WriteString("    url: http://www.gstatic.com/generate_204\n")
	sb.WriteString("    interval: 120\n")

	return sb.String()
}

// renderDirectProxyBlock 生成直接代理服务器模式的代理配置块
func renderDirectProxyBlock(dp *config.DirectProxyConfig) string {
	if dp == nil || strings.TrimSpace(dp.Server) == "" || dp.Port <= 0 {
		// 配置不完整时生成一个占位配置，避免 mihomo 启动失败
		return "proxies: []\n\nproxy-groups:\n  - name: Proxy\n    type: select\n    proxies:\n      - DIRECT\n\n  - name: Auto\n    type: select\n    proxies:\n      - DIRECT\n\n  - name: Fallback\n    type: select\n    proxies:\n      - DIRECT\n"
	}

	name := strings.TrimSpace(dp.Name)
	if name == "" {
		name = "MyProxy"
	}
	proxyType := strings.ToLower(strings.TrimSpace(dp.Type))
	if proxyType != "http" {
		proxyType = "socks5"
	}

	var sb strings.Builder
	sb.WriteString("proxies:\n")
	sb.WriteString(fmt.Sprintf("  - name: \"%s\"\n", name))
	sb.WriteString(fmt.Sprintf("    type: %s\n", proxyType))
	sb.WriteString(fmt.Sprintf("    server: %s\n", dp.Server))
	sb.WriteString(fmt.Sprintf("    port: %d\n", dp.Port))
	if strings.TrimSpace(dp.Username) != "" {
		sb.WriteString(fmt.Sprintf("    username: \"%s\"\n", dp.Username))
		sb.WriteString(fmt.Sprintf("    password: \"%s\"\n", dp.Password))
	}

	sb.WriteString("\nproxy-groups:\n")
	sb.WriteString("  - name: Proxy\n")
	sb.WriteString("    type: select\n")
	sb.WriteString("    proxies:\n")
	sb.WriteString(fmt.Sprintf("      - \"%s\"\n", name))
	sb.WriteString("      - DIRECT\n")
	sb.WriteString("\n  - name: Auto\n")
	sb.WriteString("    type: select\n")
	sb.WriteString("    proxies:\n")
	sb.WriteString(fmt.Sprintf("      - \"%s\"\n", name))
	sb.WriteString("      - DIRECT\n")
	sb.WriteString("\n  - name: Fallback\n")
	sb.WriteString("    type: select\n")
	sb.WriteString("    proxies:\n")
	sb.WriteString(fmt.Sprintf("      - \"%s\"\n", name))
	sb.WriteString("      - DIRECT\n")

	return sb.String()
}
