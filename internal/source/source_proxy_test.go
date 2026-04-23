package source

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestMaterializeSubscription_UsesConfiguredProxy(t *testing.T) {
	var seenProxy atomic.Bool
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenProxy.Store(true)
		if got, want := r.URL.String(), "http://example.invalid/sub"; got != want {
			t.Fatalf("proxy received URL %q, want %q", got, want)
		}
		_, _ = io.WriteString(w, `proxies:
  - name: hk
    type: http
    server: 1.2.3.4
    port: 80
proxy-groups:
  - name: Proxy
    type: select
    proxies: [hk]
`)
	}))
	defer proxy.Close()

	frag, err := MaterializeWithOptions(context.Background(), config.SourceConfig{
		Type: config.SourceTypeSubscription,
		Subscription: config.SubscriptionSource{
			URL:  "http://example.invalid/sub",
			Name: "test",
		},
	}, t.TempDir(), MaterializeOptions{
		SubscriptionProxyURL: proxy.URL,
	})
	if err != nil {
		t.Fatalf("MaterializeWithOptions() error = %v", err)
	}
	if !seenProxy.Load() {
		t.Fatal("expected subscription fetch to go through proxy")
	}
	if frag.Summary != "订阅 · test" {
		t.Fatalf("unexpected summary %q", frag.Summary)
	}
}
