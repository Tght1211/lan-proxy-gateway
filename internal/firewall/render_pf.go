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
	if c.DNSHijack {
		// Queries already addressed to the gateway must reach the DNS listener
		// directly. Re-redirecting them through loopback breaks ColorOS's DNS
		// cache even though the response is visible on the wire.
		fmt.Fprintf(&b, "rdr pass on %s proto udp from any to ! %s port %d -> 127.0.0.1 port %d\n", c.Iface, c.GatewayIP, c.DNSPort, c.DNSPort)
		fmt.Fprintf(&b, "rdr pass on %s proto tcp from any to ! %s port %d -> 127.0.0.1 port %d\n", c.Iface, c.GatewayIP, c.DNSPort, c.DNSPort)
	}
	if c.UDPFakeIPRedir && c.FakeIPRange != "" && c.UDPRedirPort > 0 {
		// Redirect UDP packets destined for the fake-IP range into the UDP relay,
		// so game voice / video calls reach their real destination.
		// Must come AFTER DNS hijack (port 53 already handled above) and BEFORE
		// the generic tcp rdr — first matching rdr wins.
		// No port 443 exclusion needed: fake-IP addresses never host real QUIC
		// servers; the QUIC block targets real-destination UDP/443 which never
		// lands in the fake-IP range.
		fmt.Fprintf(&b, "rdr pass on %s proto udp from any to %s -> 127.0.0.1 port %d\n", c.Iface, c.FakeIPRange, c.UDPRedirPort)
	}
	if c.TCPRedirect {
		fmt.Fprintf(&b, "rdr pass on %s proto tcp from any to ! %s -> 127.0.0.1 port %d\n", c.Iface, c.GatewayIP, c.RedirPort)
	}
	fmt.Fprintf(&b, "nat on %s from any to any -> (%s)\n", c.Iface, c.Iface)
	for _, source := range c.BlockedSources {
		fmt.Fprintf(&b, "block return quick on %s from %s to any\n", c.Iface, source)
	}
	if c.QUICBlock {
		fmt.Fprintf(&b, "block return quick on %s proto udp from any to any port 443\n", c.Iface)
	}
	if c.IPv6Block {
		fmt.Fprintf(&b, "block return quick on %s inet6 from any to any\n", c.Iface)
	}
	return b.String()
}
