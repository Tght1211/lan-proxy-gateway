package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ProxySource      string      `yaml:"proxy_source"`
	SubscriptionURL  string      `yaml:"subscription_url,omitempty"`
	ProxyConfigFile  string      `yaml:"proxy_config_file,omitempty"`
	SubscriptionName string      `yaml:"subscription_name"`
	Ports            PortsConfig `yaml:"ports"`
	APISecret        string      `yaml:"api_secret,omitempty"`
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
