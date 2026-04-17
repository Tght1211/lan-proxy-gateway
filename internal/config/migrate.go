package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// v1Root captures the subset of the v1 schema we actually migrate.
type v1Root struct {
	Proxy struct {
		Source          string `yaml:"source"`
		SubscriptionURL string `yaml:"subscription_url"`
		ConfigFile      string `yaml:"config_file"`
		DirectProxy     struct {
			Server   string `yaml:"server"`
			Port     int    `yaml:"port"`
			Type     string `yaml:"type"`
			Username string `yaml:"username"`
			Password string `yaml:"password"`
		} `yaml:"direct_proxy"`
		SubscriptionName string `yaml:"subscription_name"`
	} `yaml:"proxy"`

	Runtime struct {
		Ports struct {
			Mixed int `yaml:"mixed"`
			Redir int `yaml:"redir"`
			API   int `yaml:"api"`
			DNS   int `yaml:"dns"`
		} `yaml:"ports"`
		APISecret string `yaml:"api_secret"`
		TUN       struct {
			Enabled     bool `yaml:"enabled"`
			BypassLocal bool `yaml:"bypass_local"`
		} `yaml:"tun"`
	} `yaml:"runtime"`

	Rules struct {
		LANDirect     *bool    `yaml:"lan_direct"`
		ChinaDirect   *bool    `yaml:"china_direct"`
		AppleRules    *bool    `yaml:"apple_rules"`
		NintendoProxy *bool    `yaml:"nintendo_proxy"`
		GlobalProxy   *bool    `yaml:"global_proxy"`
		AdsReject     *bool    `yaml:"ads_reject"`
		ExtraDirect   []string `yaml:"extra_direct_rules"`
		ExtraProxy    []string `yaml:"extra_proxy_rules"`
		ExtraReject   []string `yaml:"extra_reject_rules"`
	} `yaml:"rules"`

	Extension struct {
		Mode       string `yaml:"mode"`
		ScriptPath string `yaml:"script_path"`
	} `yaml:"extension"`
}

// MigrateV1 converts a v1 YAML document into a v2 Config, returning a list of
// human-readable notes about what was changed so the user knows their file shape
// drifted. The returned config still needs to be Normalize()d and Validate()d.
func MigrateV1(data []byte) (*Config, []string, error) {
	var old v1Root
	if err := yaml.Unmarshal(data, &old); err != nil {
		return nil, nil, fmt.Errorf("parse legacy gateway.yaml: %w", err)
	}
	cfg := Default()
	var notes []string

	// Runtime
	if old.Runtime.Ports.Mixed != 0 {
		cfg.Runtime.Ports.Mixed = old.Runtime.Ports.Mixed
	}
	if old.Runtime.Ports.Redir != 0 {
		cfg.Runtime.Ports.Redir = old.Runtime.Ports.Redir
	}
	if old.Runtime.Ports.API != 0 {
		cfg.Runtime.Ports.API = old.Runtime.Ports.API
	}
	if old.Runtime.Ports.DNS != 0 {
		cfg.Gateway.DNS.Port = old.Runtime.Ports.DNS
		notes = append(notes, "runtime.ports.dns 已迁移到 gateway.dns.port")
	}
	cfg.Runtime.APISecret = old.Runtime.APISecret
	cfg.Gateway.TUN.Enabled = old.Runtime.TUN.Enabled
	cfg.Gateway.TUN.BypassLocal = old.Runtime.TUN.BypassLocal

	// Rules
	if old.Rules.LANDirect != nil {
		cfg.Traffic.Rulesets.LANDirect = *old.Rules.LANDirect
	}
	if old.Rules.ChinaDirect != nil {
		cfg.Traffic.Rulesets.ChinaDirect = *old.Rules.ChinaDirect
	}
	if old.Rules.AppleRules != nil {
		cfg.Traffic.Rulesets.Apple = *old.Rules.AppleRules
	}
	if old.Rules.NintendoProxy != nil {
		cfg.Traffic.Rulesets.Nintendo = *old.Rules.NintendoProxy
	}
	if old.Rules.GlobalProxy != nil {
		cfg.Traffic.Rulesets.Global = *old.Rules.GlobalProxy
	}
	if old.Rules.AdsReject != nil {
		cfg.Traffic.Adblock = *old.Rules.AdsReject
		notes = append(notes, "rules.ads_reject 已迁移到 traffic.adblock")
	}
	cfg.Traffic.Extras.Direct = append(cfg.Traffic.Extras.Direct, old.Rules.ExtraDirect...)
	cfg.Traffic.Extras.Proxy = append(cfg.Traffic.Extras.Proxy, old.Rules.ExtraProxy...)
	cfg.Traffic.Extras.Reject = append(cfg.Traffic.Extras.Reject, old.Rules.ExtraReject...)

	// Proxy → Source
	switch old.Proxy.Source {
	case "url":
		cfg.Source.Type = SourceTypeSubscription
		cfg.Source.Subscription.URL = old.Proxy.SubscriptionURL
		if old.Proxy.SubscriptionName != "" {
			cfg.Source.Subscription.Name = old.Proxy.SubscriptionName
		}
	case "file":
		cfg.Source.Type = SourceTypeFile
		cfg.Source.File.Path = old.Proxy.ConfigFile
	case "proxy":
		if old.Proxy.DirectProxy.Server == "127.0.0.1" || old.Proxy.DirectProxy.Server == "localhost" {
			cfg.Source.Type = SourceTypeExternal
			cfg.Source.External = ExternalProxy{
				Name:   "本机已有代理",
				Server: old.Proxy.DirectProxy.Server,
				Port:   old.Proxy.DirectProxy.Port,
				Kind:   old.Proxy.DirectProxy.Type,
			}
		} else {
			cfg.Source.Type = SourceTypeRemote
			cfg.Source.Remote = RemoteProxy{
				Name:     "RemoteProxy",
				Kind:     old.Proxy.DirectProxy.Type,
				Server:   old.Proxy.DirectProxy.Server,
				Port:     old.Proxy.DirectProxy.Port,
				Username: old.Proxy.DirectProxy.Username,
				Password: old.Proxy.DirectProxy.Password,
			}
		}
	default:
		cfg.Source.Type = SourceTypeNone
	}
	notes = append(notes, fmt.Sprintf("proxy.source(%s) 已迁移到 source.type=%s", old.Proxy.Source, cfg.Source.Type))

	// Extension → Source.ScriptPath
	if old.Extension.Mode == "script" && old.Extension.ScriptPath != "" {
		cfg.Source.ScriptPath = old.Extension.ScriptPath
		notes = append(notes, "extension.script_path 已迁移到 source.script_path")
	} else if old.Extension.Mode == "chains" {
		notes = append(notes, "extension.mode=chains 已废弃，请改用 source.script_path 指向 scripts/residential-chain.js")
	}

	cfg.Version = Version
	return cfg, notes, nil
}
