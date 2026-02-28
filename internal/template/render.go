package template

import (
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

	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}

	// For file mode: patch proxy-providers from http to file type
	if cfg.ProxySource == "file" {
		result = patchForFileMode(result)
	}

	return os.WriteFile(outputPath, []byte(result), 0644)
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
