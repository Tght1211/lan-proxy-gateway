package egress

import (
	"net"
	"testing"
)

func TestIsFakeBenchmarkIP(t *testing.T) {
	if !isFakeBenchmarkIP(net.ParseIP("198.18.0.46")) {
		t.Fatal("198.18.0.46 should be treated as fake benchmark IP")
	}
	if isFakeBenchmarkIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("8.8.8.8 should not be treated as fake benchmark IP")
	}
}

func TestFirstUsableIPSkipsFakeRange(t *testing.T) {
	ips := []net.IP{
		net.ParseIP("198.18.0.46"),
		net.ParseIP("203.0.113.10"),
	}

	if got := firstUsableIP(ips); got != "203.0.113.10" {
		t.Fatalf("firstUsableIP() = %q, want 203.0.113.10", got)
	}
}
