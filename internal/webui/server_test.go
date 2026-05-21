package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeController 是给 server_test 用的最小 Controller 实现：把每次调用记录到原子计数器，
// 让我们能断言 mux 路由确实命中了对应方法。
type fakeController struct {
	snap         Snapshot
	gatewayMode  string
	trafficMode  string
	adblock      atomic.Bool
	dnsEnabled   atomic.Bool
	tunEnabled   atomic.Bool
	sourceCalls  atomic.Int32
	reloadCalls  atomic.Int32
	restartCalls atomic.Int32
	gatewayCalls atomic.Int32
	failOn       string
}

func (f *fakeController) Snapshot() Snapshot { return f.snap }
func (f *fakeController) SetGatewayMode(_ context.Context, mode string) error {
	if f.failOn == "gateway" {
		return errors.New("simulated failure")
	}
	f.gatewayCalls.Add(1)
	f.gatewayMode = mode
	f.snap.GatewayMode = mode
	return nil
}
func (f *fakeController) SetTUNEnabled(_ context.Context, enabled bool) error {
	f.tunEnabled.Store(enabled)
	f.snap.TUNEnabled = enabled
	return nil
}
func (f *fakeController) SetTrafficMode(_ context.Context, mode string) error {
	f.trafficMode = mode
	f.snap.TrafficMode = mode
	return nil
}
func (f *fakeController) SetAdblock(_ context.Context, enabled bool) error {
	f.adblock.Store(enabled)
	f.snap.Adblock = enabled
	return nil
}
func (f *fakeController) SetDNSEnabled(_ context.Context, enabled bool) error {
	f.dnsEnabled.Store(enabled)
	f.snap.DNSEnabled = enabled
	return nil
}
func (f *fakeController) SetAutoGroups(_ context.Context, _ bool) error         { return nil }
func (f *fakeController) SetRulesets(_ context.Context, _ Rulesets) error       { return nil }
func (f *fakeController) SetCustomRules(_ context.Context, _ CustomRules) error { return nil }
func (f *fakeController) SetSource(_ context.Context, _ SourcePayload) error {
	f.sourceCalls.Add(1)
	return nil
}
func (f *fakeController) SetScript(_ context.Context, _ ScriptPayload) error { return nil }
func (f *fakeController) SetPorts(_ context.Context, _ Ports) error          { return nil }
func (f *fakeController) SetProxyService(_ context.Context, p ProxyServicePayload) error {
	f.snap.ProxyService = p
	return nil
}
func (f *fakeController) CheckUpdate(_ context.Context) (UpdateInfo, error) {
	return UpdateInfo{Current: "test", Latest: "test"}, nil
}
func (f *fakeController) RunUpdate(_ context.Context) error { return nil }
func (f *fakeController) Reload(_ context.Context) error    { f.reloadCalls.Add(1); return nil }
func (f *fakeController) Restart(_ context.Context) error   { f.restartCalls.Add(1); return nil }

func newTestServer(ctrl Controller) *httptest.Server {
	mux := http.NewServeMux()
	// 空 token = 测试模式，跳过鉴权
	(&Server{}).routes(mux, ctrl)
	return httptest.NewServer(mux)
}

// newAuthTestServer 用真实 token 起 Server，测试鉴权中间件本身。
func newAuthTestServer(ctrl Controller, token string) *httptest.Server {
	mux := http.NewServeMux()
	(&Server{token: token}).routes(mux, ctrl)
	return httptest.NewServer(mux)
}

func TestAPIStatus(t *testing.T) {
	fc := &fakeController{snap: Snapshot{
		Version: "test", Platform: "darwin", GatewayMode: "tun",
		MixedPort: 17890, MihomoAPIPort: 19090, LocalIP: "192.168.1.5",
	}}
	srv := newTestServer(fc)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/status")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status code = %d, want 200", resp.StatusCode)
	}
	var snap Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if snap.GatewayMode != "tun" {
		t.Errorf("GatewayMode = %q, want tun", snap.GatewayMode)
	}
	if snap.LocalIP != "192.168.1.5" {
		t.Errorf("LocalIP = %q, want 192.168.1.5", snap.LocalIP)
	}
}

func TestAPIGatewayModeSwitch(t *testing.T) {
	fc := &fakeController{snap: Snapshot{GatewayMode: "tun"}}
	srv := newTestServer(fc)
	defer srv.Close()

	body := bytes.NewReader([]byte(`{"mode":"forward"}`))
	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/config/gateway", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d body=%s", resp.StatusCode, b)
	}
	if fc.gatewayCalls.Load() != 1 {
		t.Errorf("SetGatewayMode 应被调一次，实际 %d", fc.gatewayCalls.Load())
	}
	if fc.gatewayMode != "forward" {
		t.Errorf("gatewayMode = %q, want forward", fc.gatewayMode)
	}
}

func TestAPIGatewayModeValidationRejectsBogus(t *testing.T) {
	fc := &fakeController{}
	srv := newTestServer(fc)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/config/gateway",
		bytes.NewReader([]byte(`{"mode":"turbo"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("应拒绝非法 mode，实际 status = %d", resp.StatusCode)
	}
	if fc.gatewayCalls.Load() != 0 {
		t.Errorf("非法 mode 不应触发 SetGatewayMode")
	}
}

func TestAPIControlReload(t *testing.T) {
	fc := &fakeController{}
	srv := newTestServer(fc)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/control/reload",
		bytes.NewReader([]byte(`{}`)))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("reload status = %d, want 204", resp.StatusCode)
	}
	if fc.reloadCalls.Load() != 1 {
		t.Errorf("Reload 应被调一次")
	}
}

func TestAPISourceValidation(t *testing.T) {
	fc := &fakeController{}
	srv := newTestServer(fc)
	defer srv.Close()

	// 缺 subscription.url
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/source",
		bytes.NewReader([]byte(`{"type":"subscription","subscription":{"url":""}}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("应拒绝空 subscription.url，实际 status = %d", resp.StatusCode)
	}
}

func TestStaticIndexServed(t *testing.T) {
	fc := &fakeController{}
	srv := newTestServer(fc)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("get /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "LAN Proxy Gateway") {
		t.Errorf("根路径未返回 index.html 内容；前 200 字节: %q", string(b[:min(len(b), 200)]))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ─────────── P0-1 鉴权测试 ───────────

func TestAPIRejectsRequestsWithoutToken(t *testing.T) {
	fc := &fakeController{}
	mux := http.NewServeMux()
	(&Server{token: "secret-token"}).routes(mux, fc)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.RemoteAddr = "192.168.12.113:53000"
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestAPIAcceptsLoopbackWithoutToken(t *testing.T) {
	fc := &fakeController{snap: Snapshot{GatewayMode: "tun"}}
	mux := http.NewServeMux()
	(&Server{token: "secret-token"}).routes(mux, fc)

	for _, remoteAddr := range []string{"127.0.0.1:53000", "[::1]:53000"} {
		req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
		req.RemoteAddr = remoteAddr
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("remoteAddr %s status = %d, want 200", remoteAddr, rr.Code)
		}
	}
}

func TestAPIAcceptsBearerToken(t *testing.T) {
	fc := &fakeController{snap: Snapshot{GatewayMode: "tun"}}
	srv := newAuthTestServer(fc, "secret-token")
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/status", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestAPIAcceptsXGatewayTokenHeader(t *testing.T) {
	fc := &fakeController{snap: Snapshot{GatewayMode: "tun"}}
	srv := newAuthTestServer(fc, "secret-token")
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/status", nil)
	req.Header.Set("X-Gateway-Token", "secret-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestAPIRejectsWrongToken(t *testing.T) {
	fc := &fakeController{}
	mux := http.NewServeMux()
	(&Server{token: "secret-token"}).routes(mux, fc)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.RemoteAddr = "192.168.12.113:53000"
	req.Header.Set("Authorization", "Bearer wrong-token")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token must be rejected; got %d", rr.Code)
	}
}

// /api/ping 是探活端点，不需要 token —— cmd/webui.go probeURL 和监控用得到。
func TestAPIPingIsPublic(t *testing.T) {
	fc := &fakeController{}
	srv := newAuthTestServer(fc, "secret-token")
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/ping")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ping should not require token; got %d", resp.StatusCode)
	}
}
