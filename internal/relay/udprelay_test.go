package relay

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"
)

func TestUDPRelayForwardsFakeIP(t *testing.T) {
	// 1. Start a "real server" UDP echo server.
	realServer, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer realServer.Close()
	realAddr := realServer.LocalAddr().(*net.UDPAddr)

	go func() {
		buf := make([]byte, 4096)
		for {
			n, addr, err := realServer.ReadFrom(buf)
			if err != nil {
				return
			}
			_, _ = realServer.WriteTo(buf[:n], addr)
		}
	}()

	// 2. Set up the fake-IP pool: 198.18.0.1 → "voice.game.com"
	fakeIP := netip.MustParseAddr("198.18.0.1")
	fakePrefix := netip.MustParsePrefix("198.18.0.0/16")

	lookupFakeIP := func(ip netip.Addr) (string, bool) {
		if ip == fakeIP {
			return "voice.game.com", true
		}
		return "", false
	}

	resolve := func(ctx context.Context, domain string) (netip.Addr, error) {
		if domain == "voice.game.com" {
			return netip.MustParseAddr("127.0.0.1"), nil
		}
		return netip.Addr{}, &net.DNSError{Err: "not found", Name: domain}
	}

	// 3. Create and start the UDP relay.
	r := NewUDPRelay(UDPRelayOptions{
		ListenAddr:   "127.0.0.1:0",
		FakeIPRange:  &fakePrefix,
		LookupFakeIP: lookupFakeIP,
		Resolve:      resolve,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	laddr, _ := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		t.Fatal(err)
	}
	r.conn = conn
	go r.cleanupLoop(ctx)

	// 4. Simulate a client sending UDP to fake IP on the real server's port.
	clientConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()
	clientAddr := clientConn.LocalAddr().(*net.UDPAddr).AddrPort()

	origDst := netip.AddrPortFrom(fakeIP, uint16(realAddr.Port))

	payload := []byte("hello voice")
	r.handlePacket(payload, clientAddr, origDst)

	// 5. Wait for the echo and verify.
	time.Sleep(100 * time.Millisecond)

	if r.SessionCount() != 1 {
		t.Fatalf("expected 1 session, got %d", r.SessionCount())
	}

	readBuf := make([]byte, 4096)
	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := clientConn.ReadFrom(readBuf)
	if err != nil {
		t.Fatalf("client did not receive response: %v", err)
	}
	if string(readBuf[:n]) != "hello voice" {
		t.Fatalf("unexpected response: %q", readBuf[:n])
	}

	cancel()
	_ = r.Close()
	_ = conn.Close()
}

func TestUDPRelayIgnoresNonFakeIP(t *testing.T) {
	fakePrefix := netip.MustParsePrefix("198.18.0.0/16")
	lookupFakeIP := func(ip netip.Addr) (string, bool) { return "", false }

	r := NewUDPRelay(UDPRelayOptions{
		ListenAddr:   "127.0.0.1:0",
		FakeIPRange:  &fakePrefix,
		LookupFakeIP: lookupFakeIP,
	})
	laddr, _ := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	conn, _ := net.ListenUDP("udp4", laddr)
	r.conn = conn
	defer conn.Close()

	nonFake := netip.MustParseAddrPort("8.8.8.8:53")
	client := netip.MustParseAddrPort("192.168.1.50:12345")
	r.handlePacket([]byte("test"), client, nonFake)

	if r.SessionCount() != 0 {
		t.Fatal("session created for non-fake-IP destination")
	}
}

func TestUDPRelaySessionCleanup(t *testing.T) {
	fakePrefix := netip.MustParsePrefix("198.18.0.0/16")
	fakeIP := netip.MustParseAddr("198.18.0.5")

	lookupFakeIP := func(ip netip.Addr) (string, bool) {
		if ip == fakeIP {
			return "test.example.com", true
		}
		return "", false
	}

	realServer, _ := net.ListenPacket("udp4", "127.0.0.1:0")
	defer realServer.Close()
	realAddr := realServer.LocalAddr().(*net.UDPAddr)

	resolve := func(ctx context.Context, domain string) (netip.Addr, error) {
		return netip.MustParseAddr("127.0.0.1"), nil
	}

	r := NewUDPRelay(UDPRelayOptions{
		ListenAddr:   "127.0.0.1:0",
		FakeIPRange:  &fakePrefix,
		LookupFakeIP: lookupFakeIP,
		Resolve:      resolve,
	})
	laddr, _ := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	conn, _ := net.ListenUDP("udp4", laddr)
	r.conn = conn
	defer conn.Close()

	origDst := netip.AddrPortFrom(fakeIP, uint16(realAddr.Port))
	client := netip.MustParseAddrPort("192.168.1.50:54321")

	r.handlePacket([]byte("data"), client, origDst)
	if r.SessionCount() != 1 {
		t.Fatalf("expected 1 session, got %d", r.SessionCount())
	}

	// Manually expire the session.
	r.mu.Lock()
	for _, s := range r.sessions {
		s.lastActive = time.Now().Add(-3 * time.Minute)
	}
	r.mu.Unlock()

	r.cleanup()

	if r.SessionCount() != 0 {
		t.Fatal("stale session not cleaned up")
	}
}
