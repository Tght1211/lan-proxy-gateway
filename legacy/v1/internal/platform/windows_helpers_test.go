package platform

import "testing"

func TestParseWindowsDefaultRoute(t *testing.T) {
	sample := `===========================================================================
Interface List
 11...60 45 bd 3a 11 22 ......Intel(R) Ethernet Controller
===========================================================================
IPv4 Route Table
===========================================================================
Active Routes:
Network Destination        Netmask          Gateway       Interface  Metric
          0.0.0.0          0.0.0.0     192.168.12.1    192.168.12.100     25
        127.0.0.0        255.0.0.0         On-link         127.0.0.1    331
`

	route, ok := parseWindowsDefaultRoute(sample)
	if !ok {
		t.Fatalf("expected default route to be parsed")
	}
	if route.Gateway != "192.168.12.1" {
		t.Fatalf("Gateway = %q, want %q", route.Gateway, "192.168.12.1")
	}
	if route.InterfaceIP != "192.168.12.100" {
		t.Fatalf("InterfaceIP = %q, want %q", route.InterfaceIP, "192.168.12.100")
	}
}

func TestParseWindowsDefaultRouteRejectsInvalidRows(t *testing.T) {
	route, ok := parseWindowsDefaultRoute("Network Destination Netmask Gateway Interface Metric")
	if ok {
		t.Fatalf("expected invalid route output to fail, got %#v", route)
	}
}

func TestBuildWindowsStartupTaskCommand(t *testing.T) {
	cfg := ServiceConfig{
		BinaryPath: `C:\Program Files\gateway\gateway.exe`,
		ConfigFile: `C:\Users\demo\gateway.yaml`,
		DataDir:    `C:\Users\demo\data`,
	}

	got := buildWindowsStartupTaskCommand(cfg)
	want := `"C:\Program Files\gateway\gateway.exe" start --config "C:\Users\demo\gateway.yaml" --data-dir "C:\Users\demo\data"`
	if got != want {
		t.Fatalf("buildWindowsStartupTaskCommand() = %q, want %q", got, want)
	}
}
