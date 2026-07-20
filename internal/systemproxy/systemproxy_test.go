package systemproxy

import "testing"

func TestParseProxyOutput(t *testing.T) {
	got := parseProxyOutput("Enabled: Yes\nServer: 127.0.0.1\nPort: 7897\nAuthenticated Proxy Enabled: 0\n")
	if !got.Enabled || got.Host != "127.0.0.1" || got.Port != 7897 {
		t.Fatalf("unexpected proxy: %+v", got)
	}
}

func TestStatusSummary(t *testing.T) {
	status := Status{Services: []Service{
		{Name: "Ethernet"},
		{Name: "Wi-Fi", Mode: ModeSOCKS5, Enabled: true, Host: "127.0.0.1", Port: 7897},
	}}
	if got := status.Summary(); got != "socks5 127.0.0.1:7897" {
		t.Fatalf("summary = %q", got)
	}
}
