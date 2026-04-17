package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestRenderExternalSource(t *testing.T) {
	cfg := config.Default()
	cfg.Source.Type = config.SourceTypeExternal
	cfg.Source.External.Server = "127.0.0.1"
	cfg.Source.External.Port = 7890
	cfg.Source.External.Kind = "http"

	out, err := Render(context.Background(), cfg, t.TempDir())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "mixed-port: 7890") {
		t.Errorf("missing mixed port substitution")
	}
	if !strings.Contains(s, "mode: rule") {
		t.Errorf("mihomo mode not substituted")
	}
	if !strings.Contains(s, "MATCH,Proxy") {
		t.Errorf("rules not rendered")
	}
	if !strings.Contains(s, "127.0.0.1") {
		t.Errorf("upstream server not rendered")
	}
	if !strings.Contains(s, "tun:") || !strings.Contains(s, "enable: true") {
		t.Errorf("tun block not rendered:\n%s", s)
	}
}

func TestRenderDirectMode(t *testing.T) {
	cfg := config.Default()
	cfg.Source.Type = config.SourceTypeNone
	cfg.Traffic.Mode = config.ModeDirect

	out, err := Render(context.Background(), cfg, t.TempDir())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "mode: direct") {
		t.Errorf("expected mode: direct")
	}
	if strings.Contains(s, "MATCH,Proxy") {
		t.Errorf("direct mode should not emit MATCH,Proxy: %s", s)
	}
}

func TestRenderTUNOff(t *testing.T) {
	cfg := config.Default()
	cfg.Source.Type = config.SourceTypeNone
	cfg.Gateway.TUN.Enabled = false
	out, err := Render(context.Background(), cfg, t.TempDir())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(out), "enable: false") {
		t.Errorf("TUN off not rendered")
	}
}
