package relay

import "testing"

type namedDialer struct{ Dialer }

func TestRoutingPolicyFirstMatchWins(t *testing.T) {
	direct := &namedDialer{}
	proxy := &namedDialer{}
	p := &routingPolicy{
		defaultAction: RouteProxy,
		direct:        direct,
		proxy:         proxy,
		rules: []RouteRule{
			{Type: "domain", Value: "api.example.com", Action: RouteReject},
			{Type: "domain-suffix", Value: "example.com", Action: RouteDirect},
		},
	}
	if _, _, rejected := p.selectDialer("api.example.com"); !rejected {
		t.Fatal("exact rule should reject before suffix rule")
	}
	if got, viaProxy, _ := p.selectDialer("www.example.com"); got != direct || viaProxy {
		t.Fatal("suffix rule should use direct")
	}
	if got, viaProxy, _ := p.selectDialer("other.test"); got != proxy || !viaProxy {
		t.Fatal("unmatched host should use default proxy")
	}
}
