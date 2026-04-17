// Package traffic implements the "sub" feature: traffic control policy.
//
// Given a config.TrafficConfig, it produces the `rules:` section that mihomo
// consumes. The same policy drives all three modes (rule/global/direct) —
// mihomo's native `mode:` field handles the coarse routing, and this rule
// renderer handles the fine-grained overrides (adblock, extras, rulesets).
package traffic

import (
	"strings"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/traffic/rulesets"
)

// ProxyTag is the name of the proxy-group that routes to the user's proxy source.
// Kept in sync with internal/engine/render.go so rules can target it.
const ProxyTag = "Proxy"

// Render returns the `rules:` YAML block (including the "rules:" header).
// It's deterministic: no randomness, no time-based input.
func Render(t config.TrafficConfig) string {
	var b strings.Builder
	b.WriteString("rules:\n")

	emit := func(lines []string, verdict string) {
		for _, l := range lines {
			b.WriteString("  - ")
			b.WriteString(withVerdict(l, verdict))
			b.WriteString("\n")
		}
	}

	// Adblock is orthogonal to mode: even "direct" mode still kills ads.
	if t.Adblock {
		emit(rulesets.Adblock(), "REJECT")
	}
	// User extras always come before built-ins so they can override.
	emit(t.Extras.Reject, "REJECT")
	emit(t.Extras.Direct, "DIRECT")
	emit(t.Extras.Proxy, ProxyTag)

	switch t.Mode {
	case config.ModeDirect:
		// Nothing more — mihomo's mode=direct catches everything else.
	case config.ModeGlobal:
		// Still respect LAN direct so we don't route 192.168/16 through the proxy.
		if t.Rulesets.LANDirect {
			emit(rulesets.LANDirect(), "DIRECT")
		}
	case config.ModeRule:
		fallthrough
	default:
		if t.Rulesets.LANDirect {
			emit(rulesets.LANDirect(), "DIRECT")
		}
		if t.Rulesets.Nintendo {
			emit(rulesets.Nintendo(), ProxyTag)
		}
		if t.Rulesets.Apple {
			emit(rulesets.Apple(), "DIRECT")
		}
		if t.Rulesets.ChinaDirect {
			emit(rulesets.ChinaDirect(), "DIRECT")
		}
		if t.Rulesets.Global {
			emit(rulesets.Global(), ProxyTag)
		}
		b.WriteString("  - MATCH,")
		b.WriteString(ProxyTag)
		b.WriteString("\n")
	}
	return b.String()
}

// withVerdict returns a rule line guaranteed to have a verdict at the right spot.
//
// clash rule syntax is: TYPE,MATCHER[,VERDICT][,MODIFIER...]
// where MODIFIER is things like "no-resolve", "src". If the line the user or
// a ruleset gives us has a modifier but no verdict, the verdict must slot in
// between the matcher and the modifier, not tacked onto the end — or mihomo
// refuses to parse and treats "no-resolve" as a proxy-group name.
func withVerdict(line, verdict string) string {
	parts := strings.Split(line, ",")
	for i, p := range parts {
		p = strings.TrimSpace(p)
		switch p {
		case "DIRECT", "REJECT", "Proxy", "PROXY":
			// Verdict already present; pass through.
			return line
		case "no-resolve", "src", "src-no-resolve":
			// Insert verdict before the first modifier we see.
			if i == 0 {
				// no-resolve on its own wouldn't make sense as pos 0; still insert.
				continue
			}
			head := strings.Join(parts[:i], ",")
			tail := strings.Join(parts[i:], ",")
			return head + "," + verdict + "," + tail
		}
	}
	// No verdict and no modifier: just append.
	return line + "," + verdict
}
