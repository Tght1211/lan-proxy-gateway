package hotspot

import (
	"strings"
	"testing"
)

const hardwareFixture = `Hardware Port: Ethernet
Device: en0
Hardware Port: Wi-Fi
Device: en1
Hardware Port: Thunderbolt Bridge
Device: bridge0
`
const interfaceFixture = `en0: flags=8863<UP,BROADCAST,SMART,RUNNING,SIMPLEX,MULTICAST>
 inet 192.168.1.10 netmask 0xffffff00 broadcast 192.168.1.255
bridge0: flags=8863<UP,BROADCAST,SMART,RUNNING,SIMPLEX,MULTICAST>
 member: en2 flags=3<LEARNING,DISCOVER>
bridge100: flags=8863<UP,BROADCAST,SMART,RUNNING,SIMPLEX,MULTICAST>
 inet 192.168.2.1 netmask 0xffffff00 broadcast 192.168.2.255
 member: ap1 flags=3<LEARNING,DISCOVER>
`

func TestDiscoveryRequiresIsolatedWirelessSharing(t *testing.T) {
	for _, tc := range []struct {
		name, route, interfaces string
		sharing, available      bool
	}{
		{"sharing", "interface: en0", interfaceFixture, true, true},
		{"off", "interface: en0", interfaceFixture, false, false},
		{"wireless uplink", "interface: en1", interfaceFixture, true, false},
		{"VPN default", "interface: utun4", interfaceFixture, true, false},
		{"overlap", "interface: en0", strings.ReplaceAll(interfaceFixture, "192.168.2.", "192.168.1."), true, false},
		{"mixed bridge", "interface: en0", interfaceFixture + " member: en0 flags=3<LEARNING,DISCOVER>\n", true, false},
		{"wired bridge", "interface: en0", strings.ReplaceAll(interfaceFixture, "member: ap1", "member: en3"), true, false},
		{"ambiguous", "interface: en0", interfaceFixture + strings.ReplaceAll(interfaceFixture[strings.Index(interfaceFixture, "bridge100:"):], "bridge100:", "bridge101:"), true, false},
		{"renumbered bridge", "interface: en0", strings.ReplaceAll(interfaceFixture, "bridge100:", "bridge102:"), true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := Inspect(tc.route, hardwareFixture, tc.interfaces, tc.sharing)
			if s.Available != tc.available {
				t.Fatalf("status: %+v", s)
			}
			if s.Available && (s.IP != "192.168.2.1" || s.CIDR != "192.168.2.0/24" || s.Uplink != "en0") {
				t.Fatalf("wrong scope: %+v", s)
			}
		})
	}
}

func TestSavedSharingAndRadioReadiness(t *testing.T) {
	noBridge := interfaceFixture[:strings.Index(interfaceFixture, "bridge100:")]
	for _, tc := range []struct {
		name, interfaces string
		state            Readiness
		stage            string
		available        bool
	}{
		{"saved but radio off", noBridge, Readiness{SharingConfigured: true, WiFiPowerKnown: true}, "wifi_off", false},
		{"saved but not running", noBridge, Readiness{SharingConfigured: true, WiFiPowerKnown: true, WiFiPowered: true}, "configured_not_running", false},
		{"process not visible but network ready", interfaceFixture, Readiness{SharingConfigured: true, WiFiPowerKnown: true, WiFiPowered: true}, "ready", true},
		{"process running without bridge", noBridge, Readiness{SharingRunning: true}, "waiting_network", false},
		{"radio unknown is not off", noBridge, Readiness{}, "sharing_off", false},
		{"live AP despite client radio state", interfaceFixture, Readiness{SharingConfigured: true, WiFiPowerKnown: true}, "ready", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := InspectReadiness("interface: en0", hardwareFixture, tc.interfaces, tc.state)
			if s.Stage != tc.stage || s.Available != tc.available {
				t.Fatalf("%+v", s)
			}
		})
	}
}

func TestWiFiPowerParsing(t *testing.T) {
	if wifiDevice(hardwareFixture) != "en1" {
		t.Fatal("wrong Wi-Fi device")
	}
	for _, tc := range []struct {
		output    string
		known, on bool
	}{
		{"Wi-Fi Power (en1): Off\n", true, false},
		{"Wi-Fi Power (en1): On\n", true, true},
		{"", false, false}, {"An error occurred", false, false},
	} {
		known, on := wifiPower(tc.output)
		if known != tc.known || on != tc.on {
			t.Fatalf("%q: %v %v", tc.output, known, on)
		}
	}
}
