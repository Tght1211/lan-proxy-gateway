package source

import (
	"context"
	"io"
	"net"
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

func TestExternalProxyTestRequiresRealProxyResponse(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() != "http://www.gstatic.com/generate_204" {
			t.Fatalf("proxy received URL %q", r.URL.String())
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer proxy.Close()
	host, port := splitTestAddr(t, proxy.Listener.Addr().String())

	err := Test(context.Background(), config.SourceConfig{
		Type: config.SourceTypeExternal,
		External: config.ExternalProxy{
			Server: host,
			Port:   port,
			Kind:   "http",
		},
	})
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
}

func TestExternalProxyTestRejectsOpenPortThatIsNotWorkingProxy(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()
	host, port := splitTestAddr(t, ln.Addr().String())

	err = Test(context.Background(), config.SourceConfig{
		Type: config.SourceTypeExternal,
		External: config.ExternalProxy{
			Server: host,
			Port:   port,
			Kind:   "http",
		},
	})
	if err == nil {
		t.Fatal("expected open-but-broken proxy to fail")
	}
}

func splitTestAddr(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", addr, err)
	}
	port, err := net.LookupPort("tcp", portText)
	if err != nil {
		t.Fatalf("LookupPort(%q): %v", portText, err)
	}
	return host, port
}
