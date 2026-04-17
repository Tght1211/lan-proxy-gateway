// Package config holds the v2 user-facing configuration for lan-proxy-gateway.
//
// The config maps 1:1 to the three user layers:
//   - Gateway  : main feature, LAN transparent gateway (IP forward + TUN + DNS)
//   - Traffic  : mode + adblock + rule sets
//   - Source   : proxy source (external/subscription/file/remote/none) + optional script
package config

// Version is the current schema version. Bump when a breaking change lands.
const Version = 2

// Config is the root configuration persisted at ~/.config/lan-proxy-gateway/gateway.yaml.
type Config struct {
	Version int            `yaml:"version"`
	Gateway GatewayConfig  `yaml:"gateway"`
	Traffic TrafficConfig  `yaml:"traffic"`
	Source  SourceConfig   `yaml:"source"`
	Runtime RuntimeConfig  `yaml:"runtime"`
}

// GatewayConfig drives the LAN gateway (the "main" feature).
type GatewayConfig struct {
	Enabled bool      `yaml:"enabled"`
	TUN     TUNConfig `yaml:"tun"`
	DNS     DNSConfig `yaml:"dns"`
}

// TUNConfig toggles the TUN virtual interface.
type TUNConfig struct {
	Enabled     bool `yaml:"enabled"`
	BypassLocal bool `yaml:"bypass_local"`
}

// DNSConfig toggles the DNS listener exposed to LAN devices.
type DNSConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

// TrafficConfig is the traffic policy (the "sub" feature).
type TrafficConfig struct {
	Mode     string          `yaml:"mode"`    // rule | global | direct
	Adblock  bool            `yaml:"adblock"`
	Extras   ExtraRules      `yaml:"extras"`
	Rulesets RulesetToggles  `yaml:"rulesets"`
}

// ExtraRules lets the user add custom rules without touching rulesets.
type ExtraRules struct {
	Direct []string `yaml:"direct"`
	Proxy  []string `yaml:"proxy"`
	Reject []string `yaml:"reject"`
}

// RulesetToggles enables/disables the built-in rule collections.
type RulesetToggles struct {
	ChinaDirect bool `yaml:"china_direct"`
	Apple       bool `yaml:"apple"`
	Nintendo    bool `yaml:"nintendo"`
	Global      bool `yaml:"global"`
	LANDirect   bool `yaml:"lan_direct"`
}

// SourceConfig is the proxy source (the "extension" feature).
type SourceConfig struct {
	Type         string               `yaml:"type"` // external | subscription | file | remote | none
	External     ExternalProxy        `yaml:"external"`
	Subscription SubscriptionSource   `yaml:"subscription"`
	File         FileSource           `yaml:"file"`
	Remote       RemoteProxy          `yaml:"remote"`
	ScriptPath   string               `yaml:"script_path"`
	Profiles     []Profile            `yaml:"profiles"`
	Current      string               `yaml:"current"`
}

// ExternalProxy points to a proxy port already running on localhost.
// Ideal for users who already run Clash Verge, Shadowrocket, etc.
type ExternalProxy struct {
	Name   string `yaml:"name"`
	Server string `yaml:"server"`
	Port   int    `yaml:"port"`
	Kind   string `yaml:"kind"` // http | socks5
}

// SubscriptionSource fetches a Clash subscription via HTTP.
type SubscriptionSource struct {
	URL  string `yaml:"url"`
	Name string `yaml:"name"`
}

// FileSource loads a local Clash/mihomo YAML file.
type FileSource struct {
	Path string `yaml:"path"`
}

// RemoteProxy is a single remote socks5/http proxy.
type RemoteProxy struct {
	Name     string `yaml:"name"`
	Kind     string `yaml:"kind"` // http | socks5
	Server   string `yaml:"server"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

// Profile is a named source preset (advanced).
type Profile struct {
	Name         string             `yaml:"name"`
	Type         string             `yaml:"type"`
	External     *ExternalProxy     `yaml:"external,omitempty"`
	Subscription *SubscriptionSource `yaml:"subscription,omitempty"`
	File         *FileSource        `yaml:"file,omitempty"`
	Remote       *RemoteProxy       `yaml:"remote,omitempty"`
}

// RuntimeConfig groups technical settings (ports, secrets, logging).
type RuntimeConfig struct {
	Ports     RuntimePorts `yaml:"ports"`
	APISecret string       `yaml:"api_secret"`
	LogLevel  string       `yaml:"log_level"`
}

// RuntimePorts are the listen ports exposed by mihomo.
type RuntimePorts struct {
	Mixed int `yaml:"mixed"`
	Redir int `yaml:"redir"`
	API   int `yaml:"api"`
}

// Default returns a fresh config with sensible defaults for first-time users.
// Defaults: LAN gateway on, TUN on, rule mode, adblock on, external proxy at 127.0.0.1:7890.
func Default() *Config {
	return &Config{
		Version: Version,
		Gateway: GatewayConfig{
			Enabled: true,
			TUN:     TUNConfig{Enabled: true, BypassLocal: false},
			DNS:     DNSConfig{Enabled: true, Port: 53},
		},
		Traffic: TrafficConfig{
			Mode:    ModeRule,
			Adblock: true,
			Extras:  ExtraRules{},
			Rulesets: RulesetToggles{
				ChinaDirect: true,
				Apple:       true,
				Nintendo:    true,
				Global:      true,
				LANDirect:   true,
			},
		},
		Source: SourceConfig{
			Type: SourceTypeNone,
			External: ExternalProxy{
				Name:   "本机已有代理",
				Server: "127.0.0.1",
				Port:   7890,
				Kind:   "http",
			},
		},
		Runtime: RuntimeConfig{
			Ports:    RuntimePorts{Mixed: 7890, Redir: 7892, API: 9090},
			LogLevel: "warning",
		},
	}
}

// Traffic mode constants. These mirror clash/mihomo's native `mode:` values.
const (
	ModeRule   = "rule"
	ModeGlobal = "global"
	ModeDirect = "direct"
)

// Source type constants.
const (
	SourceTypeExternal     = "external"
	SourceTypeSubscription = "subscription"
	SourceTypeFile         = "file"
	SourceTypeRemote       = "remote"
	SourceTypeNone         = "none"
)
