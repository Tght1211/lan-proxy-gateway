package engine

import (
	"context"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

// TestRenderedYAMLIsValid verifies that for every sensible combination of
// mode × source-type the emitted YAML parses as a YAML document. This is
// the closest we can get to "mihomo would accept this" without actually
// running mihomo in tests.
func TestRenderedYAMLIsValid(t *testing.T) {
	modes := []string{config.ModeRule, config.ModeGlobal, config.ModeDirect}
	sources := []config.SourceConfig{
		{Type: config.SourceTypeExternal, External: config.ExternalProxy{
			Server: "127.0.0.1", Port: 7890, Kind: "http",
		}},
		{Type: config.SourceTypeExternal, External: config.ExternalProxy{
			Server: "127.0.0.1", Port: 1080, Kind: "socks5",
		}},
		{Type: config.SourceTypeRemote, Remote: config.RemoteProxy{
			Name: "R", Kind: "socks5", Server: "1.2.3.4", Port: 443,
		}},
		{Type: config.SourceTypeNone},
	}

	for _, m := range modes {
		for _, s := range sources {
			cfg := config.Default()
			cfg.Traffic.Mode = m
			cfg.Source = s
			config.Normalize(cfg)

			data, err := Render(context.Background(), cfg, t.TempDir())
			if err != nil {
				t.Fatalf("render mode=%s src=%s: %v", m, s.Type, err)
			}

			var parsed map[string]interface{}
			if err := yaml.Unmarshal(data, &parsed); err != nil {
				t.Fatalf("mode=%s src=%s: invalid yaml: %v\n---\n%s", m, s.Type, err, data)
			}

			if _, ok := parsed["rules"]; !ok {
				t.Errorf("mode=%s src=%s: rules key missing", m, s.Type)
			}
			if _, ok := parsed["proxy-groups"]; !ok {
				t.Errorf("mode=%s src=%s: proxy-groups key missing", m, s.Type)
			}
			if v, _ := parsed["mode"].(string); v != m {
				t.Errorf("mode=%s src=%s: expected mode=%s in yaml, got %q", m, s.Type, m, v)
			}

			// Sanity: the proxy-group named "Proxy" must exist so traffic rules have a target.
			if !strings.Contains(string(data), "name: Proxy") {
				t.Errorf("mode=%s src=%s: missing Proxy group:\n%s", m, s.Type, data)
			}
		}
	}
}
