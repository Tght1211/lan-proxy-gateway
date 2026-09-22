package httpproxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPACBootstrapAndAuthentication(t *testing.T) {
	s := New(func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("PAC must not dial upstream")
		return nil, fmt.Errorf("unexpected dial")
	}, Credentials{"user", "secret"})
	for _, host := range []string{"192.168.12.100:17890", "vipvip.xiaomiqiu.com:31991", "[::1]:17890"} {
		r := httptest.NewRequest("GET", "/proxy.pac", nil)
		r.Host = host
		r.RemoteAddr = "192.168.12.20:12345"
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "PROXY "+host) || strings.Contains(w.Body.String(), "secret") || strings.Contains(w.Body.String(), "; DIRECT") {
			t.Fatalf("unexpected PAC: %d %s", w.Code, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Header().Get("Content-Type"), "proxy-autoconfig") {
			t.Fatal("PAC headers missing")
		}
	}
	r := httptest.NewRequest("GET", "http://example.com/proxy.pac", nil)
	r.RemoteAddr = "192.168.12.20:12345"
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != http.StatusProxyAuthRequired {
		t.Fatalf("forward request bypassed auth: %d", w.Code)
	}
	r = httptest.NewRequest("CONNECT", "http://example.com", nil)
	r.Host = "example.com:443"
	r.RemoteAddr = "192.168.12.20:12345"
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 407 {
		t.Fatal("CONNECT bypassed authentication")
	}
}

func TestPACRejectsUnsafeHostsAndUnsupportedMethods(t *testing.T) {
	s := New(nil, Credentials{})
	for _, host := range []string{"bad\"host:17890", "user:password@host:80", "host:0", "host:65536", "host:bad", "host/path:80"} {
		r := httptest.NewRequest("GET", "/proxy.pac", nil)
		r.Host = host
		r.RemoteAddr = "127.0.0.1:1"
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatalf("accepted %q: %d", host, w.Code)
		}
	}
	for _, method := range []string{"HEAD", "POST"} {
		r := httptest.NewRequest(method, "/proxy.pac", nil)
		r.Host = "192.168.12.100:17890"
		r.RemoteAddr = "127.0.0.1:1"
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if method == "HEAD" && (w.Code != 200 || w.Body.Len() != 0) {
			t.Fatal("invalid HEAD")
		}
		if method == "POST" && w.Code != 405 {
			t.Fatal("accepted POST")
		}
	}
}
