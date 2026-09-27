//go:build darwin

package firewall

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHotspotPFUsesOwnEarlyAnchorAndNeverEnablesSystemPF(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		var writes int
		var commands []string
		m := &darwinManager{
			write: func(string, []byte) error { writes++; return nil },
			run:   func(args ...string) error { commands = append(commands, strings.Join(args, " ")); return nil },
			output: func(args ...string) (string, error) {
				if strings.Join(args, " ") == "-s nat" {
					return `rdr-anchor "com.apple/*"`, nil
				}
				if enabled {
					return "Status: Enabled", nil
				}
				return "Status: Disabled", nil
			},
		}
		report, err := m.Apply(Config{Hotspot: true, Iface: "bridge100"})
		if !enabled {
			if err == nil || writes != 0 || len(commands) != 0 {
				t.Fatal("disabled sharing PF must remain untouched")
			}
		} else {
			if err != nil {
				t.Fatal(err)
			}
			if report.WeEnabledPF || len(commands) != 2 || !strings.HasPrefix(commands[0], "-a com.apple/000.lan-proxy-gateway -f ") {
				t.Fatalf("unexpected PF mutation: %v %+v", commands, report)
			}
			if commands[1] != "-a com.apple/lan-proxy-gateway -F all" {
				t.Fatal("legacy ingress was not removed")
			}
		}
	}
}

func TestHotspotNativePFSyntax(t *testing.T) {
	if os.Getenv("HOTSPOT_PF_SYNTAX") != "1" {
		t.Skip("opt-in syntax validation on a Mac with Internet Sharing interfaces")
	}
	rules := renderPFAnchor(Config{Hotspot: true, Iface: "bridge100", CaptureInterfaces: []string{"bridge100", "ap1"}, GatewayIP: "192.168.2.1", LANCIDRs: []string{"192.168.2.0/24"}, DNSLocalRedirect: true, DNSHijack: true, DNSPort: 1053, TCPRedirect: true, RedirPort: 17892, UDPFakeIPRedir: true, UDPRedirPort: 17893, FakeIPRange: "198.18.0.0/16", QUICBlock: true, IPv6Block: true})
	p := filepath.Join(t.TempDir(), "hotspot.pf")
	if err := os.WriteFile(p, []byte(rules), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("/sbin/pfctl", "-n", "-f", p).CombinedOutput()
	if err != nil {
		t.Fatalf("pf syntax rejected: %v\n%s\n%s", err, out, rules)
	}
}
