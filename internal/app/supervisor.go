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
	Healthy   bool      `json:"healthy"`
	LastError string    `json:"last_error,omitempty"`
	CheckedAt time.Time `json:"checked_at,omitempty"`
	FailCount int       `json:"fail_count"`
}

type healthState struct {
	mu        sync.RWMutex
	healthy   bool
	lastError string
	checkedAt time.Time
	failCount int
}

func (h *healthState) record(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkedAt = time.Now()
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
	return HealthSnapshot{
		Healthy:   h.healthy,
		LastError: h.lastError,
		CheckedAt: h.checkedAt,
		FailCount: h.failCount,
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

// StartSupervisor lazily starts the 30s egress probe loop.
func (a *App) StartSupervisor(ctx context.Context) {
	a.supervisorOnce.Do(func() {
		if a.health == nil {
			a.health = &healthState{healthy: true}
		}
		go a.supervise(ctx)
	})
}

func (a *App) supervise(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
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
		a.health.record(err)
		return
	}
	probeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	a.health.record(relay.Probe(probeCtx, dialer, probeTarget))
}
