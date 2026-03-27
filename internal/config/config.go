package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ProxySource      string       `yaml:"proxy_source"`
	SubscriptionURL  string       `yaml:"subscription_url,omitempty"`
	ProxyConfigFile  string       `yaml:"proxy_config_file,omitempty"`
	SubscriptionName string       `yaml:"subscription_name"`
	Ports            PortsConfig  `yaml:"ports"`
	APISecret        string       `yaml:"api_secret,omitempty"`
	Regions          RegionConfig `yaml:"regions,omitempty"`
	UI               UIConfig     `yaml:"ui,omitempty"`
}

type UIConfig struct {
	Listen string `yaml:"listen,omitempty"`
}

type RegionConfig struct {
	Enabled    bool                `yaml:"enabled"`
	Include    []string            `yaml:"include,omitempty"`
	AutoSwitch bool                `yaml:"auto_switch"`
	Strategy   string              `yaml:"strategy,omitempty"`
	Mapping    map[string][]string `yaml:"mapping,omitempty"`
}

type PortsConfig struct {
	Mixed int `yaml:"mixed"`
	Redir int `yaml:"redir"`
	API   int `yaml:"api"`
	DNS   int `yaml:"dns"`
}

func DefaultConfig() *Config {
	return &Config{
		ProxySource:      "url",
		SubscriptionName: "subscription",
		Ports: PortsConfig{
			Mixed: 7890,
			Redir: 7892,
			API:   9090,
			DNS:   53,
		},
		Regions: RegionConfig{
			Enabled:    false,
			Include:    []string{},
			AutoSwitch: true,
			Strategy:   "latency",
			Mapping: map[string][]string{
				"HK": []string{"香港", "HK", "Hong Kong"},
				"JP": []string{"日本", "JP", "Tokyo", "Osaka"},
				"SG": []string{"新加坡", "SG", "Singapore"},
				"US": []string{"美国", "US", "United States", "Los Angeles", "San Jose", "Seattle"},
				"TW": []string{"台湾", "TW", "Taiwan", "Taipei"},
			},
		},
		UI: UIConfig{
			Listen: "127.0.0.1:9091",
		},
	}
}

func normalizeConfig(cfg *Config) {
	if cfg.Regions.Include == nil {
		cfg.Regions.Include = []string{}
	}
	if cfg.Regions.Strategy == "" {
		cfg.Regions.Strategy = "latency"
	}
	if cfg.Regions.Mapping == nil {
		cfg.Regions.Mapping = DefaultConfig().Regions.Mapping
	}
	if cfg.UI.Listen == "" {
		cfg.UI.Listen = DefaultConfig().UI.Listen
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	normalizeConfig(cfg)
	return cfg, nil
}

func Save(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	header := []byte("# lan-proxy-gateway 配置文件\n# 此文件包含敏感信息，请勿提交到 Git\n\n")
	return os.WriteFile(path, append(header, data...), 0600)
}
