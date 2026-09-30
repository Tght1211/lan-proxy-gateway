package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"sort"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/dns"
	"github.com/tght/lan-proxy-gateway/internal/hotspot"
	"github.com/tght/lan-proxy-gateway/internal/relay"
)

// StatsResponse is served by GET /api/stats and consumed by the console and
// native App. Bump SchemaVersion when an incompatible field changes.
// ComponentHealth reports the running status of a core sub-service.
type ComponentHealth struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
	Crashes int    `json:"crashes"`
}

type StatsResponse struct {
	Hotspot        *hotspot.Status              `json:"hotspot,omitempty"`
	UsageHistory   []relay.DailyUsage           `json:"usage_history"`
	HTTPProxy      HTTPProxyStatus              `json:"http_proxy"`
	SchemaVersion  int                          `json:"schema_version"`
	Egress         string                       `json:"egress"`
	Proxy          string                       `json:"proxy,omitempty"`
	UptimeSec      int64                        `json:"uptime_sec"`
	Relay          relay.Snapshot               `json:"relay"`
	UDPRelay       *relay.UDPRelayStats         `json:"udp_relay,omitempty"`
	DNS            *dns.Stats                   `json:"dns,omitempty"`
	Health         HealthSnapshot               `json:"health"`
	Fallback       *FallbackStats               `json:"fallback,omitempty"`
	EgressHealth   *egressHealthJSON            `json:"egress_health,omitempty"`
	DeviceAdaptive relay.DeviceAdaptiveSnapshot `json:"device_adaptive"`
	Components     []ComponentHealth            `json:"components,omitempty"`
}

// egressHealthJSON exposes the global outage state and post-direct failures.
type egressHealthJSON struct {
	ProxyDown      bool                 `json:"proxy_down"`
	Since          *time.Time           `json:"since,omitempty"`
	Actions        []relay.EgressAction `json:"actions,omitempty"`
	Alerts         []string             `json:"alerts,omitempty"`
	DirectFailures []directFailure      `json:"direct_failures,omitempty"`
	FailureStats   []egressFailureStat  `json:"failure_stats,omitempty"`
	AlertThreshold int                  `json:"alert_threshold"`
	AlertWindowSec int                  `json:"alert_window_sec"`
	StatsWindowSec int                  `json:"stats_window_sec"`
}

type directFailure struct {
	Device string    `json:"device"`
	Host   string    `json:"host"`
	Reason string    `json:"reason"`
	Count  int       `json:"count"`
	LastAt time.Time `json:"last_at"`
}

// egressFailureStat is one (device, host) failure aggregate over the stats
// window, including entries below the alert threshold or suppressed by a
// recent success, so the dashboard can show why something did NOT alert.
type egressFailureStat struct {
	Device     string    `json:"device"`
	Host       string    `json:"host"`
	Reason     string    `json:"reason"`
	Count      int       `json:"count"`
	LastAt     time.Time `json:"last_at"`
	Suppressed bool      `json:"suppressed"` // a recent success indicates the target actually works
	Alerting   bool      `json:"alerting"`   // included in direct_failures
}

// FallbackStats reports automatic proxy learning and any retained legacy rules.
type FallbackStats struct {
	Settings    LearningSettings     `json:"settings"`
	Strategy    string               `json:"strategy"`
	Ignored     []string             `json:"ignored"`
	Threshold   int                  `json:"threshold"`
	WindowHours int                  `json:"window_hours"`
	Candidates  []FallbackCandidate  `json:"candidates"`
	Learned     []config.RoutingRule `json:"learned"`
}

// apiServer is the daemon's loopback-only status API.
type apiServer struct {
	app     *App
	rt      *daemonRuntime
	http    *http.Server
	started time.Time
}

func newAPIServer(a *App, rt *daemonRuntime) *apiServer {
	s := &apiServer{app: a, rt: rt, started: time.Now()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/stats", s.handleStats)
	mux.HandleFunc("POST /api/learning", s.handleLearning)
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("POST /api/reload", s.handleReload)
	mux.HandleFunc("GET /api/devices", s.handleDevices)
	mux.HandleFunc("GET /api/nat-diag", s.handleNATDiag)
	s.http = &http.Server{
		Addr:              net.JoinHostPort("127.0.0.1", fmt.Sprint(a.Cfg.Runtime.APIPort)),
		Handler:           authenticateAPI(rt.apiToken, mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s
}

// ListenAndServe blocks until ctx is cancelled or the listener fails.
func (s *apiServer) ListenAndServe(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.http.Shutdown(shutCtx)
		return nil
	case err := <-errCh:
		return err
	}
}

func (s *apiServer) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.http.Shutdown(ctx)
}

func (s *apiServer) handleStats(w http.ResponseWriter, r *http.Request) {
	cfg := s.app.getCfg()
	relaySrv, udpRelay, dnsSrv := s.rt.services()
	resp := StatsResponse{
		UsageHistory:   s.rt.tracker.UsageHistory(),
		HTTPProxy:      s.app.HTTPProxyStatus(),
		SchemaVersion:  3,
		Egress:         cfg.Egress.Mode,
		UptimeSec:      int64(time.Since(s.started).Seconds()),
		Relay:          s.rt.tracker.Snapshot(),
		Health:         s.app.Health(),
		DeviceAdaptive: relaySrv.DeviceAdaptiveHealth(),
	}
	if cfg.Gateway.AccessMode == "hotspot" {
		st := s.app.Gateway.HotspotStatus()
		st.Enabled = cfg.Gateway.Enabled
		resp.Hotspot = &st
	}
	if udpRelay != nil {
		st := udpRelay.Stats()
		resp.UDPRelay = &st
	}
	if cfg.Egress.Mode == "proxy" {
		p := cfg.Egress.Proxy
		resp.Proxy = fmt.Sprintf("%s %s:%d", p.Type, p.Host, p.Port)
	}
	if dnsSrv != nil {
		st := dnsSrv.Stats()
		resp.DNS = &st
	}
	if s.rt.learner != nil {
		learned := make([]config.RoutingRule, 0)
		for _, r := range cfg.Routing.Rules {
			if r.Learned {
				learned = append(learned, r)
			}
		}
		resp.Fallback = &FallbackStats{
			Strategy:    "direct-first",
			Settings:    s.rt.learner.Settings(),
			Ignored:     s.rt.learner.Ignored(),
			Threshold:   s.rt.learner.Settings().Confirmations,
			WindowHours: int(fallbackLearnWindow / time.Hour),
			Candidates:  s.rt.learner.Snapshot(),
			Learned:     learned,
		}
	}
	eh := relaySrv.EgressHealth()
	resp.EgressHealth = buildEgressHealthJSON(eh, resp.Relay.Recent, resp.Relay.Active)
	resp.Components = s.rt.componentHealth()
	writeJSON(w, resp)
}

const (
	// A (device, host) pair only alerts after this many direct dial failures
	// inside egressAlertWindow with zero successes — one-off timeouts from
	// telemetry/PCDN domains used to spam the banner.
	egressAlertThreshold = 3
	egressAlertWindow    = 10 * time.Minute
	egressStatsWindow    = time.Hour
)

// buildEgressHealthJSON merges the relay monitor snapshot with the recent
// connection log. Failures are aggregated per (device, host); only pairs at
// or above the alert threshold with no recent success raise DirectFailures,
// while FailureStats carries the full aggregate for the dashboard.
func buildEgressHealthJSON(snap relay.EgressSnapshot, recent, active []relay.ConnInfo) *egressHealthJSON {
	out := &egressHealthJSON{
		ProxyDown:      snap.ProxyDown,
		Actions:        snap.Actions,
		Alerts:         snap.Alerts,
		AlertThreshold: egressAlertThreshold,
		AlertWindowSec: int(egressAlertWindow / time.Second),
		StatsWindowSec: int(egressStatsWindow / time.Second),
	}
	if !snap.Since.IsZero() {
		since := snap.Since
		out.Since = &since
	}

	now := time.Now()
	statsCutoff := now.Add(-egressStatsWindow)
	alertCutoff := now.Add(-egressAlertWindow)

	type pairKey struct{ device, host string }
	type pairAgg struct {
		count       int // failures in the stats window
		alertCount  int // failures in the alert window
		lastFail    time.Time
		lastSuccess time.Time
		reason      string
	}
	pairs := map[pairKey]*pairAgg{}
	get := func(key pairKey) *pairAgg {
		a := pairs[key]
		if a == nil {
			a = &pairAgg{}
			pairs[key] = a
		}
		return a
	}

	for _, c := range recent {
		if c.Rejected || c.ViaProxy || c.SrcIP == "" || c.DstHost == "" {
			continue
		}
		key := pairKey{c.SrcIP, c.DstHost}
		if c.Status == "dial_failed" && c.Failure != "" {
			if c.EndedAt == nil || c.EndedAt.Before(statsCutoff) {
				continue
			}
			a := get(key)
			a.count++
			if c.EndedAt.After(a.lastFail) {
				a.lastFail = *c.EndedAt
				a.reason = c.Failure
			}
			if c.EndedAt.After(alertCutoff) {
				a.alertCount++
			}
			continue
		}
		if c.Status == "" {
			// A completed direct connection that dialed fine counts as success.
			at := c.StartedAt
			if c.EndedAt != nil {
				at = *c.EndedAt
			}
			if at.Before(statsCutoff) {
				continue
			}
			a := get(key)
			if at.After(a.lastSuccess) {
				a.lastSuccess = at
			}
		}
	}
	// Live direct connections prove the target is reachable right now.
	for _, c := range active {
		if c.ViaProxy || c.SrcIP == "" || c.DstHost == "" {
			continue
		}
		a := get(pairKey{c.SrcIP, c.DstHost})
		if now.After(a.lastSuccess) {
			a.lastSuccess = now
		}
	}

	for key, a := range pairs {
		if a.count == 0 {
			continue
		}
		suppressed := a.lastSuccess.After(a.lastFail)
		alerting := !suppressed && a.alertCount >= egressAlertThreshold
		out.FailureStats = append(out.FailureStats, egressFailureStat{
			Device: key.device, Host: key.host, Reason: a.reason,
			Count: a.count, LastAt: a.lastFail,
			Suppressed: suppressed, Alerting: alerting,
		})
		if alerting {
			out.DirectFailures = append(out.DirectFailures, directFailure{
				Device: key.device, Host: key.host, Reason: a.reason,
				Count: a.alertCount, LastAt: a.lastFail,
			})
		}
	}
	sort.Slice(out.FailureStats, func(i, j int) bool {
		return out.FailureStats[i].LastAt.After(out.FailureStats[j].LastAt)
	})
	sort.Slice(out.DirectFailures, func(i, j int) bool {
		return out.DirectFailures[i].LastAt.After(out.DirectFailures[j].LastAt)
	})
	return out
}

func (s *apiServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.app.Health())
}

func (s *apiServer) handleDevices(w http.ResponseWriter, r *http.Request) {
	info, err := s.app.Gateway.DeviceOnboardingInfo()
	if err != nil {
		http.Error(w, fmt.Sprintf("device scan: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, info)
}

func (s *apiServer) handleNATDiag(w http.ResponseWriter, r *http.Request) {
	diag, err := s.app.Gateway.DiagnoseNAT(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("NAT diagnosis: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, diag)
}

func (s *apiServer) handleReload(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadFrom(s.app.Paths.ConfigFile)
	if err != nil {
		http.Error(w, fmt.Sprintf("reload: %v", err), http.StatusBadRequest)
		return
	}
	if err := s.rt.applyConfig(s.app, cfg); err != nil {
		http.Error(w, fmt.Sprintf("reload: %v", err), http.StatusConflict)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// ---------- client side (console / cmd) ----------

// apiClient talks to the daemon's loopback API from other processes.
type APIClient struct {
	base      string
	tokenPath string
	hc        *http.Client
}

func apiClient(apiPort int, configFile ...string) *APIClient {
	paths, _ := config.ResolvePaths()
	file := paths.ConfigFile
	if len(configFile) > 0 {
		file = configFile[0]
	}
	return &APIClient{
		tokenPath: filepath.Join(filepath.Dir(file), "api-token"),
		base:      fmt.Sprintf("http://127.0.0.1:%d", apiPort),
		hc:        &http.Client{Timeout: 3 * time.Second},
	}
}

// NewAPIClient is the exported constructor for console/cmd use.
func NewAPIClient(apiPort int, configFile ...string) *APIClient {
	return apiClient(apiPort, configFile...)
}

func (c *APIClient) Stats(ctx context.Context) (*StatsResponse, error) {
	var out StatsResponse
	if err := c.get(ctx, "/api/stats", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *APIClient) Health(ctx context.Context) (*HealthSnapshot, error) {
	var out HealthSnapshot
	if err := c.get(ctx, "/api/health", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Reload pokes the daemon to re-read the config file.
func (c *APIClient) Reload(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/api/reload", nil)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("reload 失败: %s", body)
	}
	return nil
}

func (c *APIClient) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (s *apiServer) handleLearning(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != "" || r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "JSON requests only", http.StatusForbidden)
		return
	}
	var input struct {
		Action string `json:"action"`
		Host   string `json:"host"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if s.rt.learner == nil {
		http.Error(w, "核心尚未就绪", http.StatusServiceUnavailable)
		return
	}
	if err := s.app.applyLearningAction(s.rt.learner, input.Action, input.Host); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if input.Action == "accept" || input.Action == "undo" {
		// Only routing changed. Serialize with automatic learning and apply the
		// newest config without writing an older snapshot back into the app.
		s.rt.learner.mu.Lock()
		s.rt.applyRouting(s.app.getCfg())
		s.rt.learner.mu.Unlock()
	}
	writeJSON(w, map[string]string{"status": "ok"})
}
