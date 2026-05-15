// Package config holds the v2 user-facing configuration for lan-proxy-gateway.
//
// The config maps 1:1 to the three user layers:
//   - Gateway  : main feature, LAN transparent gateway (IP forward + TUN + DNS)
//   - Traffic  : mode + adblock + rule sets
//   - Source   : proxy source (external/subscription/file/remote/none) + optional script
package config

import (
	"net"
	"strings"
)

// Version is the current schema version. Bump when a breaking change lands.
const Version = 2

// Config is the root configuration persisted at ~/.config/lan-proxy-gateway/gateway.yaml.
type Config struct {
	Version int           `yaml:"version"`
	Gateway GatewayConfig `yaml:"gateway"`
	Traffic TrafficConfig `yaml:"traffic"`
	Source  SourceConfig  `yaml:"source"`
	Runtime RuntimeConfig `yaml:"runtime"`
}

// GatewayConfig drives the LAN gateway (the "main" feature).
type GatewayConfig struct {
	Enabled bool      `yaml:"enabled"`
	TUN     TUNConfig `yaml:"tun"`
	DNS     DNSConfig `yaml:"dns"`
	// DeviceLabels 把 LAN 设备 IP 映射成人读的名字（例如 "192.168.1.23" → "Switch"），
	// 给仪表盘设备表用。反向 DNS 拿不到/不准时用户可以在菜单里手动打标签覆盖。
	DeviceLabels map[string]string `yaml:"device_labels,omitempty"`
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
	Mode     string         `yaml:"mode"` // rule | global | direct
	Adblock  bool           `yaml:"adblock"`
	Extras   ExtraRules     `yaml:"extras"`
	Rulesets RulesetToggles `yaml:"rulesets"`
	// AutoGroups 开启后，subscription / file 源在渲染时若发现用户订阅里没有
	// url-test / fallback 类型的策略组，会自动追加 Auto + Fallback 组，引用
	// 订阅里全部节点。v2.x 的模板里默认就有这两个组，v3 重写时丢了这个能力，
	// 本字段是把能力补回来。默认 false：升级用户 config 不主动变，想要的用户
	// 在菜单 [M] → 2 → 自动补全策略组 主动开启。
	AutoGroups bool `yaml:"auto_groups,omitempty"`
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
	Type         string             `yaml:"type"` // external | subscription | file | remote | none
	External     ExternalProxy      `yaml:"external"`
	Subscription SubscriptionSource `yaml:"subscription"`
	File         FileSource         `yaml:"file"`
	Remote       RemoteProxy        `yaml:"remote"`
	// ScriptPath 是用户自定义 .js 文件的绝对路径（高级用户用）。
	// 如果同时设置了 ChainResidential，会优先用 ChainResidential 渲染出的预设脚本。
	ScriptPath string `yaml:"script_path"`
	// ChainResidential 非 nil 时，render 阶段会用 preset 模板生成链式代理脚本，
	// 覆盖 ScriptPath 指向渲染后的文件。字段为空时不启用链式代理。
	ChainResidential *ChainResidentialConfig `yaml:"chain_residential,omitempty"`
	Profiles         []Profile               `yaml:"profiles"`
	Current          string                  `yaml:"current"`
}

// ChainResidentialConfig 是「链式代理 · 住宅 IP 落地」预设需要的用户填写字段。
// 用 gateway 主菜单 → 换代理源 → S 增强脚本 → 预设向导交互式填写即可。
// 对应脚本会把订阅的机场节点组合成「🛫 AI起飞节点」，
// 把这里的住宅 IP 节点组合成「🛬 AI落地节点」，然后用 dialer-proxy 串成链式代理。
type ChainResidentialConfig struct {
	Name        string `yaml:"name"`         // 节点名（会出现在菜单节点列表里），例: "🏠 住宅IP-美国"
	Kind        string `yaml:"kind"`         // "http" | "socks5"
	Server      string `yaml:"server"`       // 主机
	Port        int    `yaml:"port"`         // 端口
	Username    string `yaml:"username"`     // 可空
	Password    string `yaml:"password"`     // 可空
	DialerProxy string `yaml:"dialer_proxy"` // 链式代理第一跳组名，默认 "🛫 AI起飞节点"
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
	Name         string              `yaml:"name"`
	Type         string              `yaml:"type"`
	External     *ExternalProxy      `yaml:"external,omitempty"`
	Subscription *SubscriptionSource `yaml:"subscription,omitempty"`
	File         *FileSource         `yaml:"file,omitempty"`
	Remote       *RemoteProxy        `yaml:"remote,omitempty"`
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
//
// 本机 Runtime 端口避开主流 VPN 客户端的默认值（Clash/mihomo/Clash Verge 都爱占
// 7890/7892/9090/7897/9097），改成 17890/17892/19090，减少端口冲突。
// Source.External 默认仍是 7890 —— 用户本机 Clash/Verge 的上游端口大多是这个。
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
			Ports:    RuntimePorts{Mixed: 17890, Redir: 17892, API: 19090},
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

// UsesLocalExternalProxy reports whether gateway is chained behind another
// proxy client on the same host, e.g. Clash Verge / Mihomo Party at 127.0.0.1.
// In this shape gateway must keep the local host out of strict TUN capture,
// otherwise the upstream client's own outbound traffic can be captured again
// and loop back through gateway.
func UsesLocalExternalProxy(cfg *Config) bool {
	if cfg == nil || cfg.Source.Type != SourceTypeExternal {
		return false
	}
	return IsLoopbackHost(cfg.Source.External.Server)
}

// EffectiveRuntimeConfig returns a copy adjusted for runtime safety.
//
// When gateway chains behind another proxy on the same machine, strict TUN
// routing can capture that upstream proxy client's own outbound traffic and
// create a self-proxy loop. Preserve the normal LAN gateway features, but force
// TUN/DNS on plus local bypass so the rendered mihomo config uses
// strict-route: false.
func EffectiveRuntimeConfig(cfg *Config) *Config {
	if cfg == nil {
		return nil
	}
	out := *cfg
	if UsesLocalExternalProxy(cfg) {
		if out.Gateway.Enabled {
			out.Gateway.TUN.Enabled = true
			out.Gateway.DNS.Enabled = true
		}
		out.Gateway.TUN.BypassLocal = true
	}
	return &out
}

func IsLoopbackHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}
