package relay

import (
	"net/netip"
	"testing"
	"time"
)

func TestProxyHealthEntersDirectTest(t *testing.T) {
	p := newProxyHealth()
	now := time.Now()
	p.recordFailure("x.com", now)
	p.recordFailure("x.com", now.Add(time.Second))
	if p.directDecision("x.com", now.Add(2*time.Second)) {
		t.Fatal("must not direct before threshold")
	}
	p.recordFailure("x.com", now.Add(2 * time.Second))
	if !p.directDecision("x.com", now.Add(3*time.Second)) {
		t.Fatal("third failure inside window must enter direct test")
	}
	if p.directDecision("x.com", now.Add(directTestDuration+time.Minute)) {
		t.Fatal("direct-test window must expire")
	}
}

func TestProxyHealthStaleStreakResets(t *testing.T) {
	p := newProxyHealth()
	now := time.Now()
	p.recordFailure("y.com", now)
	p.recordFailure("y.com", now.Add(time.Second))
	p.recordFailure("y.com", now.Add(proxyFailWindow+time.Minute))
	if p.directDecision("y.com", now.Add(proxyFailWindow+2*time.Minute)) {
		t.Fatal("stale failures must not count toward the threshold")
	}
}

func TestProxyHealthProxyOKClearsStreak(t *testing.T) {
	p := newProxyHealth()
	now := time.Now()
	p.recordFailure("z.com", now)
	p.recordFailure("z.com", now)
	p.recordProxyOK("z.com", now.Add(time.Second))
	p.recordFailure("z.com", now.Add(2*time.Second))
	if p.directDecision("z.com", now.Add(3*time.Second)) {
		t.Fatal("proxy success must clear the failure streak")
	}
}

func TestProxyHealthDirectOKTransitionsToHold(t *testing.T) {
	p := newProxyHealth()
	now := time.Now()
	for i := 0; i < proxyFailThreshold; i++ {
		p.recordFailure("h.com", now.Add(time.Duration(i)*time.Second))
	}
	if !p.directDecision("h.com", now.Add(time.Second)) {
		t.Fatal("should be in direct test")
	}
	p.recordDirectOK("h.com", now.Add(time.Second))
	if !p.directDecision("h.com", now.Add(directTestDuration+time.Hour)) {
		t.Fatal("directHold must stay direct beyond test window")
	}
}

func TestProxyHealthProbeRecoveryAndFastSwitch(t *testing.T) {
	p := newProxyHealth()
	now := time.Now()
	for i := 0; i < proxyFailThreshold; i++ {
		p.recordFailure("r.com", now.Add(time.Duration(i)*time.Second))
	}
	p.recordDirectOK("r.com", now.Add(time.Second))
	if !p.markProbeOK("r.com", now.Add(time.Minute)) {
		t.Fatal("probe success should recover")
	}
	if p.directDecision("r.com", now.Add(time.Minute)) {
		t.Fatal("recovered host must not direct")
	}
	p.markFast("r.com")
	p.recordFailure("r.com", now.Add(time.Minute))
	if !p.directDecision("r.com", now.Add(2*time.Minute)) {
		t.Fatal("fast host must switch after a single failure")
	}
}

func TestProxyHealthProbeFailAlert(t *testing.T) {
	p := newProxyHealth()
	now := time.Now()
	for i := 0; i < proxyFailThreshold; i++ {
		p.recordFailure("a.com", now.Add(time.Duration(i)*time.Second))
	}
	p.recordDirectOK("a.com", now.Add(time.Second))
	for i := 0; i < probeFailThreshold; i++ {
		p.markProbeFail("a.com", now.Add(time.Duration(i)*time.Minute))
	}
	alerts := p.alertedHosts(now.Add(time.Hour))
	if len(alerts) != 1 || alerts[0] != "a.com" {
		t.Fatalf("expected alerted a.com, got %v", alerts)
	}
}

func TestSrcIPOverrideBeatsDomainRule(t *testing.T) {
	direct := &namedDialer{}
	proxy := &namedDialer{}
	p := &routingPolicy{
		defaultAction: RouteProxy,
		direct:        direct,
		proxy:         proxy,
		rules: compileRules([]RouteRule{
			{Type: "src-ip", Value: "192.168.1.50", Action: RouteDirect},
			{Type: "domain-suffix", Value: "example.com", Action: RouteProxy},
		}),
	}
	got, viaProxy, _, matched := p.selectDialer("192.168.1.50", "www.example.com", netip.Addr{})
	if got != direct || viaProxy || !matched {
		t.Fatal("src-ip override must win and fix egress to direct")
	}
	got2, viaProxy2, _, _ := p.selectDialer("192.168.1.99", "www.example.com", netip.Addr{})
	if got2 != proxy || !viaProxy2 {
		t.Fatal("non-matching src-ip must fall through to domain rule → proxy")
	}
}
