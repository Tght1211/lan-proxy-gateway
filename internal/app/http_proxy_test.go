package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/relay"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHTTPProxyConfigCredentials(t *testing.T) {
	cfg := config.Default()
	root := t.TempDir()
	// Keep reload requests inside the test, even if a real gateway is running.
	token, err := createAPIToken(filepath.Join(root, "gateway.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	api := httptest.NewServer(authenticateAPI(token, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/reload" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})))
	defer api.Close()
	cfg.Runtime.APIPort = api.Listener.Addr().(*net.TCPAddr).Port

	a := &App{Cfg: cfg, Paths: config.Paths{ConfigFile: filepath.Join(root, "gateway.yaml"), PIDFile: filepath.Join(root, "gateway.pid")}}
	password := "a-secret"
	update := HTTPProxyUpdate{Port: 17894, Auth: "basic", Username: "user", Password: &password}
	if err := a.SetHTTPProxy(update); err != nil {
		t.Fatal(err)
	}
	update.Password = nil
	update.Port = 17895
	if err := a.SetHTTPProxy(update); err != nil {
		t.Fatal(err)
	}
	saved, err := config.LoadFrom(a.Paths.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if saved.HTTPProxy.Password != password {
		t.Fatal("blank update discarded password")
	}
	info, _ := os.Stat(a.Paths.ConfigFile)
	if info.Mode().Perm() != 0600 {
		t.Fatal("credentials not private")
	}
	data, _ := json.Marshal(a.HTTPProxyStatus())
	if strings.Contains(string(data), password) {
		t.Fatal("status leaked password")
	}
	update.Auth = "none"
	if err := a.SetHTTPProxy(update); err != nil {
		t.Fatal(err)
	}
	if a.Cfg.HTTPProxy.Password != "" || a.Cfg.HTTPProxy.Username != "" {
		t.Fatal("no-auth mode retained credentials")
	}
}
func TestHTTPProxyConflictingPortPreservesListener(t *testing.T) {
	reserve, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := reserve.Addr().(*net.TCPAddr).Port
	reserve.Close()
	occupied, err := net.Listen("tcp4", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	rt := &daemonRuntime{relay: relay.New(relay.Options{})}
	defer rt.shutdown()
	cfg := config.Default()
	cfg.HTTPProxy = config.HTTPProxyConfig{Enabled: true, Port: port, Auth: "basic", Username: "user", Password: "secret"}
	if err := rt.syncHTTPProxy(cfg); err != nil {
		t.Fatal(err)
	}
	original := rt.httpProxy
	cfg.HTTPProxy.Port = occupied.Addr().(*net.TCPAddr).Port
	if err := rt.syncHTTPProxy(cfg); err == nil {
		t.Fatal("port conflict accepted")
	}
	if rt.httpProxy != original {
		t.Fatal("old listener replaced on failed bind")
	}
	proxyURL, _ := url.Parse("http://127.0.0.1:" + fmt.Sprint(port))
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: time.Second}
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://example.com", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 407 {
		t.Fatal("old authentication lost")
	}
	cfg.HTTPProxy.Enabled = false
	if err := rt.syncHTTPProxy(cfg); err != nil {
		t.Fatal(err)
	}
	if rt.httpProxy != nil {
		t.Fatal("disabled listener retained")
	}
}
