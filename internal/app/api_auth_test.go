package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/httpproxy"
	"github.com/tght/lan-proxy-gateway/internal/relay"
)

func TestProxyCannotAccessManagementAPI(t *testing.T) {
	file := filepath.Join(t.TempDir(), "gateway.yaml")
	token, err := createAPIToken(file)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(filepath.Dir(file), "api-token"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("token permissions: %v, %v", info, err)
	}
	learner := newFallbackLearner(filepath.Join(t.TempDir(), "learning.json"), slog.Default())
	a := &App{Cfg: config.Default(), Paths: config.Paths{ConfigFile: file}}
	api := httptest.NewServer(newAPIServer(a, &daemonRuntime{apiToken: token, learner: learner}).http.Handler)
	defer api.Close()
	rs := relay.New(relay.Options{})
	rs.SetRouting("direct", relay.NewDirectDialer(0), nil, nil)
	proxy := httpproxy.New(rs.DialExplicit, httpproxy.Credentials{Username: "lan", Password: "password"})
	proxyHTTP := httptest.NewServer(proxy)
	defer proxyHTTP.Close()
	defer proxy.Close()
	const payload = `{"action":"ignore","host":"example.com"}`
	for _, path := range []string{"/api/stats", "/api/learning"} {
		t.Run("HTTP"+path, func(t *testing.T) {
			req := httptest.NewRequest("POST", api.URL+path, strings.NewReader(payload))
			if path == "/api/stats" {
				req.Method = "GET"
			}
			req.RemoteAddr = "192.168.1.20:12345"
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Proxy-Authorization", "Basic bGFuOnBhc3N3b3Jk")
			req.Header.Set("Authorization", "Bearer password") // proxy credential grants no API rights
			w := httptest.NewRecorder()
			proxy.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d: %s", w.Code, w.Body.String())
			}
		})
	}
	t.Run("CONNECT", func(t *testing.T) {
		conn, err := net.Dial("tcp", strings.TrimPrefix(proxyHTTP.URL, "http://"))
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		target := strings.TrimPrefix(api.URL, "http://")
		fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Authorization: Basic bGFuOnBhc3N3b3Jk\r\n\r\n", target, target)
		reader := bufio.NewReader(conn)
		resp, err := http.ReadResponse(reader, &http.Request{Method: "CONNECT"})
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("CONNECT: %d", resp.StatusCode)
		}
		req, _ := http.NewRequest("POST", api.URL+"/api/learning", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		if err := req.Write(conn); err != nil {
			t.Fatal(err)
		}
		resp, err = http.ReadResponse(reader, req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("tunneled status: %d", resp.StatusCode)
		}
	})
	if len(learner.Ignored()) != 0 {
		t.Fatal("proxy changed management state")
	}
	client := NewAPIClient(0, file)
	client.base = api.URL
	if err := client.Learning(context.Background(), "ignore", "example.com"); err != nil {
		t.Fatal(err)
	}
	if len(learner.Ignored()) != 1 {
		t.Fatal("authenticated local client failed")
	}
}

func TestAPIAuthenticationFailsClosed(t *testing.T) {
	handler := authenticateAPI("", http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("empty token authorized") }))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/stats", nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	file := filepath.Join(t.TempDir(), "gateway.yaml")
	first, err := createAPIToken(file)
	if err != nil {
		t.Fatal(err)
	}
	second, err := createAPIToken(file)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("token did not rotate")
	}
	c := NewAPIClient(0, file)
	srv := httptest.NewServer(authenticateAPI(second, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{}`) })))
	defer srv.Close()
	c.base = srv.URL
	if _, err := c.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(c.tokenPath); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Health(context.Background()); err == nil {
		t.Fatal("missing token accepted")
	}
}
