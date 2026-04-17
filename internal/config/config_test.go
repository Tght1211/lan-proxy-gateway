package config

import (
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
	if cfg.Runtime.Ports.Mixed != 7890 {
		t.Errorf("default mixed port = %d, want 7890", cfg.Runtime.Ports.Mixed)
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
