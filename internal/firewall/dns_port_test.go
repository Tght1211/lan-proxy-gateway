package firewall

import (
	"strings"
	"testing"
)

func TestAlternateDNSPort(t *testing.T) {
	for _, hijack := range []bool{false, true} {
		cfg := testCfg
		cfg.DNSPort = 1053
		cfg.DNSLocalRedirect = true
		cfg.DNSHijack = hijack
		cfg.TCPRedirect = true
		pf := renderPFAnchor(cfg)
		nat, _ := renderLinuxRules(cfg)
		linux := strings.Join(joinRules(nat), "\n")
		for _, proto := range []string{"udp", "tcp"} {
			wantPF := "proto " + proto + " from any to 192.168.1.100 port 53 -> 192.168.1.100 port 1053"
			if !strings.Contains(pf, wantPF) {
				t.Fatalf("missing local DNS redirect: %s", pf)
			}
			wantLinux := "-d 192.168.1.100 -p " + proto + " --dport 53"
			if i := strings.Index(linux, wantLinux); i < 0 || i > strings.Index(linux, "--dst-type LOCAL") {
				t.Fatalf("DNS mapping must precede LOCAL exemption: %s", linux)
			}
		}
		if strings.Contains(pf, "port 1053 ->") || strings.Contains(linux, "--dport 1053") {
			t.Fatal("must capture standard port 53, not the internal listener port")
		}
	}
	cfg := testCfg
	cfg.DNSPort = 1053
	if strings.Contains(renderPFAnchor(cfg), "port 1053") {
		t.Fatal("disabled DNS must not be redirected")
	}
}
