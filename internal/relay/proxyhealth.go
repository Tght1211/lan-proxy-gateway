package relay

import (
	"sync"
	"time"
)

// Per-host egress health state machine.
//
//   normal(代理) --fail×threshold--> directTest(30min, 直连优先/代理兜底)
//   directTest --directOK--> directHold(固定直连, 每3min探测代理)
//   directHold --probeOK--> normal (或 fastDirect)
//   directHold --probeFail×3--> directHold+alert (固定直连+告警)
//
//   fastDirect: 代理恢复后的一次性快速域名——之后代理失败 1 次立即回 directTest；
//   探测失败 1 次也立即回 directTest。阈值降为 1，响应更快。
const (
	proxyFailThreshold  = 3
	fastFailThreshold   = 1
	probeFailThreshold  = 3
	proxyFailWindow     = 10 * time.Minute
	directTestDuration  = 30 * time.Minute
	probeInterval       = 3 * time.Minute
	probeHost           = "443"
	proxyHealthMaxHosts = 4096
)

type hostState int

const (
	stateNormal hostState = iota
	stateDirectTest
	stateDirectHold
	stateFastDirect
)

type hostHealth struct {
	state      hostState
	fails      int
	firstAt    time.Time
	testUntil  time.Time
	probeFails int
	alertedAt  time.Time
	fast       bool
	lastProxyOK time.Time
}

type proxyHealth struct {
	mu    sync.Mutex
	hosts map[string]*hostHealth
}

func newProxyHealth() *proxyHealth {
	return &proxyHealth{hosts: map[string]*hostHealth{}}
}

func (p *proxyHealth) threshold(h *hostHealth) int {
	if h.fast {
		return fastFailThreshold
	}
	return proxyFailThreshold
}

// recordFailure counts one proxy failure and reports the host's new state.
func (p *proxyHealth) recordFailure(host string, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	h := p.hosts[host]
	if h == nil {
		if len(p.hosts) >= proxyHealthMaxHosts {
			p.pruneLocked(now)
		}
		h = &hostHealth{state: stateNormal}
		p.hosts[host] = h
	}
	if h.state == stateDirectHold || h.state == stateDirectTest {
		// already direct; nothing to count
		return
	}
	if h.fails == 0 || now.Sub(h.firstAt) > proxyFailWindow {
		h.fails = 0
		h.firstAt = now
	}
	h.fails++
	if h.fails >= p.threshold(h) {
		h.state = stateDirectTest
		h.testUntil = now.Add(directTestDuration)
		h.fails = 0
	}
}

// recordProxyOK clears the failure streak; if the host was in a direct state
// it signals the caller to demote (handled by the probe loop).
func (p *proxyHealth) recordProxyOK(host string, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if h, ok := p.hosts[host]; ok {
		h.fails = 0
		h.lastProxyOK = now
		if h.state == stateNormal {
			if now.Sub(h.lastProxyOK) > proxyFailWindow {
				delete(p.hosts, host)
			}
		}
	}
}

// recordDirectOK transitions directTest → directHold (direct connection with
// downstream data succeeded; start probing the proxy every 3 minutes).
func (p *proxyHealth) recordDirectOK(host string, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	h := p.hosts[host]
	if h == nil {
		return
	}
	if h.state == stateDirectTest {
		h.state = stateDirectHold
		h.testUntil = time.Time{}
		h.probeFails = 0
	}
}

// markProbeFail records a failed proxy-recovery probe. Fast hosts drop back
// to directTest immediately; others accumulate up to probeFailThreshold, then
// mark a fixed-direct alert.
func (p *proxyHealth) markProbeFail(host string, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	h := p.hosts[host]
	if h == nil || h.state != stateDirectHold {
		return
	}
	h.probeFails++
	if h.fast {
		h.state = stateDirectTest
		h.testUntil = now.Add(directTestDuration)
		h.probeFails = 0
		return
	}
	if h.probeFails >= probeFailThreshold {
		h.alertedAt = now
	}
}

// markProbeOK transitions directHold → normal (or fastDirect), reporting
// whether the host is now back on proxy.
func (p *proxyHealth) markProbeOK(host string, now time.Time) (recovered bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	h := p.hosts[host]
	if h == nil || h.state != stateDirectHold {
		return false
	}
	if h.fast {
		h.state = stateFastDirect
	} else {
		h.state = stateNormal
	}
	h.probeFails = 0
	h.alertedAt = time.Time{}
	h.lastProxyOK = now
	return true
}

// directDecision reports whether a default-proxy host should dial direct
// first (directTest / directHold states).
func (p *proxyHealth) directDecision(host string, now time.Time) (directFirst bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	h := p.hosts[host]
	if h == nil {
		return false
	}
	switch h.state {
	case stateDirectTest:
		return now.Before(h.testUntil)
	case stateDirectHold:
		return true
	}
	return false
}

// shouldProbe reports hosts due for a proxy-recovery probe and the list of
// hosts currently in an alerted fixed-direct state.
func (p *proxyHealth) probeDue(now time.Time) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var due []string
	for host, h := range p.hosts {
		if h.state != stateDirectHold {
			continue
		}
		if h.lastProxyOK.IsZero() || now.Sub(h.lastProxyOK) >= probeInterval {
			due = append(due, host)
		}
	}
	return due
}

func (p *proxyHealth) alertedHosts(now time.Time) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []string
	for host, h := range p.hosts {
		if h.state == stateDirectHold && !h.alertedAt.IsZero() {
			out = append(out, host)
		}
	}
	return out
}

func (p *proxyHealth) isFast(host string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if h, ok := p.hosts[host]; ok {
		return h.fast
	}
	return false
}

// markFast flags a host as a fast-switch host (1-fail threshold) after it
// recovers from a learned-direct state.
func (p *proxyHealth) markFast(host string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if h, ok := p.hosts[host]; ok {
		h.fast = true
	}
}

func (p *proxyHealth) pruneLocked(now time.Time) {
	for host, h := range p.hosts {
		if h.state == stateNormal && now.Sub(h.firstAt) > proxyFailWindow {
			delete(p.hosts, host)
		}
	}
}
