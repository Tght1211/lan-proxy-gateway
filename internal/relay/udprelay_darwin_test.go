//go:build darwin

package relay

import (
	"net"
	"testing"
	"time"
)

func TestUDPRelayListenerMatchesPFLoopbackDestination(t *testing.T) {
	r := NewUDPRelay(UDPRelayOptions{})
	if r.listenAddr != FormatUDPListenAddr(UDPRedirPort) {
		t.Fatal("default UDP listener does not match platform redirect address")
	}
	addr, err := net.ResolveUDPAddr("udp4", FormatUDPListenAddr(0))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	local := conn.LocalAddr().(*net.UDPAddr)
	if !local.IP.Equal(net.IPv4(127, 0, 0, 1)) {
		t.Fatalf("PF lookup needs concrete loopback address, got %v", local)
	}
	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := conn.WriteToUDP([]byte("reply"), client.LocalAddr().(*net.UDPAddr)); err != nil {
		t.Fatal(err)
	}
	client.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 64)
	n, source, err := client.ReadFromUDP(buf)
	if err != nil || string(buf[:n]) != "reply" || !source.IP.Equal(local.IP) || source.Port != local.Port {
		t.Fatalf("reply must retain PF's translated source tuple: %v, %v", source, err)
	}
}
