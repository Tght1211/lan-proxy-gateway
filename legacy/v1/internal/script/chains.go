package script

import (
	"fmt"
	"strings"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/rules"
	"gopkg.in/yaml.v3"
)

// ApplyChains injects residential chain proxy nodes, proxy groups, and routing
// rules into the given Clash/mihomo YAML config. This is the Go-native equivalent
// of the script.js example bundled with the project.
//
// Chain topology:  device → mihomo → airport node → residential proxy → target
// Result: traffic exits via a clean residential IP instead of a shared datacenter IP.
//
// mode="rule"   — only AI services route via residential proxy
// mode="global" — all traffic routes via residential proxy (MATCH rule replaced)
func ApplyChains(chain *config.ResidentialChain, yamlContent []byte) ([]byte, error) {
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(yamlContent, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	airportGroup := chain.AirportGroup
	if airportGroup == "" {
		airportGroup = "Auto"
	}

	// 1. Add Residential-Proxy node (dialer-proxy chains it through the airport)
	residentialProxy := map[string]interface{}{
		"name":             "Residential-Proxy",
		"type":             chain.ProxyType,
		"server":           chain.ProxyServer,
		"port":             chain.ProxyPort,
		"dialer-proxy":     airportGroup,
		"udp":              false,
		"skip-cert-verify": true,
	}
	if chain.ProxyUsername != "" {
		residentialProxy["username"] = chain.ProxyUsername
		residentialProxy["password"] = chain.ProxyPassword
	}

	proxies, _ := cfg["proxies"].([]interface{})
	cfg["proxies"] = append(proxies, residentialProxy)

	// 2. Add "AI Only" proxy group (select between residential / direct / airport)
	aiGroup := map[string]interface{}{
		"name":    "AI Only",
		"type":    "select",
		"proxies": []interface{}{"Residential-Proxy", "DIRECT", airportGroup},
	}
	proxyGroups, _ := cfg["proxy-groups"].([]interface{})
	cfg["proxy-groups"] = append([]interface{}{aiGroup}, proxyGroups...)

	// 3. Build priority rules
	var priorityRules []interface{}

	// Keep status probe domains pinned to the ordinary proxy path so
	// "普通出口" reflects the non-residential chain in rule mode.
	priorityRules = append(priorityRules, stringSliceToInterfaces(rules.NormalProbeRules("Proxy"))...)

	// User-defined direct rules (e.g. corporate intranet, office apps)
	for _, rule := range chain.ExtraDirectRules {
		priorityRules = append(priorityRules, rule)
	}

	// User-defined proxy rules (routed via residential proxy, same as AI Only group)
	for _, rule := range chain.ExtraProxyRules {
		priorityRules = append(priorityRules, rule)
	}

	// AI services: route via residential proxy (mirrors script.js lines 67-84)
	priorityRules = append(priorityRules, stringSliceToInterfaces(rules.AIProxyRules("AI Only"))...)

	// 4. Add AI rule-set providers and their RULE-SET entries (mirrors script.js lines 89-104)
	if cfg["rule-providers"] == nil {
		cfg["rule-providers"] = map[string]interface{}{}
	}
	ruleProviders, _ := cfg["rule-providers"].(map[string]interface{})

	for _, p := range rules.AIProviders() {
		ruleProviders[p.Name] = map[string]interface{}{
			"type":     "http",
			"behavior": "domain",
			"url":      p.URL,
			"path":     fmt.Sprintf("./ruleset/%s.yaml", p.Name),
			"interval": 86400,
		}
		priorityRules = append(priorityRules, fmt.Sprintf("RULE-SET,%s,AI Only", p.Name))
	}

	// 5. Merge rules: priority first, then existing
	existingRules, _ := cfg["rules"].([]interface{})

	if chain.Mode == "global" {
		// Global mode: replace the catch-all MATCH rule to route everything via residential
		var filtered []interface{}
		for _, r := range existingRules {
			if s, ok := r.(string); ok && strings.HasPrefix(s, "MATCH,") {
				continue
			}
			filtered = append(filtered, r)
		}
		existingRules = append(filtered, "MATCH,AI Only")
	}

	cfg["rules"] = append(priorityRules, existingRules...)

	return yaml.Marshal(cfg)
}

func stringSliceToInterfaces(items []string) []interface{} {
	out := make([]interface{}, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}
