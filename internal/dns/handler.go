package dns

import (
	"net/netip"
	"strings"

	"github.com/miekg/dns"
)

// action tells the handler how to answer a question.
type action int

const (
	actionForward    action = iota // forward to upstream resolvers as-is
	actionFakeIP                   // answer with a fake-ip address (A queries only)
	actionNODATA                   // NOERROR with empty answer (AAAA suppression)
	actionPTRFromMap               // answer PTR from the fake-ip map, forward if absent
)

// builtinFakeIPFilter holds suffixes that must always be resolved for real:
// local namespaces, reverse zones, and captive-portal connectivity-check
// domains. The last group is critical: if a phone's connectivity check gets a
// fake IP it can't complete, Android/iOS mark the Wi-Fi as "no internet" and
// move all app traffic to cellular.
var builtinFakeIPFilter = []string{
	"lan.", "local.", "localhost.", "internal.", "home.arpa.",
	"in-addr.arpa.", "ip6.arpa.",
	// captive-portal / connectivity checks
	"connectivitycheck.gstatic.com.",
	"clients3.google.com.",
	"googleapis.cn.",
	"captive.apple.com.",
	"connectivity.rom.miui.com.",
	"connectivitycheck.platform.hicloud.com.",
	"wifi.vivo.com.cn.",
	"allawntech.com.",
	"heytapmobi.com.",
	"detectportal.firefox.com.",
	"msftconnecttest.com.",
}

// suffixFilter matches qnames against suffix list entries (FQDN, trailing dot).
type suffixFilter struct {
	suffixes []string
}

func newSuffixFilter(extra []string) *suffixFilter {
	f := &suffixFilter{suffixes: append([]string{}, builtinFakeIPFilter...)}
	for _, s := range extra {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		if !strings.HasSuffix(s, ".") {
			s += "."
		}
		f.suffixes = append(f.suffixes, s)
	}
	return f
}

// match reports whether qname (FQDN) equals or ends with a listed suffix
// on a label boundary.
func (f *suffixFilter) match(qname string) bool {
	q := strings.ToLower(qname)
	for _, s := range f.suffixes {
		if q == s || strings.HasSuffix(q, "."+s) {
			return true
		}
	}
	return false
}

// decide picks the handling for one question. Pure and table-testable.
//
// Rules:
//   - PTR: answer from the fake-ip map when the address is ours (else forward)
//   - AAAA: always NODATA — the relay is IPv4-only; real AAAA answers would let
//     devices bypass the proxy over forwarded v6 or break inconsistently
//   - A + fake-ip on + non-loopback client + name outside the filter: fake-ip
//   - everything else: forward (loopback clients always get real answers so the
//     gateway host itself can safely use this resolver)
func decide(clientIP netip.Addr, qname string, qtype uint16, fakeIPEnabled bool, filter *suffixFilter, fakeRange netip.Prefix) action {
	switch qtype {
	case dns.TypePTR:
		if ip, err := parseReverseIPv4(qname); err == nil && fakeRange.Contains(ip) {
			return actionPTRFromMap
		}
		return actionForward
	case dns.TypeAAAA:
		return actionNODATA
	case dns.TypeA:
		if fakeIPEnabled && !clientIP.IsLoopback() && !filter.match(qname) {
			return actionFakeIP
		}
		return actionForward
	default:
		return actionForward
	}
}

// parseReverseIPv4 parses "7.0.18.198.in-addr.arpa." → 198.18.0.7.
func parseReverseIPv4(qname string) (netip.Addr, error) {
	q := strings.ToLower(strings.TrimSuffix(qname, "."))
	if !strings.HasSuffix(q, ".in-addr.arpa") {
		return netip.Addr{}, errNotReverse
	}
	parts := strings.Split(strings.TrimSuffix(q, ".in-addr.arpa"), ".")
	if len(parts) != 4 {
		return netip.Addr{}, errNotReverse
	}
	// labels are reversed in the arpa form
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return netip.ParseAddr(strings.Join(parts, "."))
}

var errNotReverse = errNotReverseT("not a reverse IPv4 name")

type errNotReverseT string

func (e errNotReverseT) Error() string { return string(e) }
