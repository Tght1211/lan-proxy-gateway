package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/dns"
	"github.com/tght/lan-proxy-gateway/internal/relay"
)

// StatsResponse is served by GET /api/stats and consumed by the console and
// native App. Bump SchemaVersion when an incompatible field changes.
type StatsResponse struct {
	SchemaVersion int            `json:"schema_version"`
	Egress        string         `json:"egress"`
	Proxy         string         `json:"proxy,omitempty"`
	UptimeSec     int64          `json:"uptime_sec"`
	Relay         relay.Snapshot `json:"relay"`
	DNS           *dns.Stats     `json:"dns,omitempty"`
	Health        HealthSnapshot `json:"health"`
	Fallback      *FallbackStats `json:"fallback,omitempty"`
}

// FallbackStats reports the proxy→direct fallback auto-learning state.
type FallbackStats struct {
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
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("POST /api/reload", s.handleReload)
	s.http = &http.Server{
		Addr:              net.JoinHostPort("127.0.0.1", fmt.Sprint(a.Cfg.Runtime.APIPort)),
		Handler:           mux,
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
	resp := StatsResponse{
		SchemaVersion: 3,
		Egress:        cfg.Egress.Mode,
		UptimeSec:     int64(time.Since(s.started).Seconds()),
		Relay:         s.rt.tracker.Snapshot(),
		Health:        s.app.Health(),
	}
	if cfg.Egress.Mode == "proxy" {
		p := cfg.Egress.Proxy
		resp.Proxy = fmt.Sprintf("%s %s:%d", p.Type, p.Host, p.Port)
	}
	if s.rt.dns != nil {
		st := s.rt.dns.Stats()
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
			Threshold:   fallbackLearnThreshold,
			WindowHours: int(fallbackLearnWindow / time.Hour),
			Candidates:  s.rt.learner.Snapshot(),
			Learned:     learned,
		}
	}
	writeJSON(w, resp)
}

func (s *apiServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.app.Health())
}

func (s *apiServer) handleReload(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadFrom(s.app.Paths.ConfigFile)
	if err != nil {
		http.Error(w, fmt.Sprintf("reload: %v", err), http.StatusBadRequest)
		return
	}
	s.rt.applyConfig(s.app, cfg)
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
	base string
	hc   *http.Client
}

func apiClient(apiPort int) *APIClient {
	return &APIClient{
		base: fmt.Sprintf("http://127.0.0.1:%d", apiPort),
		hc:   &http.Client{Timeout: 3 * time.Second},
	}
}

// NewAPIClient is the exported constructor for console/cmd use.
func NewAPIClient(apiPort int) *APIClient { return apiClient(apiPort) }

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
	resp, err := c.hc.Do(req)
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
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
