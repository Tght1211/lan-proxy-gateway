package firewall

import (
	"strconv"
	"strings"
)

// renderLinuxRules computes the desired iptables rules for cfg.
// Each rule is the argv following the chain operation, e.g.
// {"PREROUTING", "-m", "addrtype", ...} — the caller prepends -A/-D.
// The result is pure (no OS calls) so it can be golden-tested without root.
//
// Order matters: the LOCAL exemption must precede the redirect rules so
// traffic destined to the gateway host itself (SSH, DNS-to-us, ...) is never
// captured. MASQUERADE is always installed — it serves direct egress and
// non-hijacked UDP in proxy mode.
func renderLinuxRules(c Config) (nat [][]string, filter [][]string) {
	tag := []string{"-m", "comment", "--comment", CommentTag}
	iface := []string{"-i", c.Iface}
	dnsPort := strconv.Itoa(c.DNSPort)
	redirPort := strconv.Itoa(c.RedirPort)

	// Exclude CIDRs: RETURN before any capture rule.
	for _, cidr := range c.ExcludeCIDRs {
		nat = append(nat, joinArgs(
			[]string{"PREROUTING"}, iface, []string{"-s", cidr},
			tag, []string{"-j", "RETURN"},
		))
	}

	if c.DNSHijack || c.TCPRedirect {
		nat = append(nat, joinArgs(
			[]string{"PREROUTING", "-m", "addrtype", "--dst-type", "LOCAL"},
			tag, []string{"-j", "RETURN"},
		))
	}

	srcArgs := iptablesSrcArgs(c.LANCIDRs)

	if c.DNSHijack {
		for _, proto := range []string{"udp", "tcp"} {
			nat = append(nat, joinArgs(
				[]string{"PREROUTING"}, iface, srcArgs,
				[]string{"-p", proto, "--dport", dnsPort},
				tag, []string{"-j", "REDIRECT", "--to-ports", dnsPort},
			))
		}
	}
	if c.UDPFakeIPRedir && c.FakeIPRange != "" && c.UDPRedirPort > 0 {
		udpPort := strconv.Itoa(c.UDPRedirPort)
		nat = append(nat, joinArgs(
			[]string{"PREROUTING"}, iface, srcArgs,
			[]string{"-p", "udp", "-d", c.FakeIPRange},
			tag, []string{"-j", "REDIRECT", "--to-ports", udpPort},
		))
	}
	if c.TCPRedirect {
		nat = append(nat, joinArgs(
			[]string{"PREROUTING"}, iface, srcArgs, []string{"-p", "tcp"},
			tag, []string{"-j", "REDIRECT", "--to-ports", redirPort},
		))
	}
	nat = append(nat, joinArgs(
		[]string{"POSTROUTING", "-o", c.Iface},
		tag, []string{"-j", "MASQUERADE"},
	))
	for _, source := range c.BlockedSources {
		filter = append(filter, joinArgs(
			[]string{"FORWARD"}, iface, []string{"-s", source},
			tag, []string{"-j", "REJECT"},
		))
	}

	if c.QUICBlock {
		filter = append(filter, joinArgs(
			[]string{"FORWARD"}, iface, srcArgs,
			[]string{"-p", "udp", "--dport", "443"},
			tag, []string{"-j", "REJECT"},
		))
	}
	return nat, filter
}

// iptablesSrcArgs builds `-s cidr` args. Multiple CIDRs use a comma-separated
// match set (iptables supports -s cidr1,cidr2 in some versions; for max
// compat we use a single CIDR or return empty for "any").
func iptablesSrcArgs(cidrs []string) []string {
	if len(cidrs) == 0 {
		return nil
	}
	// iptables -s accepts comma-separated CIDRs natively.
	return []string{"-s", strings.Join(cidrs, ",")}
}

// renderLinuxIPv6Rules returns ip6tables FORWARD rules to block all forwarded IPv6 traffic.
func renderLinuxIPv6Rules(c Config) [][]string {
	if !c.IPv6Block {
		return nil
	}
	tag := []string{"-m", "comment", "--comment", CommentTag}
	iface := []string{"-i", c.Iface}
	return [][]string{
		joinArgs([]string{"FORWARD"}, iface, tag, []string{"-j", "REJECT"}),
	}
}

func joinArgs(parts ...[]string) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
