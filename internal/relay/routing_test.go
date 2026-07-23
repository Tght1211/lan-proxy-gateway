package relay

import (
	"errors"
	"net/netip"
	"testing"
)

type namedDialer struct{ Dialer }

func TestRoutingPolicyFirstMatchWins(t *testing.T) {
	direct := &namedDialer{}
	proxy := &namedDialer{}
	p := &routingPolicy{
		defaultAction: RouteProxy,
		direct:        direct,
		proxy:         proxy,
		rules: compileRules([]RouteRule{
			{Type: "domain", Value: "api.example.com", Action: RouteReject},
			{Type: "domain-suffix", Value: "example.com", Action: RouteDirect},
		}),
	}
	if _, _, rejected, _ := p.selectDialer("api.example.com", netip.Addr{}); !rejected {
		t.Fatal("exact rule should reject before suffix rule")
	}
	if got, viaProxy, _, _ := p.selectDialer("www.example.com", netip.Addr{}); got != direct || viaProxy {
		t.Fatal("suffix rule should use direct")
	}
	if got, viaProxy, _, _ := p.selectDialer("other.test", netip.Addr{}); got != proxy || !viaProxy {
		t.Fatal("unmatched host should use default proxy")
	}
}

func TestRoutingPolicyIPCIDR(t *testing.T) {
	direct := &namedDialer{}
	proxy := &namedDialer{}
	p := &routingPolicy{
		defaultAction: RouteProxy,
		direct:        direct,
		proxy:         proxy,
		rules: compileRules([]RouteRule{
			{Type: "ip-cidr", Value: "10.0.0.0/8", Action: RouteDirect},
			{Type: "ip-cidr", Value: "203.0.113.0/24", Action: RouteReject},
		}),
	}
	if got, _, _, _ := p.selectDialer("10.1.2.3", netip.MustParseAddr("10.1.2.3")); got != direct {
		t.Fatal("in-range IP should match ip-cidr direct rule")
	}
	if _, _, rejected, _ := p.selectDialer("203.0.113.9", netip.MustParseAddr("203.0.113.9")); !rejected {
		t.Fatal("in-range IP should match ip-cidr reject rule")
	}
	if got, viaProxy, _, _ := p.selectDialer("198.51.100.1", netip.MustParseAddr("198.51.100.1")); got != proxy || !viaProxy {
		t.Fatal("out-of-range IP should fall through to default proxy")
	}
	// Fake-ip targets expose only the domain; ip-cidr rules must not match.
	if got, viaProxy, _, _ := p.selectDialer("fake.example.com", netip.Addr{}); got != proxy || !viaProxy {
		t.Fatal("invalid dest IP should never match ip-cidr rules")
	}
}

func TestCompileRulesSkipsInvalidCIDR(t *testing.T) {
	rules := compileRules([]RouteRule{
		{Type: "ip-cidr", Value: "not-a-cidr", Action: RouteDirect},
		{Type: "domain", Value: "example.com", Action: RouteReject},
	})
	if len(rules) != 1 || rules[0].rule.Type != "domain" {
		t.Fatalf("invalid ip-cidr rule should be skipped, got %+v", rules)
	}
}

func TestTrackerRecordRejected(t *testing.T) {
	tr := NewTracker()
	tr.RecordRejected("192.168.1.201", "ads.example.com", 443)
	snap := tr.Snapshot()
	if len(snap.Recent) != 1 {
		t.Fatalf("recent should contain the rejected record, got %d", len(snap.Recent))
	}
	rec := snap.Recent[0]
	if !rec.Rejected || rec.EndedAt == nil || rec.SrcIP != "192.168.1.201" || rec.DstHost != "ads.example.com" || rec.DstPort != 443 {
		t.Fatalf("unexpected rejected record: %+v", rec)
	}
	if len(snap.Devices) != 0 || len(snap.Services) != 0 {
		t.Fatal("rejected records must not count toward usage aggregates")
	}
}

func TestTrackerRecordDialFailure(t *testing.T) {
	tr := NewTracker()
	tr.RecordDialFailure("192.168.1.202", "example.com", 443, true, "连接超时")
	snap := tr.Snapshot()
	if len(snap.Recent) != 1 {
		t.Fatalf("recent should contain the failed record, got %d", len(snap.Recent))
	}
	rec := snap.Recent[0]
	if rec.Status != "dial_failed" || rec.Failure != "连接超时" || !rec.ViaProxy || rec.Rejected {
		t.Fatalf("unexpected dial-failure record: %+v", rec)
	}
	if len(snap.Devices) != 0 || len(snap.Services) != 0 {
		t.Fatal("failed records must not count toward usage aggregates")
	}
}

func TestClassifyDialError(t *testing.T) {
	cases := []struct {
		msg      string
		viaProxy bool
		want     string
	}{
		{"dial tcp 1.2.3.4:443: i/o timeout", false, "连接超时"},
		{"context deadline exceeded", true, "连接超时"},
		{"dial tcp 1.2.3.4:443: connect: connection refused", false, "连接被拒"},
		{"connect: no route to host", false, "目标不可达"},
		{"connect: network is unreachable", false, "目标不可达"},
		{"lookup example.com: no such host", true, "域名解析失败"},
		{"socks connect: general failure", true, "上游代理错误"},
		{"something odd", false, "连接失败"},
	}
	for _, c := range cases {
		if got := classifyDialError(errors.New(c.msg), c.viaProxy); got != c.want {
			t.Fatalf("classifyDialError(%q, %v) = %q, want %q", c.msg, c.viaProxy, got, c.want)
		}
	}
	if classifyDialError(nil, false) != "" {
		t.Fatal("nil error should classify to empty string")
	}
}
