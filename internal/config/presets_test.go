package config

import (
	"bytes"
	"os"
	"testing"
)

func TestDefaultPresetsAndSavedConfigurationIsolation(t *testing.T) {
	cfg := Default()
	if len(cfg.Routing.Rules) != 76 {
		t.Fatalf("expected 76 rules, got %d", len(cfg.Routing.Rules))
	}
	groups := map[string]bool{}
	for _, r := range cfg.Routing.Rules {
		groups[r.Group] = true
	}
	if len(groups) != 14 {
		t.Fatalf("expected 14 groups, got %d", len(groups))
	}
	if cfg.Routing.Rules[1].Type != "ip-cidr" || cfg.Routing.Rules[1].Value != "::1/128" || cfg.Routing.Rules[1].Action != "direct" {
		t.Fatal("IPv6 local rule not preserved")
	}
	Normalize(cfg)
	if err := Validate(cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Routing.Rules[0].Value = "modified"
	if Default().Routing.Rules[0].Value != "127.0.0.0/8" {
		t.Fatal("defaults share mutable state")
	}
	for _, suffix := range []string{"", "\nrouting:\n  rules: []\n"} {
		saved, err := Parse([]byte("version: 4\n" + suffix))
		if err != nil {
			t.Fatal(err)
		}
		if len(saved.Routing.Rules) != 0 {
			t.Fatal("loading existing config injected presets")
		}
	}
	ui, err := os.ReadFile("../../macos/Sources/LANProxyGatewayApp/Resources/default_routing.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ui, defaultRoutingJSON) {
		t.Fatal("UI presets differ from core defaults")
	}
}
