package config

import (
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Proxy     ProxyConfig     `yaml:"proxy"`
	Runtime   RuntimeConfig   `yaml:"runtime"`
	Rules     RulesConfig     `yaml:"rules,omitempty"`
	Extension ExtensionConfig `yaml:"extension"`
}

type ProxyConfig struct {
	// Source: url = 使用订阅链接 | file = 使用本地 Clash/mihomo 配置文件 | proxy = 直接配置代理服务器
	Source           string              `yaml:"source"`
	SubscriptionURL  string              `yaml:"subscription_url,omitempty"`
	ConfigFile       string              `yaml:"config_file,omitempty"`
	SubscriptionName string              `yaml:"subscription_name"`
	CurrentProfile   string              `yaml:"current_profile,omitempty"`
	Profiles         []ProxyProfile      `yaml:"profiles,omitempty"`
	DirectProxy      *DirectProxyConfig  `yaml:"direct_proxy,omitempty"`
}

// DirectProxyConfig 直接配置代理服务器模式（无需订阅链接或本地配置文件）
// 通过 proxy.source: proxy 启用，适合手头有 SOCKS5/HTTP 代理但没有机场订阅的场景
type DirectProxyConfig struct {
	Name     string `yaml:"name,omitempty"`    // 节点显示名称，默认 "MyProxy"
	Type     string `yaml:"type"`              // 协议类型：socks5 / http
	Server   string `yaml:"server"`            // 代理服务器地址
	Port     int    `yaml:"port"`              // 代理服务器端口
	Username string `yaml:"username,omitempty"` // 认证用户名（可选）
	Password string `yaml:"password,omitempty"` // 认证密码（可选）
}

type ProxyProfile struct {
	Name            string `yaml:"name"`
	Source          string `yaml:"source"`
	SubscriptionURL string `yaml:"subscription_url,omitempty"`
	ConfigFile      string `yaml:"config_file,omitempty"`
}

type ExtensionConfig struct {
	// Mode 控制启用哪种扩展：chains（内置链式代理）/ script（JS 脚本）/ 留空（不启用）
	Mode             string            `yaml:"mode"`
	ScriptPath       string            `yaml:"script_path,omitempty"`
	ResidentialChain *ResidentialChain `yaml:"residential_chain,omitempty"`
}

type RuntimeConfig struct {
	Ports     PortsConfig `yaml:"ports"`
	APISecret string      `yaml:"api_secret,omitempty"`
	Tun       TunConfig   `yaml:"tun"`
}

type TunConfig struct {
	Enabled     bool `yaml:"enabled,omitempty"`
	BypassLocal bool `yaml:"bypass_local,omitempty"`
}

type RulesConfig struct {
	LanDirect        *bool    `yaml:"lan_direct,omitempty"`
	ChinaDirect      *bool    `yaml:"china_direct,omitempty"`
	AppleRules       *bool    `yaml:"apple_rules,omitempty"`
	NintendoProxy    *bool    `yaml:"nintendo_proxy,omitempty"`
	GlobalProxy      *bool    `yaml:"global_proxy,omitempty"`
	AdsReject        *bool    `yaml:"ads_reject,omitempty"`
	ExtraDirectRules []string `yaml:"extra_direct_rules,omitempty"`
	ExtraProxyRules  []string `yaml:"extra_proxy_rules,omitempty"`
	ExtraRejectRules []string `yaml:"extra_reject_rules,omitempty"`
}

// ResidentialChain 链式代理配置：通过机场节点出口连接住宅代理，获得纯净住宅 IP
// 通过 extension.mode: chains 启用，无需单独 enabled 字段
type ResidentialChain struct {
	LegacyEnabled    bool     `yaml:"enabled,omitempty"` // 兼容旧配置，保存时会被清空
	Mode             string   `yaml:"mode"`              // rule（默认）/ global
	ProxyServer      string   `yaml:"proxy_server"`
	ProxyPort        int      `yaml:"proxy_port"`
	ProxyUsername    string   `yaml:"proxy_username,omitempty"`
	ProxyPassword    string   `yaml:"proxy_password,omitempty"`
	ProxyType        string   `yaml:"proxy_type"`                   // socks5 / http
	AirportGroup     string   `yaml:"airport_group"`                // 机场中的延迟测速代理组，默认 Auto
	ExtraDirectRules []string `yaml:"extra_direct_rules,omitempty"` // 追加的强制直连规则
	ExtraProxyRules  []string `yaml:"extra_proxy_rules,omitempty"`  // 追加的走住宅代理规则
}

type PortsConfig struct {
	Mixed int `yaml:"mixed"`
	Redir int `yaml:"redir"`
	API   int `yaml:"api"`
	DNS   int `yaml:"dns"`
}

func DefaultConfig() *Config {
	return &Config{
		Proxy: ProxyConfig{
			Source:           "url",
			SubscriptionName: "subscription",
			CurrentProfile:   "subscription",
			Profiles: []ProxyProfile{
				{
					Name:   "subscription",
					Source: "url",
				},
			},
		},
		Runtime: RuntimeConfig{
			Ports: PortsConfig{
				Mixed: 7890,
				Redir: 7892,
				API:   9090,
				DNS:   53,
			},
			Tun: TunConfig{
				Enabled: true,
			},
		},
		Extension: ExtensionConfig{},
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw diskConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	hasExplicitExtensionMode := hasKeyPath(&root, "extension", "mode") || hasKeyPath(&root, "extension_mode")
	return raw.toConfig(hasExplicitExtensionMode), nil
}

func Save(cfg *Config, path string) error {
	data, err := yaml.Marshal(newDiskConfig(cfg))
	if err != nil {
		return err
	}

	header := []byte("# lan-proxy-gateway 配置文件\n# 此文件包含敏感信息，请勿提交到 Git\n\n")
	return os.WriteFile(path, append(header, data...), 0600)
}

type diskConfig struct {
	Proxy     *ProxyConfig     `yaml:"proxy,omitempty"`
	Runtime   *RuntimeConfig   `yaml:"runtime,omitempty"`
	Rules     *RulesConfig     `yaml:"rules,omitempty"`
	Extension *ExtensionConfig `yaml:"extension,omitempty"`

	LegacyProxySource      string `yaml:"proxy_source,omitempty"`
	LegacySubscriptionURL  string `yaml:"subscription_url,omitempty"`
	LegacyProxyConfigFile  string `yaml:"proxy_config_file,omitempty"`
	LegacySubscriptionName string `yaml:"subscription_name,omitempty"`

	LegacyPorts      PortsConfig `yaml:"ports,omitempty"`
	LegacyAPISecret  string      `yaml:"api_secret,omitempty"`
	LegacyTunEnabled bool        `yaml:"tun_enabled,omitempty"`

	LegacyExtensionMode    string             `yaml:"extension_mode,omitempty"`
	LegacyScriptPath       string             `yaml:"script_path,omitempty"`
	LegacyResidentialChain *ResidentialChain  `yaml:"residential_chain,omitempty"`
}

func newDiskConfig(cfg *Config) *diskConfig {
	return &diskConfig{
		Proxy:     sanitizeProxy(cfg.Proxy),
		Runtime:   sanitizeRuntime(cfg.Runtime),
		Rules:     sanitizeRules(cfg.Rules),
		Extension: sanitizeExtension(cfg.Extension),
	}
}

func (raw *diskConfig) toConfig(hasExplicitExtensionMode bool) *Config {
	cfg := DefaultConfig()

	if raw.Proxy != nil {
		cfg.Proxy = *sanitizeProxy(*raw.Proxy)
	} else {
		cfg.Proxy = *sanitizeProxy(ProxyConfig{
			Source:           raw.LegacyProxySource,
			SubscriptionURL:  raw.LegacySubscriptionURL,
			ConfigFile:       raw.LegacyProxyConfigFile,
			SubscriptionName: raw.LegacySubscriptionName,
		})
	}

	if raw.Runtime != nil {
		cfg.Runtime = *sanitizeRuntime(*raw.Runtime)
	} else {
		cfg.Runtime = *sanitizeRuntime(RuntimeConfig{
			Ports:     raw.LegacyPorts,
			APISecret: raw.LegacyAPISecret,
			Tun: TunConfig{
				Enabled: raw.LegacyTunEnabled,
			},
		})
	}

	if raw.Rules != nil {
		cfg.Rules = *sanitizeRules(*raw.Rules)
	}

	switch {
	case raw.Extension != nil:
		cfg.Extension = *sanitizeExtension(*raw.Extension)
	case raw.LegacyExtensionMode != "" || raw.LegacyScriptPath != "" || raw.LegacyResidentialChain != nil:
		cfg.Extension = ExtensionConfig{
			Mode:             raw.LegacyExtensionMode,
			ScriptPath:       raw.LegacyScriptPath,
			ResidentialChain: cloneResidentialChain(raw.LegacyResidentialChain),
		}
	default:
		cfg.Extension = ExtensionConfig{}
	}

	if !hasExplicitExtensionMode && cfg.Extension.Mode == "" {
		switch {
		case cfg.Extension.ResidentialChain != nil && cfg.Extension.ResidentialChain.LegacyEnabled:
			cfg.Extension.Mode = "chains"
		case cfg.Extension.ScriptPath != "":
			cfg.Extension.Mode = "script"
		}
	}

	cfg.Extension = *sanitizeExtension(cfg.Extension)
	return cfg
}

func sanitizeProxy(proxy ProxyConfig) *ProxyConfig {
	sanitized := proxy
	if sanitized.Source == "" {
		sanitized.Source = "url"
	}
	// proxy 模式：直接代理服务器，不需要订阅档案管理
	if sanitized.Source == "proxy" {
		if sanitized.SubscriptionName == "" {
			sanitized.SubscriptionName = "direct"
		}
		sanitized.CurrentProfile = sanitized.SubscriptionName
		sanitized.DirectProxy = sanitizeDirectProxy(sanitized.DirectProxy)
		return &sanitized
	}

	if sanitized.SubscriptionName == "" {
		sanitized.SubscriptionName = "subscription"
	}
	selectedName := strings.TrimSpace(sanitized.CurrentProfile)
	if selectedName == "" {
		selectedName = sanitized.SubscriptionName
	}

	active := sanitizeProxyProfile(ProxyProfile{
		Name:            sanitized.SubscriptionName,
		Source:          sanitized.Source,
		SubscriptionURL: sanitized.SubscriptionURL,
		ConfigFile:      sanitized.ConfigFile,
	})

	profiles := normalizeProxyProfiles(sanitized.Profiles)
	if len(profiles) == 0 {
		profiles = []ProxyProfile{active}
	}

	selectedIdx := indexOfProfile(profiles, selectedName)
	if selectedIdx < 0 {
		selectedIdx = indexOfProfile(profiles, active.Name)
	}

	switch {
	case selectedIdx >= 0:
		profiles[selectedIdx] = active
	case active.Name != "":
		profiles = append(profiles, active)
		selectedIdx = len(profiles) - 1
	default:
		selectedIdx = 0
	}

	selected := profiles[selectedIdx]
	sanitized.Source = selected.Source
	sanitized.SubscriptionURL = selected.SubscriptionURL
	sanitized.ConfigFile = selected.ConfigFile
	sanitized.SubscriptionName = selected.Name
	sanitized.CurrentProfile = selected.Name
	sanitized.Profiles = profiles
	return &sanitized
}

func sanitizeDirectProxy(dp *DirectProxyConfig) *DirectProxyConfig {
	if dp == nil {
		return &DirectProxyConfig{
			Name: "MyProxy",
			Type: "socks5",
		}
	}
	sanitized := *dp
	sanitized.Name = strings.TrimSpace(sanitized.Name)
	if sanitized.Name == "" {
		sanitized.Name = "MyProxy"
	}
	sanitized.Type = strings.ToLower(strings.TrimSpace(sanitized.Type))
	if sanitized.Type != "http" {
		sanitized.Type = "socks5"
	}
	sanitized.Server = strings.TrimSpace(sanitized.Server)
	sanitized.Username = strings.TrimSpace(sanitized.Username)
	sanitized.Password = strings.TrimSpace(sanitized.Password)
	return &sanitized
}

func sanitizeRuntime(runtime RuntimeConfig) *RuntimeConfig {
	sanitized := runtime
	defaults := DefaultConfig().Runtime.Ports
	if sanitized.Ports.Mixed == 0 {
		sanitized.Ports.Mixed = defaults.Mixed
	}
	if sanitized.Ports.Redir == 0 {
		sanitized.Ports.Redir = defaults.Redir
	}
	if sanitized.Ports.API == 0 {
		sanitized.Ports.API = defaults.API
	}
	if sanitized.Ports.DNS == 0 {
		sanitized.Ports.DNS = defaults.DNS
	}
	return &sanitized
}

func sanitizeProxyProfile(profile ProxyProfile) ProxyProfile {
	sanitized := profile
	sanitized.Name = strings.TrimSpace(sanitized.Name)
	sanitized.Source = strings.ToLower(strings.TrimSpace(sanitized.Source))
	if sanitized.Source == "" {
		sanitized.Source = "url"
	}
	// 支持三种来源模式
	if sanitized.Source != "url" && sanitized.Source != "file" && sanitized.Source != "proxy" {
		sanitized.Source = "url"
	}
	if sanitized.Name == "" {
		sanitized.Name = "subscription"
	}
	sanitized.SubscriptionURL = strings.TrimSpace(sanitized.SubscriptionURL)
	sanitized.ConfigFile = strings.TrimSpace(sanitized.ConfigFile)
	return sanitized
}

func normalizeProxyProfiles(profiles []ProxyProfile) []ProxyProfile {
	if len(profiles) == 0 {
		return nil
	}
	normalized := make([]ProxyProfile, 0, len(profiles))
	seen := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		sanitized := sanitizeProxyProfile(profile)
		if slices.Contains(seen, sanitized.Name) {
			continue
		}
		seen = append(seen, sanitized.Name)
		normalized = append(normalized, sanitized)
	}
	return normalized
}

func indexOfProfile(profiles []ProxyProfile, name string) int {
	name = strings.TrimSpace(name)
	if name == "" {
		return -1
	}
	for idx, profile := range profiles {
		if profile.Name == name {
			return idx
		}
	}
	return -1
}

// IsDirectProxy 返回当前是否使用直接代理服务器模式
func (p ProxyConfig) IsDirectProxy() bool {
	return p.Source == "proxy"
}

// IsConfigured 返回当前代理来源是否已完整配置
func (p ProxyConfig) IsConfigured() bool {
	switch p.Source {
	case "url":
		return strings.TrimSpace(p.SubscriptionURL) != ""
	case "file":
		return strings.TrimSpace(p.ConfigFile) != ""
	case "proxy":
		dp := p.DirectProxy
		return dp != nil && strings.TrimSpace(dp.Server) != "" && dp.Port > 0
	}
	return false
}

func (p ProxyConfig) ActiveProfile() ProxyProfile {
	profiles := normalizeProxyProfiles(p.Profiles)
	selected := strings.TrimSpace(p.CurrentProfile)
	if selected == "" {
		selected = p.SubscriptionName
	}
	if idx := indexOfProfile(profiles, selected); idx >= 0 {
		return profiles[idx]
	}
	return sanitizeProxyProfile(ProxyProfile{
		Name:            p.SubscriptionName,
		Source:          p.Source,
		SubscriptionURL: p.SubscriptionURL,
		ConfigFile:      p.ConfigFile,
	})
}

func sanitizeRules(rules RulesConfig) *RulesConfig {
	sanitized := rules
	return &sanitized
}

func sanitizeExtension(ext ExtensionConfig) *ExtensionConfig {
	sanitized := ExtensionConfig{
		Mode:       ext.Mode,
		ScriptPath: ext.ScriptPath,
	}
	if ext.ResidentialChain != nil {
		sanitized.ResidentialChain = cloneResidentialChain(ext.ResidentialChain)
		if sanitized.ResidentialChain.Mode == "" {
			sanitized.ResidentialChain.Mode = "rule"
		}
		if sanitized.ResidentialChain.ProxyType == "" {
			sanitized.ResidentialChain.ProxyType = "socks5"
		}
		if sanitized.ResidentialChain.AirportGroup == "" {
			sanitized.ResidentialChain.AirportGroup = "Auto"
		}
		sanitized.ResidentialChain.LegacyEnabled = false
	}
	return &sanitized
}

func cloneResidentialChain(chain *ResidentialChain) *ResidentialChain {
	if chain == nil {
		return nil
	}
	cloned := *chain
	return &cloned
}

func hasKeyPath(root *yaml.Node, keys ...string) bool {
	if root == nil || len(keys) == 0 || len(root.Content) == 0 {
		return false
	}

	node := root.Content[0]
	for _, key := range keys {
		if node.Kind != yaml.MappingNode {
			return false
		}

		found := false
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				node = node.Content[i+1]
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func boolDefault(v *bool, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return *v
}

func (r RulesConfig) LanDirectEnabled() bool {
	return boolDefault(r.LanDirect, true)
}

func (r RulesConfig) ChinaDirectEnabled() bool {
	return boolDefault(r.ChinaDirect, true)
}

func (r RulesConfig) AppleRulesEnabled() bool {
	return boolDefault(r.AppleRules, true)
}

func (r RulesConfig) NintendoProxyEnabled() bool {
	return boolDefault(r.NintendoProxy, true)
}

func (r RulesConfig) GlobalProxyEnabled() bool {
	return boolDefault(r.GlobalProxy, true)
}

func (r RulesConfig) AdsRejectEnabled() bool {
	return boolDefault(r.AdsReject, true)
}
