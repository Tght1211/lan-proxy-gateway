package app

import (
	"context"
	"sync"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/relay"
)

// HealthSnapshot is the advisory egress health view (no auto-flipping — v4
// leaves routing decisions to the upstream proxy and the user).
type HealthSnapshot struct {
	Healthy      bool         `json:"healthy"`
	LastError    string       `json:"last_error,omitempty"`
	CheckedAt    time.Time    `json:"checked_at,omitempty"`
	FailCount    int          `json:"fail_count"`
	LatencyMS    float64      `json:"latency_ms"`
	JitterMS     float64      `json:"jitter_ms"`
	Availability float64      `json:"availability"`
	History      []ProbePoint `json:"history"`
}

type ProbePoint struct {
	At        time.Time `json:"at"`
	LatencyMS float64   `json:"latency_ms"`
	OK        bool      `json:"ok"`
}

type healthState struct {
	mu        sync.RWMutex
	healthy   bool
	lastError string
	checkedAt time.Time
	failCount int
	history   []ProbePoint
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
	dialer, err := buildDialer(a.getCfg().Egress)
	if err != nil {
		a.health.record(err, 0)
		return
	}
	probeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	started := time.Now()
	err = relay.Probe(probeCtx, dialer, probeTarget)
	a.health.record(err, time.Since(started))
}
