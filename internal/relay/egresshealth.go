package relay

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// egressMonitor watches the upstream proxy port and records a global
// outage flag. When down, the relay forces all default-proxy traffic
// direct (reject rules still apply). The monitor also runs the per-host
// proxy-recovery probe loop.
type egressMonitor struct {
	health    *proxyHealth
	dialer    atomic.Value // Dialer for probes
	addr      atomic.Value // string "host:port"
	down      atomic.Bool
	since     time.Time
	actionsMu sync.Mutex
	actions   []EgressAction
	logger    atomic.Pointer[func(string, ...any)]
}

type EgressAction struct {
	At   time.Time `json:"at"`
	Text string    `json:"text"`
}

func newEgressMonitor(ph *proxyHealth) *egressMonitor {
	return &egressMonitor{health: ph}
}

func (m *egressMonitor) setProxy(addr string, d Dialer) {
	m.addr.Store(addr)
	if d != nil {
		m.dialer.Store(d)
	}
}

func (m *egressMonitor) isDown() bool { return m.down.Load() }

func (m *egressMonitor) snapshot() EgressSnapshot {
	var actions []EgressAction
	m.actionsMu.Lock()
	actions = append(actions, m.actions...)
	m.actionsMu.Unlock()
	return EgressSnapshot{
		ProxyDown: m.down.Load(),
		Since:     m.since,
		Actions:   actions,
		Alerts:    m.health.alertedHosts(time.Now()),
	}
}

func (m *egressMonitor) recordAction(text string) {
	m.actionsMu.Lock()
	defer m.actionsMu.Unlock()
	const max = 20
	m.actions = append(m.actions, EgressAction{At: time.Now(), Text: text})
	if len(m.actions) > max {
		m.actions = m.actions[len(m.actions)-max:]
	}
}

// probePort TCP-dials the configured proxy address to decide liveness.
func (m *egressMonitor) probePort() bool {
	addr, _ := m.addr.Load().(string)
	if addr == "" {
		return true
	}
	c, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// runLoop performs port probes every 30s and per-host recovery probes every
// 3 minutes until the context is cancelled.
func (m *egressMonitor) runLoop(ctx context.Context, probeHosts func() []string, onRecovered func(string)) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	probeTicker := time.NewTicker(probeInterval)
	defer probeTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ok := m.probePort()
			if ok && m.down.Load() {
				m.down.Store(false)
				m.recordAction("代理端口已恢复，恢复代理出口")
			} else if !ok && !m.down.Load() {
				m.down.Store(true)
				m.since = time.Now()
				m.recordAction("代理端口不可用，已将全部设备切换为直连")
			}
		case <-probeTicker.C:
			for _, host := range probeHosts() {
				m.probeHostRecovery(ctx, host, onRecovered)
			}
		}
	}
}

// probeHostRecovery dials host:443 through the proxy; on success the host
// returns to proxy egress (and any learned direct rule is removed by the
// onRecovered callback).
func (m *egressMonitor) probeHostRecovery(ctx context.Context, host string, onRecovered func(string)) {
	d, ok := m.dialer.Load().(Dialer)
	if !ok || d == nil {
		return
	}
	target := net.JoinHostPort(host, probeHost)
	dctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	c, err := d.DialContext(dctx, "tcp", target)
	if err != nil {
		m.health.markProbeFail(host, time.Now())
		return
	}
	c.Close()
	if m.health.markProbeOK(host, time.Now()) {
		m.health.markFast(host)
		if onRecovered != nil {
			onRecovered(host)
		}
	}
}

type EgressSnapshot struct {
	ProxyDown bool           `json:"proxy_down"`
	Since     time.Time      `json:"since,omitempty"`
	Actions   []EgressAction `json:"actions,omitempty"`
	Alerts    []string       `json:"alerts,omitempty"`
}
