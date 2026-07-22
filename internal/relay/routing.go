package relay

import (
	"net/netip"
	"strings"
)

const (
	RouteProxy  = "proxy"
	RouteDirect = "direct"
	RouteReject = "reject"
)

type RouteRule struct {
	Type   string
	Value  string
	Action string
}

type compiledRule struct {
	rule   RouteRule
	prefix netip.Prefix
}

type routingPolicy struct {
	defaultAction string
	direct        Dialer
	proxy         Dialer
	rules         []compiledRule
}

func compileRules(rules []RouteRule) []compiledRule {
	out := make([]compiledRule, 0, len(rules))
	for _, rule := range rules {
		c := compiledRule{rule: rule}
		if rule.Type == "ip-cidr" {
			prefix, err := netip.ParsePrefix(rule.Value)
			if err != nil {
				continue
			}
			c.prefix = prefix
		}
		out = append(out, c)
	}
	return out
}

// selectDialer picks the egress for one connection. host is the observed
// domain (or IP literal); ip is the real destination address, invalid when the
// target came through fake-ip and only the domain is known.
func (p *routingPolicy) selectDialer(host string, ip netip.Addr) (Dialer, bool, bool) {
	action := p.defaultAction
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, c := range p.rules {
		rule := c.rule
		matched := false
		switch rule.Type {
		case "domain":
			matched = host == strings.ToLower(strings.Trim(rule.Value, "."))
		case "domain-suffix":
			value := strings.ToLower(strings.Trim(rule.Value, "."))
			matched = host == value || strings.HasSuffix(host, "."+value)
		case "ip-cidr":
			matched = ip.IsValid() && c.prefix.Contains(ip.Unmap())
		}
		if matched {
			action = rule.Action
			break
		}
	}
	switch action {
	case RouteReject:
		return nil, false, true
	case RouteProxy:
		if p.proxy != nil {
			return p.proxy, true, false
		}
	}
	return p.direct, false, false
}
