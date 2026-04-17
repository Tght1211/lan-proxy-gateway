package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPreservesNestedProxyConfig(t *testing.T) {
	path := writeTempConfig(t, `
proxy:
  source: file
  config_file: /tmp/demo.yaml
  subscription_name: demo
runtime:
  ports:
    mixed: 7890
    redir: 7892
    api: 9090
    dns: 53
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Proxy.Source != "file" {
		t.Fatalf("Proxy.Source = %q, want file", cfg.Proxy.Source)
	}
	if cfg.Proxy.ConfigFile != "/tmp/demo.yaml" {
		t.Fatalf("Proxy.ConfigFile = %q", cfg.Proxy.ConfigFile)
	}
	if cfg.Proxy.SubscriptionName != "demo" {
		t.Fatalf("Proxy.SubscriptionName = %q, want demo", cfg.Proxy.SubscriptionName)
	}
}

func TestLoadMigratesLegacyProxyConfig(t *testing.T) {
	path := writeTempConfig(t, `
proxy_source: file
proxy_config_file: /tmp/demo.yaml
subscription_name: demo
ports:
  mixed: 7890
  redir: 7892
  api: 9090
  dns: 53
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Proxy.Source != "file" {
		t.Fatalf("Proxy.Source = %q, want file", cfg.Proxy.Source)
	}
	if cfg.Proxy.ConfigFile != "/tmp/demo.yaml" {
		t.Fatalf("Proxy.ConfigFile = %q, want /tmp/demo.yaml", cfg.Proxy.ConfigFile)
	}
	if cfg.Proxy.SubscriptionName != "demo" {
		t.Fatalf("Proxy.SubscriptionName = %q, want demo", cfg.Proxy.SubscriptionName)
	}
}

func TestLoadMigratesLegacyRuntimeConfig(t *testing.T) {
	path := writeTempConfig(t, `
proxy_source: url
subscription_name: demo
ports:
  mixed: 17890
  redir: 17892
  api: 19090
  dns: 1053
api_secret: legacy-secret
tun_enabled: true
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Runtime.Ports.Mixed != 17890 {
		t.Fatalf("Runtime.Ports.Mixed = %d, want 17890", cfg.Runtime.Ports.Mixed)
	}
	if cfg.Runtime.Ports.Redir != 17892 {
		t.Fatalf("Runtime.Ports.Redir = %d, want 17892", cfg.Runtime.Ports.Redir)
	}
	if cfg.Runtime.Ports.API != 19090 {
		t.Fatalf("Runtime.Ports.API = %d, want 19090", cfg.Runtime.Ports.API)
	}
	if cfg.Runtime.Ports.DNS != 1053 {
		t.Fatalf("Runtime.Ports.DNS = %d, want 1053", cfg.Runtime.Ports.DNS)
	}
	if cfg.Runtime.APISecret != "legacy-secret" {
		t.Fatalf("Runtime.APISecret = %q, want legacy-secret", cfg.Runtime.APISecret)
	}
	if !cfg.Runtime.Tun.Enabled {
		t.Fatal("Runtime.Tun.Enabled = false, want true")
	}
}

func TestLoadPreservesNestedRuntimeConfig(t *testing.T) {
	path := writeTempConfig(t, `
proxy:
  source: url
  subscription_name: demo
runtime:
  ports:
    mixed: 17890
    redir: 17892
    api: 19090
    dns: 1053
  api_secret: nested-secret
  tun:
    enabled: true
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Runtime.Ports.Mixed != 17890 {
		t.Fatalf("Runtime.Ports.Mixed = %d, want 17890", cfg.Runtime.Ports.Mixed)
	}
	if cfg.Runtime.APISecret != "nested-secret" {
		t.Fatalf("Runtime.APISecret = %q, want nested-secret", cfg.Runtime.APISecret)
	}
	if !cfg.Runtime.Tun.Enabled {
		t.Fatal("Runtime.Tun.Enabled = false, want true")
	}
}

func TestLoadFullLegacyConfigCompatibility(t *testing.T) {
	path := writeTempConfig(t, `
proxy_source: file
proxy_config_file: /tmp/legacy.yaml
subscription_name: legacy-sub
ports:
  mixed: 17890
  redir: 17892
  api: 19090
  dns: 1053
api_secret: legacy-secret
tun_enabled: true
script_path: ./legacy-script.js
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Proxy.Source != "file" {
		t.Fatalf("Proxy.Source = %q, want file", cfg.Proxy.Source)
	}
	if cfg.Proxy.ConfigFile != "/tmp/legacy.yaml" {
		t.Fatalf("Proxy.ConfigFile = %q, want /tmp/legacy.yaml", cfg.Proxy.ConfigFile)
	}
	if cfg.Proxy.SubscriptionName != "legacy-sub" {
		t.Fatalf("Proxy.SubscriptionName = %q, want legacy-sub", cfg.Proxy.SubscriptionName)
	}
	if cfg.Runtime.Ports.API != 19090 {
		t.Fatalf("Runtime.Ports.API = %d, want 19090", cfg.Runtime.Ports.API)
	}
	if cfg.Runtime.APISecret != "legacy-secret" {
		t.Fatalf("Runtime.APISecret = %q, want legacy-secret", cfg.Runtime.APISecret)
	}
	if !cfg.Runtime.Tun.Enabled {
		t.Fatal("Runtime.Tun.Enabled = false, want true")
	}
	if cfg.Extension.Mode != "script" {
		t.Fatalf("Extension.Mode = %q, want script", cfg.Extension.Mode)
	}
	if cfg.Extension.ScriptPath != "./legacy-script.js" {
		t.Fatalf("Extension.ScriptPath = %q, want ./legacy-script.js", cfg.Extension.ScriptPath)
	}
}

func TestLoadPreservesExplicitEmptyExtensionMode(t *testing.T) {
	path := writeTempConfig(t, `
proxy:
  source: url
  subscription_name: demo
runtime:
  ports:
    mixed: 7890
    redir: 7892
    api: 9090
    dns: 53
extension:
  mode: ""
  script_path: ./script-demo.js
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Extension.Mode != "" {
		t.Fatalf("Extension.Mode = %q, want empty", cfg.Extension.Mode)
	}
}

func TestLoadPreservesNestedScriptMode(t *testing.T) {
	path := writeTempConfig(t, `
proxy:
  source: url
  subscription_name: demo
runtime:
  ports:
    mixed: 7890
    redir: 7892
    api: 9090
    dns: 53
extension:
  mode: script
  script_path: ./script-demo.js
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Extension.Mode != "script" {
		t.Fatalf("Extension.Mode = %q, want script", cfg.Extension.Mode)
	}
	if cfg.Extension.ScriptPath != "./script-demo.js" {
		t.Fatalf("Extension.ScriptPath = %q, want ./script-demo.js", cfg.Extension.ScriptPath)
	}
}

func TestLoadMigratesLegacyScriptMode(t *testing.T) {
	path := writeTempConfig(t, `
proxy_source: url
subscription_name: demo
ports:
  mixed: 7890
  redir: 7892
  api: 9090
  dns: 53
script_path: ./script-demo.js
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Extension.Mode != "script" {
		t.Fatalf("Extension.Mode = %q, want script", cfg.Extension.Mode)
	}
}

func TestLoadPreservesNestedChainsMode(t *testing.T) {
	path := writeTempConfig(t, `
proxy:
  source: url
  subscription_name: demo
runtime:
  ports:
    mixed: 7890
    redir: 7892
    api: 9090
    dns: 53
extension:
  mode: chains
  residential_chain:
    proxy_server: 1.2.3.4
    proxy_port: 443
    proxy_type: socks5
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Extension.Mode != "chains" {
		t.Fatalf("Extension.Mode = %q, want chains", cfg.Extension.Mode)
	}
	if cfg.Extension.ResidentialChain == nil {
		t.Fatal("Extension.ResidentialChain = nil, want populated config")
	}
	if cfg.Extension.ResidentialChain.LegacyEnabled {
		t.Fatal("LegacyEnabled should be cleared after load")
	}
}

func TestLoadMigratesLegacyChainsMode(t *testing.T) {
	path := writeTempConfig(t, `
proxy_source: url
subscription_name: demo
ports:
  mixed: 7890
  redir: 7892
  api: 9090
  dns: 53
residential_chain:
  enabled: true
  proxy_server: 1.2.3.4
  proxy_port: 443
  proxy_type: socks5
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Extension.Mode != "chains" {
		t.Fatalf("Extension.Mode = %q, want chains", cfg.Extension.Mode)
	}
	if cfg.Extension.ResidentialChain == nil {
		t.Fatal("Extension.ResidentialChain = nil, want populated config")
	}
}

func TestSaveKeepsNestedConfigBlocks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.yaml")
	cfg := &Config{
		Proxy: ProxyConfig{
			Source:           "url",
			SubscriptionURL:  "https://example.com/sub",
			SubscriptionName: "demo",
		},
		Runtime: RuntimeConfig{
			Ports: PortsConfig{
				Mixed: 7890,
				Redir: 7892,
				API:   9090,
				DNS:   53,
			},
			APISecret: "secret",
			Tun: TunConfig{
				Enabled: true,
			},
		},
		Extension: ExtensionConfig{
			Mode:       "",
			ScriptPath: "./script-demo.js",
			ResidentialChain: &ResidentialChain{
				LegacyEnabled: true,
				ProxyServer:   "1.2.3.4",
				ProxyPort:     443,
				ProxyType:     "socks5",
			},
		},
	}

	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "proxy:\n") || !strings.Contains(content, "source: url") {
		t.Fatalf("saved config missing nested proxy block:\n%s", content)
	}
	if !strings.Contains(content, "runtime:\n") || !strings.Contains(content, "enabled: true") {
		t.Fatalf("saved config missing nested runtime block:\n%s", content)
	}
	if !strings.Contains(content, "extension:\n") || !strings.Contains(content, "mode: \"\"") {
		t.Fatalf("saved config missing nested extension block:\n%s", content)
	}
	if strings.Contains(content, "residential_chain:\n    enabled: true") {
		t.Fatalf("saved config should not include legacy enabled flag:\n%s", content)
	}
}

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gateway.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}
