package template

import (
	"os"
	"sort"
	"strconv"
	"strings"

	embed "github.com/tght/lan-proxy-gateway/embed"
	"github.com/tght/lan-proxy-gateway/internal/config"
)

// RenderTemplate replaces {{VARIABLE}} placeholders with actual values
// and writes the result to outputPath.
func RenderTemplate(cfg *config.Config, iface, ip, outputPath string) error {
	result := embed.TemplateContent

	regionFilter := ""
	if cfg.Regions.Enabled {
		regionFilter = buildRegionFilter(cfg)
	}

	replacements := map[string]string{
		"{{MIXED_PORT}}":          strconv.Itoa(cfg.Ports.Mixed),
		"{{REDIR_PORT}}":          strconv.Itoa(cfg.Ports.Redir),
		"{{API_PORT}}":            strconv.Itoa(cfg.Ports.API),
		"{{API_SECRET}}":          cfg.APISecret,
		"{{DNS_LISTEN_PORT}}":     strconv.Itoa(cfg.Ports.DNS),
		"{{SUBSCRIPTION_URL}}":    cfg.SubscriptionURL,
		"{{SUBSCRIPTION_NAME}}":   cfg.SubscriptionName,
		"{{LAN_INTERFACE}}":       iface,
		"{{LAN_IP}}":              ip,
		"{{REGION_FILTER_BLOCK}}": regionFilter,
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

func buildRegionFilter(cfg *config.Config) string {
	keywords := make([]string, 0)
	seen := make(map[string]struct{})
	for _, code := range cfg.Regions.Include {
		for _, keyword := range cfg.Regions.Mapping[code] {
			keyword = strings.TrimSpace(keyword)
			if keyword == "" {
				continue
			}
			if _, ok := seen[keyword]; ok {
				continue
			}
			seen[keyword] = struct{}{}
			keywords = append(keywords, keyword)
		}
	}
	if len(keywords) == 0 {
		return ""
	}
	sort.Strings(keywords)
	escaped := make([]string, 0, len(keywords))
	for _, keyword := range keywords {
		escaped = append(escaped, regexpQuote(keyword))
	}
	return "    filter: '(?i)(" + strings.Join(escaped, "|") + ")'\n"
}

func regexpQuote(s string) string {
	replacer := strings.NewReplacer(
		`\\`, `\\\\`,
		`.`, `\\.`,
		`+`, `\\+`,
		`*`, `\\*`,
		`?`, `\\?`,
		`(`, `\\(`,
		`)`, `\\)`,
		`[`, `\\[`,
		`]`, `\\]`,
		`{`, `\\{`,
		`}`, `\\}`,
		`^`, `\\^`,
		`$`, `\\$`,
		`|`, `\\|`,
	)
	return replacer.Replace(s)
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
