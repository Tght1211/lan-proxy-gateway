package cmd

import (
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestParseOnOff(t *testing.T) {
	on := []string{"on", "On", "TRUE", "1", "yes", "enable", "enabled"}
	off := []string{"off", "false", "0", "no", "disable", "disabled"}
	for _, s := range on {
		v, err := parseOnOff(s)
		if err != nil || !v {
			t.Fatalf("%q 应解析为 on，得到 v=%v err=%v", s, v, err)
		}
	}
	for _, s := range off {
		v, err := parseOnOff(s)
		if err != nil || v {
			t.Fatalf("%q 应解析为 off，得到 v=%v err=%v", s, v, err)
		}
	}
	if _, err := parseOnOff("maybe"); err == nil {
		t.Fatal("非法开关值应报错")
	}
}

func TestBuildConfigView(t *testing.T) {
	cfg := config.Default()
	config.Normalize(cfg)
	cfg.Source.Type = config.SourceTypeSubscription
	cfg.Source.Subscription.URL = "https://e.com/sub"
	cfg.Traffic.Extras.Proxy = []string{"DOMAIN-SUFFIX,openai.com"}
	a := &app.App{Cfg: cfg}

	v := buildConfigView(a)
	if v.Source.Type != "subscription" || v.Source.URL != "https://e.com/sub" {
		t.Fatalf("source 视图不对: %+v", v.Source)
	}
	if v.Mode != cfg.Traffic.Mode {
		t.Fatalf("mode 不对: %s", v.Mode)
	}
	if len(v.Rules.Proxy) != 1 || v.Rules.Proxy[0] != "DOMAIN-SUFFIX,openai.com" {
		t.Fatalf("proxy 规则不对: %+v", v.Rules.Proxy)
	}
	if v.GatewayMode == "" {
		t.Fatal("网关模式不应为空（应回退 tun）")
	}
}
