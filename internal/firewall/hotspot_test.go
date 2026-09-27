package firewall

import (
	"strings"
	"testing"
)

func TestHotspotRulesExcludeHostAndRelyOnSystemNAT(t *testing.T) {
	s := renderPFAnchor(Config{Hotspot: true, Iface: "bridge100", CaptureInterfaces: []string{"bridge100", "ap1"}, GatewayIP: "192.168.2.1", LANCIDRs: []string{"192.168.2.0/24"}, DNSLocalRedirect: true, DNSHijack: true, DNSPort: 1053, TCPRedirect: true, RedirPort: 17892, IPv6Block: true, QUICBlock: true, BlockedSources: []string{"192.168.2.8"}})
	for _, forbidden := range []string{"nat on ", "on en0", "rdr pass on lo0"} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("host-affecting rule %q: %s", forbidden, s)
		}
	}
	for _, required := range []string{
		"no rdr on { bridge100 ap1 } from 192.168.2.1 to any",
		"no rdr on { bridge100 ap1 } from 192.168.2.8 to any",
		"rdr pass on { bridge100 ap1 } proto udp from 192.168.2.0/24 to 192.168.2.1 port 53 -> 192.168.2.1 port 1053",
		"rdr pass on { bridge100 ap1 } proto tcp from 192.168.2.0/24 to ! 192.168.2.1 -> 127.0.0.1 port 17892",
		"block return in quick on { bridge100 ap1 } inet6 from any to any",
	} {
		if !strings.Contains(s, required) {
			t.Fatalf("missing %q: %s", required, s)
		}
	}
}
