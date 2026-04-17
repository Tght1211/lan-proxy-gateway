package traffic

import (
	"strings"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestRenderRuleModeIncludesMatchProxy(t *testing.T) {
	cfg := config.Default().Traffic
	out := Render(cfg)
	if !strings.Contains(out, "MATCH,Proxy") {
		t.Fatalf("expected MATCH,Proxy at tail, got:\n%s", out)
	}
	if !strings.Contains(out, "GEOIP,CN") {
		t.Fatalf("expected china_direct GEOIP rule, got:\n%s", out)
	}
}

func TestRenderDirectModeSkipsProxyRules(t *testing.T) {
	cfg := config.Default().Traffic
	cfg.Mode = config.ModeDirect
	out := Render(cfg)
	if strings.Contains(out, "MATCH,Proxy") {
		t.Fatalf("direct mode should not emit MATCH,Proxy")
	}
	// Adblock still applies
	if !strings.Contains(out, "REJECT") {
		t.Fatalf("adblock should still be rendered in direct mode")
	}
}

func TestRenderGlobalModeLANStillDirect(t *testing.T) {
	cfg := config.Default().Traffic
	cfg.Mode = config.ModeGlobal
	out := Render(cfg)
	if !strings.Contains(out, "IP-CIDR,192.168.0.0/16") {
		t.Fatalf("LAN direct block missing in global mode:\n%s", out)
	}
	// Global mode does not emit MATCH — mihomo's mode=global handles fallback
	if strings.Contains(out, "MATCH,Proxy") {
		t.Fatalf("global mode should leave MATCH to mihomo mode=global")
	}
}

func TestRenderAdblockOff(t *testing.T) {
	cfg := config.Default().Traffic
	cfg.Adblock = false
	out := Render(cfg)
	if strings.Contains(out, "doubleclick") {
		t.Fatalf("adblock disabled but doubleclick still present:\n%s", out)
	}
}

func TestNoResolveVerdictPlacement(t *testing.T) {
	cfg := config.Default().Traffic
	out := Render(cfg)
	// Every IP-CIDR line must have "DIRECT,no-resolve" (not "no-resolve,DIRECT")
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "IP-CIDR") {
			continue
		}
		if strings.Contains(line, "no-resolve,DIRECT") {
			t.Errorf("wrong verdict placement: %s", line)
		}
	}
	// Sanity: look for a correctly-placed rule.
	if !strings.Contains(out, "IP-CIDR,192.168.0.0/16,DIRECT,no-resolve") {
		t.Errorf("expected canonical LAN rule not found in:\n%s", out)
	}
}

func TestRenderExtrasRespected(t *testing.T) {
	cfg := config.Default().Traffic
	cfg.Extras.Direct = []string{"DOMAIN-SUFFIX,corp.example.com"}
	cfg.Extras.Proxy = []string{"DOMAIN-SUFFIX,foo.bar,Proxy"} // already has verdict
	out := Render(cfg)
	if !strings.Contains(out, "DOMAIN-SUFFIX,corp.example.com,DIRECT") {
		t.Fatalf("extra direct not appended with DIRECT verdict:\n%s", out)
	}
	if !strings.Contains(out, "DOMAIN-SUFFIX,foo.bar,Proxy") {
		t.Fatalf("user-supplied verdict should be preserved:\n%s", out)
	}
	// Ensure no double-verdict mangling
	if strings.Contains(out, "Proxy,Proxy") {
		t.Fatalf("duplicate verdict appended:\n%s", out)
	}
}
