package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultValidAndRoundtrip(t *testing.T) {
	cfg := Default()
	Normalize(cfg)
	if !cfg.DNS.FakeIP || cfg.DNS.Hijack {
		t.Fatal("proxy-safe defaults must enable fake-ip without DNS hijacking")
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.yaml")
	if err := Save(cfg, path); err != nil {
		t.Fatal(err)
	}
	got, err := loadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Egress.Mode != EgressDirect || got.DNS.Port != 53 || got.Runtime.RedirPort != 17892 {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
	if len(got.Routing.Rules) != 0 {
		t.Fatalf("default routing rules = %+v", got.Routing.Rules)
	}
}

func TestParseNormalizes(t *testing.T) {
	yaml := `
version: 4
egress:
  mode: proxy
  proxy:
    type: socks5
    host: 127.0.0.1
    port: 1080
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Egress.Mode != EgressProxy {
		t.Fatalf("mode = %q", cfg.Egress.Mode)
	}
	if cfg.DNS.Port != 53 || cfg.Runtime.APIPort != 19090 || !cfg.QUICBlock {
		t.Fatalf("defaults not applied: %+v", cfg)
	}
}

func TestParseIgnoresExperimentalUDPFieldsAndKeepsQUICBlocked(t *testing.T) {
	cfg, err := Parse([]byte(`
version: 4
egress:
  mode: proxy
  proxy:
    type: http
    host: 127.0.0.1
    port: 7897
udp:
  mode: block
runtime:
  udp_port: 17893
`))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.QUICBlock {
		t.Fatal("proxy config without quic_block must retain the safe default")
	}
}

func TestValidateErrors(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{"bad mode", "version: 4\negress:\n  mode: turbo\n", "egress.mode"},
		{"bad proxy type", "version: 4\negress:\n  mode: proxy\n  proxy:\n    type: vless\n    host: h\n    port: 1\n", "egress.proxy.type"},
		{"bad port", "version: 4\negress:\n  mode: proxy\n  proxy:\n    type: http\n    host: h\n    port: 99999\n", "port"},
		{"bad fake range", "version: 4\nruntime:\n  fake_ip_range: nope\n", "fake_ip_range"},
		{"bad route type", "version: 4\nrouting:\n  rules:\n    - type: process-name\n      value: app\n      action: proxy\n", "routing.rules[0].type"},
		{"bad route action", "version: 4\nrouting:\n  rules:\n    - type: domain-suffix\n      value: example.com\n      action: drop\n", "routing.rules[0].action"},
		{"bad cidr value", "version: 4\nrouting:\n  rules:\n    - type: ip-cidr\n      value: 10.0.0.1\n      action: direct\n", "routing.rules[0].value"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse([]byte(c.yaml))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want error containing %q, got %v", c.want, err)
			}
		})
	}
}

const legacyV2Fixture = `
version: 2
gateway:
  enabled: true
  mode: tun
traffic:
  mode: rule
  adblock: true
source:
  type: subscription
  subscription:
    url: https://example.com/sub
`

func TestLegacyConfigBackedUpAndNotConfigured(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.yaml")
	if err := os.WriteFile(path, []byte(legacyV2Fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := loadFrom(path)
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("original file should have been renamed")
	}
	bak := path + ".pre-v4.bak"
	data, err := os.ReadFile(bak)
	if err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	if string(data) != legacyV2Fixture {
		t.Fatal("backup content mismatch")
	}

	// second legacy file → collision gets a timestamped name
	if err := os.WriteFile(path, []byte(legacyV2Fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadFrom(path); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured again, got %v", err)
	}
	matches, _ := filepath.Glob(path + ".pre-v4.bak.*")
	if len(matches) != 1 {
		t.Fatalf("want timestamped backup, got %v", matches)
	}
}

func TestLegacyDetectedByForeignKeys(t *testing.T) {
	// version=4 but still carrying mihomo-era top-level keys
	yaml := "version: 4\nsource:\n  type: none\negress:\n  mode: direct\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadFrom(path); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
}

func TestParseOldVersionInMemory(t *testing.T) {
	if _, err := Parse([]byte(legacyV2Fixture)); err == nil {
		t.Fatal("want old-version error")
	}
}
