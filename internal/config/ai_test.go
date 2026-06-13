// internal/config/ai_test.go
package config

import "testing"

func TestNormalizeInjectsBuiltinAIBackend(t *testing.T) {
	cfg := &Config{}
	Normalize(cfg)
	if !cfg.AI.Enabled {
		t.Fatal("AI 应默认启用")
	}
	if cfg.AI.Active != BuiltinAIBackendID {
		t.Fatalf("active 应为内置后端，得到 %q", cfg.AI.Active)
	}
	var found *AIBackend
	for i := range cfg.AI.Backends {
		if cfg.AI.Backends[i].ID == BuiltinAIBackendID {
			found = &cfg.AI.Backends[i]
		}
	}
	if found == nil {
		t.Fatal("应注入内置免费后端")
	}
	if found.Format != "openai" || found.Model != "openrouter/free" {
		t.Fatalf("内置后端字段不对: %+v", *found)
	}
	if found.APIKey == "" || found.BaseURL == "" {
		t.Fatal("内置后端应带 baseURL 和 key")
	}
}

func TestNormalizeBuiltinAIIsIdempotent(t *testing.T) {
	cfg := &Config{}
	Normalize(cfg)
	Normalize(cfg)
	n := 0
	for _, b := range cfg.AI.Backends {
		if b.ID == BuiltinAIBackendID {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("内置后端应只有 1 个，得到 %d", n)
	}
}

func TestNormalizeKeepsUserActive(t *testing.T) {
	cfg := &Config{AI: AIConfig{Active: "my-claude", Backends: []AIBackend{
		{ID: "my-claude", Format: "anthropic", BaseURL: "https://api.anthropic.com", Model: "claude-opus-4-8"},
	}}}
	Normalize(cfg)
	if cfg.AI.Active != "my-claude" {
		t.Fatalf("用户已设 active 不应被覆盖，得到 %q", cfg.AI.Active)
	}
}
