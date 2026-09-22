package firewall

import (
	"fmt"
	"strings"
)

// renderPFAnchor computes the pf anchor file content for cfg.
// Pure (no OS calls) so it can be golden-tested without root.
//
// Rule order: DNS hijack rdr first, then the generic TCP rdr (first matching
// translation rule wins). `pass` on rdr rules bypasses filter evaluation for
// translated flows. The nat rule uses (iface) so it tracks DHCP address
// changes; the rdr exclusion embeds the current gateway IP and is refreshed
// on re-Apply.
func renderPFAnchor(c Config) string {
	var b strings.Builder
	b.WriteString("# lan-proxy-gateway — managed file, do not edit\n")

	// Exclude CIDRs: `no rdr` opts flows out of translation. Must stay in the
	// translation section (macOS pf rejects filter rules before rdr), and must
	// precede the rdr rules because the first matching translation rule wins.
	for _, cidr := range c.ExcludeCIDRs {
		fmt.Fprintf(&b, "no rdr on %s from %s to any\n", c.Iface, cidr)
	}

	// Source filter: "from <cidr>" or "from any"
	src := pfSource(c.LANCIDRs)

	if c.DNSLocalRedirect {
		// Keep the LAN destination address; only translate the port so replies
		// retain the gateway's address (including on Android DNS clients).
		for _, proto := range []string{"udp", "tcp"} {
			fmt.Fprintf(&b, "rdr pass on %s proto %s from %s to %s port 53 -> %s port %d\n", c.Iface, proto, src, c.GatewayIP, c.GatewayIP, c.DNSPort)
		}
	}

	if c.DNSHijack {
		// Queries already addressed to the gateway must reach the DNS listener
		// directly. Re-redirecting them through loopback breaks ColorOS's DNS
		// cache even though the response is visible on the wire.
		fmt.Fprintf(&b, "rdr pass on %s proto udp from %s to ! %s port 53 -> 127.0.0.1 port %d\n", c.Iface, src, c.GatewayIP, c.DNSPort)
		fmt.Fprintf(&b, "rdr pass on %s proto tcp from %s to ! %s port 53 -> 127.0.0.1 port %d\n", c.Iface, src, c.GatewayIP, c.DNSPort)
	}
	if c.UDPFakeIPRedir && c.FakeIPRange != "" && c.UDPRedirPort > 0 {
		fmt.Fprintf(&b, "rdr pass on %s proto udp from %s to %s -> 127.0.0.1 port %d\n", c.Iface, src, c.FakeIPRange, c.UDPRedirPort)
	}
	if c.TCPRedirect {
		fmt.Fprintf(&b, "rdr pass on %s proto tcp from %s to ! %s -> 127.0.0.1 port %d\n", c.Iface, src, c.GatewayIP, c.RedirPort)
	}
	fmt.Fprintf(&b, "nat on %s from %s to any -> (%s)\n", c.Iface, src, c.Iface)
	for _, source := range c.BlockedSources {
		fmt.Fprintf(&b, "block return quick on %s from %s to any\n", c.Iface, source)
	}
	if c.QUICBlock {
		fmt.Fprintf(&b, "block return quick on %s proto udp from %s to any port 443\n", c.Iface, src)
	}
	if c.IPv6Block {
		// Forward-only scope (parity with the Linux FORWARD chain): keep
		// NDP/RA alive and let the gateway host itself stay reachable over
		// IPv6; only inbound LAN traffic destined elsewhere is refused.
		fmt.Fprintf(&b, "pass quick on %s inet6 proto ipv6-icmp\n", c.Iface)
		fmt.Fprintf(&b, "pass in quick on %s inet6 from any to (%s)\n", c.Iface, c.Iface)
		fmt.Fprintf(&b, "block return in quick on %s inet6 from any to any\n", c.Iface)
	}
	return b.String()
}

// pfSource builds the pf "from" clause. Multiple CIDRs use a pf table syntax: "{ cidr1 cidr2 }".
func pfSource(cidrs []string) string {
	switch len(cidrs) {
	case 0:
		return "any"
	case 1:
		return cidrs[0]
	default:
		return "{ " + strings.Join(cidrs, " ") + " }"
	}
}
