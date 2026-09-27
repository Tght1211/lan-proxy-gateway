package firewall

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

var testCfg = Config{
	Iface:     "eth0",
	GatewayIP: "192.168.1.100",
	RedirPort: 17892,
	DNSPort:   53,
}

func joinRules(rules [][]string) []string {
	out := make([]string, 0, len(rules))
	for _, r := range rules {
		out = append(out, strings.Join(r, " "))
	}
	return out
}

func TestRenderLinuxRules(t *testing.T) {
	cases := []struct {
		name                   string
		redirect, hijack, quic bool
		wantNat, wantFilter    []string
	}{
		{
			name: "direct mode no hijack no quic",
			wantNat: []string{
				"POSTROUTING -o eth0 -m comment --comment lan-proxy-gateway -j MASQUERADE",
			},
		},
		{
			name:     "proxy mode full",
			redirect: true, hijack: true, quic: true,
			wantNat: []string{
				"PREROUTING -m addrtype --dst-type LOCAL -m comment --comment lan-proxy-gateway -j RETURN",
				"PREROUTING -i eth0 -p udp --dport 53 -m comment --comment lan-proxy-gateway -j REDIRECT --to-ports 53",
				"PREROUTING -i eth0 -p tcp --dport 53 -m comment --comment lan-proxy-gateway -j REDIRECT --to-ports 53",
				"PREROUTING -i eth0 -p tcp -m comment --comment lan-proxy-gateway -j REDIRECT --to-ports 17892",
				"POSTROUTING -o eth0 -m comment --comment lan-proxy-gateway -j MASQUERADE",
			},
			wantFilter: []string{
				"FORWARD -i eth0 -p udp --dport 443 -m comment --comment lan-proxy-gateway -j REJECT",
			},
		},
		{
			name:     "redirect only",
			redirect: true,
			wantNat: []string{
				"PREROUTING -m addrtype --dst-type LOCAL -m comment --comment lan-proxy-gateway -j RETURN",
				"PREROUTING -i eth0 -p tcp -m comment --comment lan-proxy-gateway -j REDIRECT --to-ports 17892",
				"POSTROUTING -o eth0 -m comment --comment lan-proxy-gateway -j MASQUERADE",
			},
		},
		{
			name:   "hijack only",
			hijack: true,
			wantNat: []string{
				"PREROUTING -m addrtype --dst-type LOCAL -m comment --comment lan-proxy-gateway -j RETURN",
				"PREROUTING -i eth0 -p udp --dport 53 -m comment --comment lan-proxy-gateway -j REDIRECT --to-ports 53",
				"PREROUTING -i eth0 -p tcp --dport 53 -m comment --comment lan-proxy-gateway -j REDIRECT --to-ports 53",
				"POSTROUTING -o eth0 -m comment --comment lan-proxy-gateway -j MASQUERADE",
			},
		},
		{
			name: "quic only (direct mode with quic off normally)",
			quic: true,
			wantNat: []string{
				"POSTROUTING -o eth0 -m comment --comment lan-proxy-gateway -j MASQUERADE",
			},
			wantFilter: []string{
				"FORWARD -i eth0 -p udp --dport 443 -m comment --comment lan-proxy-gateway -j REJECT",
			},
		},
		{
			name:     "redirect+hijack no quic",
			redirect: true, hijack: true,
			wantNat: []string{
				"PREROUTING -m addrtype --dst-type LOCAL -m comment --comment lan-proxy-gateway -j RETURN",
				"PREROUTING -i eth0 -p udp --dport 53 -m comment --comment lan-proxy-gateway -j REDIRECT --to-ports 53",
				"PREROUTING -i eth0 -p tcp --dport 53 -m comment --comment lan-proxy-gateway -j REDIRECT --to-ports 53",
				"PREROUTING -i eth0 -p tcp -m comment --comment lan-proxy-gateway -j REDIRECT --to-ports 17892",
				"POSTROUTING -o eth0 -m comment --comment lan-proxy-gateway -j MASQUERADE",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := testCfg
			cfg.TCPRedirect, cfg.DNSHijack, cfg.QUICBlock = c.redirect, c.hijack, c.quic
			nat, filter := renderLinuxRules(cfg)
			if !reflect.DeepEqual(joinRules(nat), c.wantNat) {
				t.Fatalf("nat =\n%s\nwant\n%s", strings.Join(joinRules(nat), "\n"), strings.Join(c.wantNat, "\n"))
			}
			gotFilter := joinRules(filter)
			if len(gotFilter) == 0 {
				gotFilter = nil
			}
			if !reflect.DeepEqual(gotFilter, c.wantFilter) {
				t.Fatalf("filter = %v, want %v", gotFilter, c.wantFilter)
			}
		})
	}
}

func TestRenderPFAnchor(t *testing.T) {
	cfg := Config{
		Iface:          "en0",
		GatewayIP:      "192.168.1.100",
		RedirPort:      17892,
		DNSPort:        53,
		TCPRedirect:    true,
		DNSHijack:      true,
		QUICBlock:      true,
		BlockedSources: []string{"192.168.1.50"},
	}
	got := renderPFAnchor(cfg)
	wantLines := []string{
		"rdr pass on en0 proto udp from any to ! 192.168.1.100 port 53 -> 127.0.0.1 port 53",
		"rdr pass on en0 proto tcp from any to ! 192.168.1.100 port 53 -> 127.0.0.1 port 53",
		"rdr pass on en0 proto tcp from any to ! 192.168.1.100 -> 127.0.0.1 port 17892",
		"nat on en0 from any to any -> (en0)",
		"block return quick on en0 from 192.168.1.50 to any",
		"block return quick on en0 proto udp from any to any port 443",
	}
	for _, want := range wantLines {
		if !strings.Contains(got, want) {
			t.Fatalf("anchor missing %q in:\n%s", want, got)
		}
	}
	// DNS rdr must precede the generic TCP rdr
	if strings.Index(got, "port 53") > strings.Index(got, "17892") {
		t.Fatalf("DNS rdr must come first:\n%s", got)
	}

	// minimal config: only nat
	min := renderPFAnchor(Config{Iface: "en0"})
	if strings.Contains(min, "rdr") || strings.Contains(min, "block") {
		t.Fatalf("minimal config should have no rdr/block:\n%s", min)
	}
	if !strings.Contains(min, "nat on en0 from any to any -> (en0)") {
		t.Fatalf("minimal config missing nat:\n%s", min)
	}
}

func TestRenderLinuxBlockedSources(t *testing.T) {
	cfg := testCfg
	cfg.BlockedSources = []string{"192.168.1.50", "192.168.1.51"}
	_, filter := renderLinuxRules(cfg)
	want := []string{
		"FORWARD -i eth0 -s 192.168.1.50 -m comment --comment lan-proxy-gateway -j REJECT",
		"FORWARD -i eth0 -s 192.168.1.51 -m comment --comment lan-proxy-gateway -j REJECT",
	}
	if got := joinRules(filter); !reflect.DeepEqual(got, want) {
		t.Fatalf("filter = %v, want %v", got, want)
	}
}

// ---------- linux sync logic (seams, no root) ----------

func TestSyncTableDiff(t *testing.T) {
	current := `# Generated by iptables-save
*nat
:PREROUTING ACCEPT [0:0]
-A PREROUTING -i eth0 -p tcp -m comment --comment lan-proxy-gateway -j REDIRECT --to-ports 17892
-A PREROUTING -p tcp -j SOMEONES_ELSE
-A POSTROUTING -o eth0 -m comment --comment lan-proxy-gateway -j MASQUERADE
COMMIT
`
	var ran []string
	m := &iptablesManager{
		run: func(args ...string) error {
			ran = append(ran, strings.Join(args, " "))
			return nil
		},
		save: func(table string) (string, error) { return current, nil },
	}
	desired := [][]string{
		{"POSTROUTING", "-o", "eth0", "-m", "comment", "--comment", "lan-proxy-gateway", "-j", "MASQUERADE"},
		{"PREROUTING", "-i", "eth0", "-p", "udp", "--dport", "53", "-m", "comment", "--comment", "lan-proxy-gateway", "-j", "REDIRECT", "--to-ports", "53"},
	}
	if err := m.syncTable("nat", desired); err != nil {
		t.Fatal(err)
	}
	var adds, dels int
	for _, r := range ran {
		if strings.Contains(r, " -A ") {
			adds++
			if !strings.Contains(r, "--dport 53") {
				t.Fatalf("unexpected add: %s", r)
			}
		}
		if strings.Contains(r, " -D ") {
			dels++
			if !strings.Contains(r, "17892") {
				t.Fatalf("unexpected del: %s", r)
			}
			if strings.Contains(r, "SOMEONES_ELSE") {
				t.Fatalf("must never delete foreign rules: %s", r)
			}
		}
	}
	if adds != 1 || dels != 1 {
		t.Fatalf("adds=%d dels=%d, want 1/1; ran=%v", adds, dels, ran)
	}
}

func TestSyncTableRemoveAll(t *testing.T) {
	current := `*nat
-A PREROUTING -i eth0 -p tcp -m comment --comment lan-proxy-gateway -j REDIRECT --to-ports 17892
COMMIT
`
	var ran []string
	m := &iptablesManager{
		run:  func(args ...string) error { ran = append(ran, strings.Join(args, " ")); return nil },
		save: func(table string) (string, error) { return current, nil },
	}
	if err := m.syncTable("nat", nil); err != nil {
		t.Fatal(err)
	}
	if len(ran) != 1 || !strings.Contains(ran[0], "-D PREROUTING") {
		t.Fatalf("want one delete, got %v", ran)
	}
}

// ---------- darwin manager (seams, no root) ----------

func TestDarwinApplyEnablesPFOnlyWhenDisabled(t *testing.T) {
	var ran []string
	var wrote string
	m := &darwinManager{
		run: func(args ...string) error { ran = append(ran, strings.Join(args, " ")); return nil },
		output: func(args ...string) (string, error) {
			switch strings.Join(args, " ") {
			case "-s nat":
				return `nat-anchor "com.apple/*"`, nil
			case "-s info":
				return "Status: Disabled for 0 days", nil
			}
			return "", fmt.Errorf("unexpected %v", args)
		},
		write: func(path string, data []byte) error { wrote = string(data); return nil },
	}
	rep, err := m.Apply(Config{Iface: "en0", GatewayIP: "192.168.1.100", RedirPort: 17892, DNSPort: 53})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.WeEnabledPF {
		t.Fatal("pf was disabled; report must say we enabled it")
	}
	if !strings.Contains(wrote, "nat on en0") {
		t.Fatalf("anchor file = %q", wrote)
	}
	var hasEnable bool
	for _, r := range ran {
		if r == "-e" {
			hasEnable = true
		}
	}
	if !hasEnable {
		t.Fatalf("want pfctl -e, ran %v", ran)
	}
}

func TestDarwinApplyFailsWithoutWildcardAnchor(t *testing.T) {
	m := &darwinManager{
		run: func(args ...string) error { return nil },
		output: func(args ...string) (string, error) {
			return "# empty ruleset", nil
		},
		write: func(path string, data []byte) error { return nil },
	}
	_, err := m.Apply(Config{Iface: "en0"})
	if err == nil || !strings.Contains(err.Error(), "com.apple") {
		t.Fatalf("want wildcard-anchor error, got %v", err)
	}
}

func TestRenderPFAnchorUDPFakeIP(t *testing.T) {
	cfg := Config{
		Iface:          "en0",
		GatewayIP:      "192.168.1.100",
		RedirPort:      17892,
		UDPRedirPort:   17893,
		FakeIPRange:    "198.18.0.0/16",
		DNSPort:        53,
		TCPRedirect:    true,
		UDPFakeIPRedir: true,
		DNSHijack:      true,
		QUICBlock:      true,
	}
	got := renderPFAnchor(cfg)
	// QUIC must remain unredirected so the filter can reject it.
	wantLine := "rdr pass on en0 proto udp from any to 198.18.0.0/16 -> 127.0.0.1 port 17893"
	if !strings.Contains(got, wantLine) {
		t.Fatalf("anchor missing UDP fake-IP rdr rule %q in:\n%s", wantLine, got)
	}
	exclusion := "no rdr on en0 proto udp from any to 198.18.0.0/16 port 443"
	if i := strings.Index(got, exclusion); i < 0 || i > strings.Index(got, wantLine) {
		t.Fatalf("QUIC must bypass UDP rdr before the filter rejects it:\n%s", got)
	}
	cfg.QUICBlock = false
	if strings.Contains(renderPFAnchor(cfg), exclusion) {
		t.Fatal("QUIC disabled but still excluded")
	}
	// UDP rdr must come after DNS hijack but before TCP rdr
	dnsIdx := strings.Index(got, "port 53")
	udpIdx := strings.Index(got, "198.18.0.0/16")
	tcpIdx := strings.Index(got, "17892")
	if udpIdx < dnsIdx {
		t.Fatalf("UDP fake-IP rdr should come after DNS hijack:\n%s", got)
	}
	if udpIdx > tcpIdx {
		t.Fatalf("UDP fake-IP rdr should come before TCP rdr:\n%s", got)
	}
}

func TestRenderLinuxRulesUDPFakeIP(t *testing.T) {
	cfg := testCfg
	cfg.TCPRedirect = true
	cfg.UDPFakeIPRedir = true
	cfg.UDPRedirPort = 17893
	cfg.FakeIPRange = "198.18.0.0/16"
	cfg.QUICBlock = true // Keep QUIC in FORWARD so its reject rule applies.
	nat, _ := renderLinuxRules(cfg)
	rules := joinRules(nat)
	wantRule := "PREROUTING -i eth0 -p udp -d 198.18.0.0/16 ! --dport 443 -m comment --comment lan-proxy-gateway -j REDIRECT --to-ports 17893"
	found := false
	for _, r := range rules {
		if r == wantRule {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("UDP fake-IP REDIRECT rule missing, got:\n%s", strings.Join(rules, "\n"))
	}
}

func TestRenderPFAnchorIPv6Block(t *testing.T) {
	cfg := Config{Iface: "en0", GatewayIP: "192.168.1.100", IPv6Block: true}
	got := renderPFAnchor(cfg)
	// Forward-only block: host's own IPv6 (NDP, inbound-to-self) must stay alive.
	for _, want := range []string{
		"pass quick on en0 inet6 proto ipv6-icmp",
		"pass in quick on en0 inet6 from any to (en0)",
		"block return in quick on en0 inet6 from any to any",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("anchor missing IPv6 rule %q in:\n%s", want, got)
		}
	}
	// Without IPv6Block the rule must not appear
	cfg.IPv6Block = false
	got2 := renderPFAnchor(cfg)
	if strings.Contains(got2, "inet6") {
		t.Fatalf("IPv6 block rule should not appear when disabled:\n%s", got2)
	}
}

func TestRenderLinuxIPv6Rules(t *testing.T) {
	cfg := testCfg
	cfg.IPv6Block = true
	rules := renderLinuxIPv6Rules(cfg)
	if len(rules) != 1 {
		t.Fatalf("want 1 IPv6 rule, got %d", len(rules))
	}
	got := strings.Join(rules[0], " ")
	want := "FORWARD -i eth0 -m comment --comment lan-proxy-gateway -j REJECT"
	if got != want {
		t.Fatalf("IPv6 rule = %q, want %q", got, want)
	}
	// Disabled
	cfg.IPv6Block = false
	if rules := renderLinuxIPv6Rules(cfg); len(rules) != 0 {
		t.Fatalf("want 0 IPv6 rules when disabled, got %d", len(rules))
	}
}

func TestRenderPFAnchorLANCIDRs(t *testing.T) {
	cfg := Config{
		Iface: "en0", GatewayIP: "192.168.1.100",
		TCPRedirect: true, RedirPort: 17892,
		LANCIDRs: []string{"192.168.1.0/24"},
	}
	got := renderPFAnchor(cfg)
	if !strings.Contains(got, "from 192.168.1.0/24 to") {
		t.Fatalf("anchor should restrict source to CIDR:\n%s", got)
	}
	if strings.Contains(got, "from any to") {
		t.Fatalf("anchor should NOT use 'from any' when LANCIDRs set:\n%s", got)
	}
}

func TestRenderPFAnchorExcludeCIDRs(t *testing.T) {
	cfg := Config{
		Iface: "en0", GatewayIP: "192.168.1.100",
		TCPRedirect: true, RedirPort: 17892,
		ExcludeCIDRs: []string{"172.17.0.0/16"},
	}
	got := renderPFAnchor(cfg)
	// `no rdr` must precede the rdr rules (first matching translation rule wins),
	// and no filter rule may precede translation rules on macOS pf.
	noRdr := strings.Index(got, "no rdr on en0 from 172.17.0.0/16 to any")
	rdr := strings.Index(got, "rdr pass on en0")
	if noRdr < 0 {
		t.Fatalf("anchor should exclude Docker CIDR via no rdr:\n%s", got)
	}
	if rdr >= 0 && noRdr > rdr {
		t.Fatalf("no rdr must precede rdr rules:\n%s", got)
	}
}

func TestRenderPFAnchorMultipleLANCIDRs(t *testing.T) {
	cfg := Config{
		Iface: "en0", GatewayIP: "192.168.1.100",
		TCPRedirect: true, RedirPort: 17892,
		LANCIDRs: []string{"192.168.1.0/24", "10.0.0.0/8"},
	}
	got := renderPFAnchor(cfg)
	if !strings.Contains(got, "{ 192.168.1.0/24 10.0.0.0/8 }") {
		t.Fatalf("anchor should use pf table syntax for multiple CIDRs:\n%s", got)
	}
}

func TestRenderLinuxRulesLANCIDRs(t *testing.T) {
	cfg := testCfg
	cfg.TCPRedirect = true
	cfg.LANCIDRs = []string{"192.168.1.0/24"}
	nat, _ := renderLinuxRules(cfg)
	found := false
	for _, rule := range nat {
		joined := strings.Join(rule, " ")
		if strings.Contains(joined, "-s 192.168.1.0/24") && strings.Contains(joined, "REDIRECT") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("iptables should include -s CIDR in redirect rules")
	}
}

func TestRenderLinuxRulesExcludeCIDRs(t *testing.T) {
	cfg := testCfg
	cfg.TCPRedirect = true
	cfg.ExcludeCIDRs = []string{"172.17.0.0/16"}
	nat, _ := renderLinuxRules(cfg)
	if len(nat) == 0 {
		t.Fatal("expected nat rules")
	}
	first := strings.Join(nat[0], " ")
	if !strings.Contains(first, "-s 172.17.0.0/16") || !strings.Contains(first, "RETURN") {
		t.Fatalf("first iptables rule should RETURN excluded CIDR, got: %s", first)
	}
}
