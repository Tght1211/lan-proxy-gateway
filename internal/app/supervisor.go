package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/relay"
)

// HealthSnapshot is the advisory egress health view (no auto-flipping — v4
// leaves routing decisions to the upstream proxy and the user).
type HealthSnapshot struct {
	Healthy        bool            `json:"healthy"`
	LastError      string          `json:"last_error,omitempty"`
	CheckedAt      time.Time       `json:"checked_at,omitempty"`
	FailCount      int             `json:"fail_count"`
	LatencyMS      float64         `json:"latency_ms"`
	JitterMS       float64         `json:"jitter_ms"`
	Availability   float64         `json:"availability"`
	History        []ProbePoint    `json:"history"`
	EgressIdentity *EgressIdentity `json:"egress_identity,omitempty"`
}

// EgressIdentity describes the public network observed through the currently
// configured egress. It is advisory telemetry and never affects routing.
type EgressIdentity struct {
	IP          string    `json:"ip"`
	CountryCode string    `json:"country_code,omitempty"`
	Region      string    `json:"region,omitempty"`
	City        string    `json:"city,omitempty"`
	ISP         string    `json:"isp,omitempty"`
	CheckedAt   time.Time `json:"checked_at"`
}

type ProbePoint struct {
	At        time.Time `json:"at"`
	LatencyMS float64   `json:"latency_ms"`
	OK        bool      `json:"ok"`
}

type healthState struct {
	mu              sync.RWMutex
	healthy         bool
	lastError       string
	checkedAt       time.Time
	failCount       int
	history         []ProbePoint
	identity        *EgressIdentity
	identityKey     string
	identityAttempt time.Time
}

func (h *healthState) record(err error, latency time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkedAt = time.Now()
	point := ProbePoint{At: h.checkedAt, LatencyMS: float64(latency.Microseconds()) / 1000, OK: err == nil}
	h.history = append(h.history, point)
	if len(h.history) > 180 {
		h.history = append([]ProbePoint(nil), h.history[len(h.history)-180:]...)
	}
	if err == nil {
		h.healthy = true
		h.lastError = ""
		h.failCount = 0
		return
	}
	h.healthy = false
	h.lastError = err.Error()
	h.failCount++
}

func (h *healthState) snapshot() HealthSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := HealthSnapshot{
		Healthy:   h.healthy,
		LastError: h.lastError,
		CheckedAt: h.checkedAt,
		FailCount: h.failCount,
		History:   append([]ProbePoint{}, h.history...),
	}
	if h.identity != nil {
		copy := *h.identity
		out.EgressIdentity = &copy
	}
	var successes int
	var latencySum, jitterSum float64
	var previous float64
	for _, point := range h.history {
		if !point.OK {
			continue
		}
		successes++
		latencySum += point.LatencyMS
		if previous > 0 {
			delta := point.LatencyMS - previous
			if delta < 0 {
				delta = -delta
			}
			jitterSum += delta
		}
		previous = point.LatencyMS
	}
	if len(h.history) > 0 {
		out.Availability = float64(successes) / float64(len(h.history)) * 100
	}
	if successes > 0 {
		out.LatencyMS = latencySum / float64(successes)
	}
	if successes > 1 {
		out.JitterMS = jitterSum / float64(successes-1)
	}
	return out
}

func (h *healthState) identityDue(now time.Time, key string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.identityKey != key {
		return true
	}
	interval := 30 * time.Minute
	if h.identity == nil {
		interval = 5 * time.Minute
	}
	return now.Sub(h.identityAttempt) >= interval
}

func (h *healthState) recordIdentity(identity *EgressIdentity, key string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.identityAttempt = time.Now()
	h.identityKey = key
	if identity != nil {
		h.identity = identity
	}
}

// Health returns the current advisory health snapshot. Before the supervisor
// has run (e.g. in a fresh console process) it reports healthy-by-default.
func (a *App) Health() HealthSnapshot {
	if a.health == nil {
		return HealthSnapshot{Healthy: true}
	}
	return a.health.snapshot()
}

// StartSupervisor lazily starts the egress probe loop.
func (a *App) StartSupervisor(ctx context.Context) {
	a.supervisorOnce.Do(func() {
		if a.health == nil {
			a.health = &healthState{healthy: true}
		}
		go a.supervise(ctx)
	})
}

func (a *App) supervise(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	a.probeOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.probeOnce(ctx)
		}
	}
}

func (a *App) probeOnce(ctx context.Context) {
	egress := a.getCfg().Egress
	dialer, err := buildDialer(egress)
	if err != nil {
		a.health.record(err, 0)
		return
	}
	probeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	started := time.Now()
	err = relay.Probe(probeCtx, dialer, probeTarget)
	a.health.record(err, time.Since(started))
	if err != nil {
		return
	}
	key := fmt.Sprintf("%s|%s|%s|%d", egress.Mode, egress.Proxy.Type, egress.Proxy.Host, egress.Proxy.Port)
	if !a.health.identityDue(time.Now(), key) {
		return
	}
	identityCtx, identityCancel := context.WithTimeout(ctx, 8*time.Second)
	defer identityCancel()
	identity, identityErr := lookupEgressIdentity(identityCtx, dialer)
	if identityErr != nil {
		a.health.recordIdentity(nil, key)
		return
	}
	a.health.recordIdentity(identity, key)
}

func lookupEgressIdentity(ctx context.Context, dialer relay.Dialer) (*EgressIdentity, error) {
	transport := &http.Transport{DialContext: dialer.DialContext, ForceAttemptHTTP2: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 8 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://ipwho.is/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "LAN-Proxy-Gateway/4")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("出口信息服务返回 %s", resp.Status)
	}
	var payload struct {
		IP          string `json:"ip"`
		Success     bool   `json:"success"`
		CountryCode string `json:"country_code"`
		Region      string `json:"region"`
		City        string `json:"city"`
		Connection  struct {
			ISP string `json:"isp"`
		} `json:"connection"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if !payload.Success || payload.IP == "" {
		return nil, fmt.Errorf("出口信息服务未返回有效地址")
	}
	return &EgressIdentity{
		IP: payload.IP, CountryCode: payload.CountryCode, Region: payload.Region,
		City: payload.City, ISP: payload.Connection.ISP, CheckedAt: time.Now(),
	}, nil
}
