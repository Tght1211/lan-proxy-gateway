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

// BuildRoutingPolicy constructs a routingPolicy from the given parameters.
// Exported so the daemon can share the same policy with both TCP and UDP relays.
func BuildRoutingPolicy(defaultAction string, direct, proxy Dialer, rules []RouteRule) *routingPolicy {
	return &routingPolicy{
		defaultAction: defaultAction,
		direct:        direct,
		proxy:         proxy,
		rules:         compileRules(rules),
	}
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

// selectDialer picks the egress for one connection. srcIP is the LAN device
// address; host is the observed domain (or IP literal); ip is the real
// destination address, invalid when the target came through fake-ip and only
// the domain is known. matchedRule reports whether an explicit rule matched
// (false = default action decided), which the caller uses to gate proxy→direct
// fallback retries.
func (p *routingPolicy) selectDialer(srcIP, host string, ip netip.Addr) (Dialer, bool, bool, bool) {
	action := p.defaultAction
	matchedRule := false
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	// Device rules are a pre-routing policy layer, independent of their visual
	// position in the flat editor. A device override must never be shadowed by
	// an earlier domain/IP rule.
	for _, c := range p.rules {
		if c.rule.Type == "src-ip" && c.rule.Value == srcIP {
			return p.selectAction(c.rule.Action, true)
		}
	}
	for _, c := range p.rules {
		rule := c.rule
		if rule.Type == "src-ip" {
			continue
		}
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
			matchedRule = true
			break
		}
	}
	return p.selectAction(action, matchedRule)
}

func (p *routingPolicy) selectAction(action string, matchedRule bool) (Dialer, bool, bool, bool) {
	switch action {
	case RouteReject:
		return nil, false, true, matchedRule
	case RouteProxy:
		if p.proxy != nil {
			return p.proxy, true, false, matchedRule
		}
	}
	return p.direct, false, false, matchedRule
}

// matchType returns the type of the first rule matching the given connection,
// or "" when only the default action applies. Used to detect src-ip device
// overrides without re-implementing the matching loop.
func (p *routingPolicy) matchType(srcIP, host string, ip netip.Addr) string {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, c := range p.rules {
		if c.rule.Type == "src-ip" && c.rule.Value == srcIP {
			return c.rule.Type
		}
	}
	for _, c := range p.rules {
		rule := c.rule
		switch rule.Type {
		case "domain":
			if host == strings.ToLower(strings.Trim(rule.Value, ".")) {
				return rule.Type
			}
		case "domain-suffix":
			value := strings.ToLower(strings.Trim(rule.Value, "."))
			if host == value || strings.HasSuffix(host, "."+value) {
				return rule.Type
			}
		case "ip-cidr":
			if ip.IsValid() && c.prefix.Contains(ip.Unmap()) {
				return rule.Type
			}
		}
	}
	return ""
}

// classifyDialError reduces an egress dial error to a short reason shown in
// the connection history.
func classifyDialError(err error, viaProxy bool) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded"):
		return "连接超时"
	case strings.Contains(msg, "connection refused"):
		return "连接被拒"
	case strings.Contains(msg, "no route to host") || strings.Contains(msg, "network is unreachable") || strings.Contains(msg, "unreachable"):
		return "目标不可达"
	case strings.Contains(msg, "no such host") || strings.Contains(msg, "dns"):
		return "域名解析失败"
	case viaProxy:
		return "上游代理错误"
	default:
		return "连接失败"
	}
}
