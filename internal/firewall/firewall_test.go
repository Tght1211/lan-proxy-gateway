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
		Iface:       "en0",
		GatewayIP:   "192.168.1.100",
		RedirPort:   17892,
		DNSPort:     53,
		TCPRedirect: true,
		DNSHijack:   true,
		QUICBlock:   true,
	}
	got := renderPFAnchor(cfg)
	wantLines := []string{
		"rdr pass on en0 proto udp from any to ! 192.168.1.100 port 53 -> 127.0.0.1 port 53",
		"rdr pass on en0 proto tcp from any to ! 192.168.1.100 port 53 -> 127.0.0.1 port 53",
		"rdr pass on en0 proto tcp from any to ! 192.168.1.100 -> 127.0.0.1 port 17892",
		"nat on en0 from any to any -> (en0)",
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
