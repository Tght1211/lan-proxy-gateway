package source

import (
	"context"
	"strings"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestExternalHTTP(t *testing.T) {
	frag, err := Materialize(context.Background(), config.SourceConfig{
		Type: config.SourceTypeExternal,
		External: config.ExternalProxy{
			Name: "Upstream", Server: "127.0.0.1", Port: 7890, Kind: "http",
		},
	}, t.TempDir())
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if !strings.Contains(frag.YAML, "type: http") {
		t.Fatalf("expected http proxy type, got:\n%s", frag.YAML)
	}
	if !strings.Contains(frag.YAML, "name: Proxy") {
		t.Fatalf("expected Proxy group:\n%s", frag.YAML)
	}
	if !strings.Contains(frag.YAML, "127.0.0.1") {
		t.Fatalf("expected server IP:\n%s", frag.YAML)
	}
}

func TestExternalSOCKS5(t *testing.T) {
	frag, err := Materialize(context.Background(), config.SourceConfig{
		Type: config.SourceTypeExternal,
		External: config.ExternalProxy{
			Server: "127.0.0.1", Port: 1080, Kind: "socks5",
		},
	}, t.TempDir())
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if !strings.Contains(frag.YAML, "type: socks5") {
		t.Fatalf("expected socks5 proxy type, got:\n%s", frag.YAML)
	}
}

func TestNoneSourceStillProxiesThroughDirect(t *testing.T) {
	frag, err := Materialize(context.Background(), config.SourceConfig{
		Type: config.SourceTypeNone,
	}, t.TempDir())
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if !strings.Contains(frag.YAML, "Proxy") {
		t.Fatalf("Proxy group missing even in none mode")
	}
	if !strings.Contains(frag.YAML, "DIRECT") {
		t.Fatalf("expected DIRECT fallback:\n%s", frag.YAML)
	}
}

func TestRemoteSource(t *testing.T) {
	frag, err := Materialize(context.Background(), config.SourceConfig{
		Type: config.SourceTypeRemote,
		Remote: config.RemoteProxy{
			Name: "MyProxy", Kind: "socks5", Server: "1.2.3.4", Port: 443,
			Username: "u", Password: "p",
		},
	}, t.TempDir())
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if !strings.Contains(frag.YAML, "username: \"u\"") {
		t.Fatalf("expected credentials rendered:\n%s", frag.YAML)
	}
	if !strings.Contains(frag.YAML, "MyProxy") {
		t.Fatalf("expected custom name:\n%s", frag.YAML)
	}
}
