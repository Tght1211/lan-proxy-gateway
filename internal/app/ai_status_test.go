package app

import (
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestStatusRedactsAIKey(t *testing.T) {
	cfg := config.Default()
	config.Normalize(cfg)
	a := &App{Cfg: cfg, Plat: nil}
	// Status() 会调用 a.Plat / a.Gateway，这里只测 AI 字段，构造最小 App。
	st := a.aiStatus()
	if len(st) == 0 {
		t.Fatal("应至少有内置后端")
	}
	for _, b := range st {
		if b.HasKey && b.APIKeyMasked == "" {
			t.Fatal("有 key 的后端应给出掩码而非空")
		}
		if b.APIKeyMasked != "" && b.APIKeyMasked[:3] != "sk-" && b.APIKeyMasked != "***" {
			// 掩码必须不是明文 key
			if len(b.APIKeyMasked) > 6 {
				t.Fatalf("掩码疑似泄漏明文: %q", b.APIKeyMasked)
			}
		}
	}
}
