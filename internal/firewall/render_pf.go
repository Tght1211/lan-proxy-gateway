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

	// Exclude CIDRs: pass out before any rdr/block captures them.
	for _, cidr := range c.ExcludeCIDRs {
		fmt.Fprintf(&b, "pass in quick on %s from %s to any\n", c.Iface, cidr)
	}

	// Source filter: "from <cidr>" or "from any"
	src := pfSource(c.LANCIDRs)

	if c.DNSHijack {
		// Queries already addressed to the gateway must reach the DNS listener
		// directly. Re-redirecting them through loopback breaks ColorOS's DNS
		// cache even though the response is visible on the wire.
		fmt.Fprintf(&b, "rdr pass on %s proto udp from %s to ! %s port %d -> 127.0.0.1 port %d\n", c.Iface, src, c.GatewayIP, c.DNSPort, c.DNSPort)
		fmt.Fprintf(&b, "rdr pass on %s proto tcp from %s to ! %s port %d -> 127.0.0.1 port %d\n", c.Iface, src, c.GatewayIP, c.DNSPort, c.DNSPort)
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
		fmt.Fprintf(&b, "block return quick on %s inet6 from any to any\n", c.Iface)
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
