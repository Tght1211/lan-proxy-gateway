package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_OldConfigKeepsCompatibility(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.yaml")

	content := `proxy_source: url
subscription_url: https://example.com/sub
subscription_name: demo
ports:
  mixed: 7890
  redir: 7892
  api: 9090
  dns: 53
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Regions.Strategy != "latency" {
		t.Fatalf("expected default strategy latency, got %q", cfg.Regions.Strategy)
	}
	if cfg.Regions.Include == nil {
		t.Fatalf("expected regions include to be initialized")
	}
	if len(cfg.Regions.Mapping) == 0 {
		t.Fatalf("expected default regions mapping")
	}
}

func TestSaveAndLoad_RegionsConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.yaml")

	cfg := DefaultConfig()
	cfg.Regions.Enabled = true
	cfg.Regions.Include = []string{"HK", "JP"}
	cfg.Regions.AutoSwitch = true

	if err := Save(cfg, path); err != nil {
		t.Fatalf("save config: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if !loaded.Regions.Enabled {
		t.Fatalf("expected regions enabled")
	}
	if len(loaded.Regions.Include) != 2 {
		t.Fatalf("expected 2 regions, got %d", len(loaded.Regions.Include))
	}
}
