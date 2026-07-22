package relay

import "strings"

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

type routingPolicy struct {
	defaultAction string
	direct        Dialer
	proxy         Dialer
	rules         []RouteRule
}

func (p *routingPolicy) selectDialer(host string) (Dialer, bool, bool) {
	action := p.defaultAction
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, rule := range p.rules {
		value := strings.ToLower(strings.Trim(rule.Value, "."))
		matched := rule.Type == "domain" && host == value
		if rule.Type == "domain-suffix" {
			matched = host == value || strings.HasSuffix(host, "."+value)
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
