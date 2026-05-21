package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultIsValid(t *testing.T) {
	cfg := Default()
	Normalize(cfg)
	if err := Validate(cfg); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}
	if cfg.Traffic.Mode != ModeRule {
		t.Errorf("default traffic.mode = %q, want %q", cfg.Traffic.Mode, ModeRule)
	}
	if !cfg.Traffic.Adblock {
		t.Errorf("default traffic.adblock should be true")
	}
	if cfg.Runtime.Ports.Mixed != 17890 {
		t.Errorf("default mixed port = %d, want 17890 (避开 Clash 默认 7890)", cfg.Runtime.Ports.Mixed)
	}
}

func TestValidateRejectsBadMode(t *testing.T) {
	cfg := Default()
	cfg.Traffic.Mode = "turbo"
	if err := Validate(cfg); err == nil {
		t.Fatalf("expected validation error for bogus mode")
	}
}

func TestValidateRejectsBadSource(t *testing.T) {
	cfg := Default()
	cfg.Source.Type = "magic"
	if err := Validate(cfg); err == nil {
		t.Fatalf("expected validation error for bogus source type")
	}
}

func TestMigrateV1_FileSource(t *testing.T) {
	yaml := `
proxy:
  source: file
  config_file: /tmp/clash.yaml
runtime:
  ports:
    mixed: 7890
    redir: 7892
    api: 9090
    dns: 53
  tun:
    enabled: true
    bypass_local: true
rules:
  ads_reject: false
  extra_direct_rules:
    - "DOMAIN-SUFFIX,corp.example.com,DIRECT"
extension:
  mode: chains
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse migrated config: %v", err)
	}
	if cfg.Source.Type != SourceTypeFile {
		t.Errorf("expected source.type=file, got %q", cfg.Source.Type)
	}
	if cfg.Source.File.Path != "/tmp/clash.yaml" {
		t.Errorf("expected file path migrated, got %q", cfg.Source.File.Path)
	}
	if cfg.Traffic.Adblock {
		t.Errorf("adblock should be false after migration")
	}
	if !cfg.Gateway.TUN.Enabled || !cfg.Gateway.TUN.BypassLocal {
		t.Errorf("TUN settings not migrated correctly")
	}
	found := false
	for _, r := range cfg.Traffic.Extras.Direct {
		if strings.Contains(r, "corp.example.com") {
			found = true
		}
	}
	if !found {
		t.Errorf("extra direct rule not migrated")
	}
}

func TestMigrateV1_LocalProxyBecomesExternal(t *testing.T) {
	yaml := `
proxy:
  source: proxy
  direct_proxy:
    server: 127.0.0.1
    port: 7890
    type: http
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Source.Type != SourceTypeExternal {
		t.Errorf("expected external, got %q", cfg.Source.Type)
	}
	if cfg.Source.External.Port != 7890 {
		t.Errorf("external port not migrated")
	}
}

func TestRoundTrip(t *testing.T) {
	cfg := Default()
	cfg.Source.Type = SourceTypeExternal
	Normalize(cfg)
	if err := Validate(cfg); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestUsesLocalExternalProxy(t *testing.T) {
	cfg := Default()
	cfg.Source.Type = SourceTypeExternal
	for _, host := range []string{"127.0.0.1", "localhost", "::1"} {
		cfg.Source.External.Server = host
		if !UsesLocalExternalProxy(cfg) {
			t.Fatalf("host %q should be treated as local external proxy", host)
		}
	}
	cfg.Source.External.Server = "192.168.1.2"
	if UsesLocalExternalProxy(cfg) {
		t.Fatal("LAN host should not be treated as local external proxy")
	}
}

func TestEffectiveRuntimeConfigRespectsTUNOffWithLocalExternalProxy(t *testing.T) {
	cfg := Default()
	cfg.Gateway.Enabled = true
	cfg.Gateway.Mode = GatewayModeTUN
	cfg.Gateway.TUN.Enabled = false
	cfg.Gateway.TUN.BypassLocal = false
	cfg.Gateway.DNS.Enabled = false
	cfg.Source.Type = SourceTypeExternal
	cfg.Source.External.Server = "127.0.0.1"

	effective := EffectiveRuntimeConfig(cfg)
	if effective == cfg {
		t.Fatal("EffectiveRuntimeConfig must return a copy")
	}
	if effective.Gateway.TUN.Enabled || effective.Gateway.DNS.Enabled || effective.Gateway.TUN.BypassLocal {
		t.Fatalf("local external proxy must not override an explicit TUN/DNS off state: %+v", effective.Gateway)
	}
	if !cfg.Gateway.Enabled || cfg.Gateway.TUN.Enabled || cfg.Gateway.TUN.BypassLocal || cfg.Gateway.DNS.Enabled {
		t.Fatal("original config should not be mutated")
	}
}

func TestEffectiveRuntimeConfigProtectsLocalExternalProxyWhenTUNOn(t *testing.T) {
	cfg := Default()
	cfg.Gateway.Enabled = true
	cfg.Gateway.Mode = GatewayModeTUN
	cfg.Gateway.TUN.Enabled = true
	cfg.Gateway.TUN.BypassLocal = false
	cfg.Source.Type = SourceTypeExternal
	cfg.Source.External.Server = "127.0.0.1"

	effective := EffectiveRuntimeConfig(cfg)
	if !effective.Gateway.TUN.Enabled {
		t.Fatalf("local external proxy should keep enabled TUN on: %+v", effective.Gateway.TUN)
	}
	if !effective.Gateway.TUN.BypassLocal {
		t.Fatalf("local external proxy should force local bypass only when TUN is on: %+v", effective.Gateway.TUN)
	}
	if !cfg.Gateway.TUN.Enabled || cfg.Gateway.TUN.BypassLocal {
		t.Fatal("original config should not be mutated")
	}
}

func TestDefaultModeIsTUN(t *testing.T) {
	// 默认 mode 必须是 tun —— 一键式旁路由是本项目卖点，新用户跑起来就该
	// 把投影仪/Switch/AppleTV 接入网关。要"零干扰本机"的用户菜单切 forward 即可。
	cfg := Default()
	if cfg.Gateway.Mode != GatewayModeTUN {
		t.Fatalf("Default().Gateway.Mode = %q, want %q", cfg.Gateway.Mode, GatewayModeTUN)
	}
	if !cfg.Gateway.TUN.Enabled {
		t.Fatalf("Default().Gateway.TUN.Enabled must be true (旁路由开箱即用)")
	}
}

func TestEffectiveRuntimeConfig_ForwardWithLocalExternalProxy(t *testing.T) {
	cfg := Default()
	cfg.Gateway.Enabled = true
	cfg.Gateway.Mode = GatewayModeForward
	cfg.Gateway.TUN.Enabled = false
	cfg.Source.Type = SourceTypeExternal
	cfg.Source.External.Server = "127.0.0.1"
	cfg.Source.External.Port = 7890
	cfg.Source.External.Kind = "http"

	effective := EffectiveRuntimeConfig(cfg)
	if effective.Gateway.TUN.Enabled || effective.Gateway.TUN.BypassLocal {
		t.Fatalf("forward + local-external 不应强制打开 TUN：%+v", effective.Gateway.TUN)
	}
}

func TestNormalize_AutoGenWebUIToken(t *testing.T) {
	cfg := &Config{}
	Normalize(cfg)
	if cfg.Runtime.WebUIToken == "" {
		t.Fatal("Normalize 必须自动生成 WebUIToken，不能留空")
	}
	if len(cfg.Runtime.WebUIToken) != 32 {
		t.Fatalf("WebUIToken 应是 32 字符 hex；got len=%d", len(cfg.Runtime.WebUIToken))
	}
	// Normalize 再跑一次不应覆盖已存在的 token（否则用户保留的 token 会丢）。
	saved := cfg.Runtime.WebUIToken
	Normalize(cfg)
	if cfg.Runtime.WebUIToken != saved {
		t.Fatalf("Normalize 不应覆盖已有 token；before=%q after=%q", saved, cfg.Runtime.WebUIToken)
	}
}

func TestLoadPersistsGeneratedWebUIToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.yaml")
	data := []byte(`
version: 2
gateway:
  enabled: true
traffic:
  mode: rule
source:
  type: none
runtime:
  ports:
    mixed: 17890
    redir: 17892
    api: 19090
    web_ui: 19091
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, migrated, normalized, err := loadFrom(path)
	if err != nil {
		t.Fatalf("loadFrom: %v", err)
	}
	if len(migrated) != 0 {
		t.Fatalf("unexpected migration notes: %v", migrated)
	}
	if !normalized {
		t.Fatal("missing web_ui_token should mark config as normalized")
	}
	if cfg.Runtime.WebUIToken == "" {
		t.Fatal("expected generated web_ui_token")
	}
}

func TestEffectiveRuntimeConfigForwardModeDoesNotForceOffTUN(t *testing.T) {
	// forward 只表示网关层使用端口转发策略；TUN 是独立能力，WebUI 允许两者同时开。
	cfg := Default()
	cfg.Gateway.Enabled = true
	cfg.Gateway.Mode = GatewayModeForward
	cfg.Gateway.TUN.Enabled = true
	cfg.Gateway.TUN.BypassLocal = false

	effective := EffectiveRuntimeConfig(cfg)
	if !effective.Gateway.TUN.Enabled {
		t.Fatalf("forward 模式不应强制关闭 TUN：%+v", effective.Gateway.TUN)
	}
}

func TestEffectiveRuntimeConfigAllowsPortOnlyModeWhenGatewayDisabled(t *testing.T) {
	cfg := Default()
	cfg.Gateway.Enabled = false
	cfg.Gateway.TUN.Enabled = false
	cfg.Gateway.DNS.Enabled = false
	cfg.Source.Type = SourceTypeExternal
	cfg.Source.External.Server = "127.0.0.1"

	effective := EffectiveRuntimeConfig(cfg)
	if effective.Gateway.Enabled || effective.Gateway.TUN.Enabled || effective.Gateway.DNS.Enabled {
		t.Fatalf("disabled gateway should stay port-only: %+v", effective.Gateway)
	}
	if effective.Gateway.TUN.BypassLocal {
		t.Fatalf("local external proxy should not force local bypass when TUN is off")
	}
}
