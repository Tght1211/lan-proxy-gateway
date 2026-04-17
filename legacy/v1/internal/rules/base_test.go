package rules

import (
	"strings"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestRenderIncludesCoreHighlights(t *testing.T) {
	cfg := config.DefaultConfig()

	rendered := Render(cfg)

	for _, want := range []string{
		"DOMAIN-SUFFIX,weixin.qq.com,DIRECT",
		"DOMAIN-SUFFIX,xiaohongshu.com,DIRECT",
		"DOMAIN-SUFFIX,douyin.com,DIRECT",
		"DOMAIN-SUFFIX,pvp.qq.com,DIRECT",
		"DOMAIN-SUFFIX,gdt.qq.com,REJECT",
		"DOMAIN-SUFFIX,openai.com,Proxy",
		"IP-CIDR,192.168.0.0/16,DIRECT",
		"MATCH,Proxy",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered rules missing %q\n%s", want, rendered)
		}
	}
}

func TestRenderRespectsTogglesAndExtras(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Rules.ChinaDirect = boolPtr(false)
	cfg.Rules.AdsReject = boolPtr(false)
	cfg.Rules.ExtraDirectRules = []string{"DOMAIN-SUFFIX,corp.example.com,DIRECT"}
	cfg.Rules.ExtraRejectRules = []string{"DOMAIN-SUFFIX,ads.example,REJECT"}

	rendered := Render(cfg)

	if strings.Contains(rendered, "DOMAIN-SUFFIX,weixin.qq.com,DIRECT") {
		t.Fatalf("china direct rules should be disabled\n%s", rendered)
	}
	if strings.Contains(rendered, "DOMAIN-SUFFIX,gdt.qq.com,REJECT") {
		t.Fatalf("ads reject rules should be disabled\n%s", rendered)
	}
	if !strings.Contains(rendered, "DOMAIN-SUFFIX,corp.example.com,DIRECT") {
		t.Fatalf("missing custom direct rule\n%s", rendered)
	}
	if !strings.Contains(rendered, "DOMAIN-SUFFIX,ads.example,REJECT") {
		t.Fatalf("missing custom reject rule\n%s", rendered)
	}
}

func boolPtr(v bool) *bool {
	return &v
}
