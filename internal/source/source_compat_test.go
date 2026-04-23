package source

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestNormalizeSubscriptionContent_StripsBOMAndManagedHeader(t *testing.T) {
	in := []byte("\ufeff#!MANAGED-CONFIG https://example.com/sub\n\nproxies:\n  - name: test\n")
	out, err := normalizeSubscriptionContent(in)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	s := string(out)
	if strings.HasPrefix(s, "\ufeff") {
		t.Fatalf("BOM not stripped: %q", s)
	}
	if !strings.Contains(s, "proxies:") {
		t.Fatalf("expected YAML kept, got:\n%s", s)
	}
}

func TestNormalizeSubscriptionContent_DecodesBase64YAML(t *testing.T) {
	yamlText := "proxies:\n  - name: test\nproxy-groups:\n  - name: Proxy\n    type: select\n    proxies: [test]\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(yamlText))
	out, err := normalizeSubscriptionContent([]byte(encoded))
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if got := string(out); !strings.Contains(got, "proxy-groups:") {
		t.Fatalf("base64 YAML not decoded:\n%s", got)
	}
}

func TestInlineUserYAML_AddsFallbackProxyGroupWhenMissing(t *testing.T) {
	frag, err := inlineUserYAML([]byte("proxies:\n  - name: hk\n    type: socks5\n    server: 1.2.3.4\n    port: 443\n"))
	if err != nil {
		t.Fatalf("inlineUserYAML: %v", err)
	}
	if !strings.Contains(frag.YAML, "proxy-groups:") || !strings.Contains(frag.YAML, "name: Proxy") {
		t.Fatalf("fallback Proxy group missing:\n%s", frag.YAML)
	}
}

func TestMaterializeSubscription_Base64Response(t *testing.T) {
	yamlText := "proxies:\n  - name: hk\n    type: socks5\n    server: 1.2.3.4\n    port: 443\nproxy-groups:\n  - name: grp\n    type: select\n    proxies: [hk]\nrules:\n  - MATCH,grp\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(base64.StdEncoding.EncodeToString([]byte(yamlText))))
	}))
	defer server.Close()

	frag, err := materializeSubscription(context.Background(), config.SubscriptionSource{URL: server.URL, Name: "test"}, t.TempDir(), "")
	if err != nil {
		t.Fatalf("materializeSubscription: %v", err)
	}
	if !strings.Contains(frag.YAML, "proxy-groups:") || len(frag.Rules) == 0 {
		t.Fatalf("unexpected fragment after base64 decode: %#v\nYAML:\n%s", frag, frag.YAML)
	}
}
