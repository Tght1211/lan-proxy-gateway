package relay

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"
)

// TestUDPRelayForwardsFakeIP verifies the end-to-end UDP relay flow:
// client → (fake IP) → relay → (resolve domain) → real server → relay → client.
//
// @author buchi
// @since 2026-08-08
func TestUDPRelayForwardsFakeIP(t *testing.T) {
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

	tracker := NewTracker()
	r := NewUDPRelay(UDPRelayOptions{
		ListenAddr:   "127.0.0.1:0",
		FakeIPRange:  &fakePrefix,
		LookupFakeIP: lookupFakeIP,
		Resolve:      resolve,
		Tracker:      tracker,
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

	clientConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()
	clientAddr := clientConn.LocalAddr().(*net.UDPAddr).AddrPort()

	origDst := netip.AddrPortFrom(fakeIP, uint16(realAddr.Port))

	payload := []byte("hello voice")
	r.handlePacket(payload, clientAddr, origDst)

	time.Sleep(100 * time.Millisecond)

	if r.SessionCount() != 1 {
		t.Fatalf("expected 1 session, got %d", r.SessionCount())
	}

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	readBuf := make([]byte, 4096)
	n, _, err := clientConn.ReadFrom(readBuf)
	if err != nil {
		t.Fatalf("client did not receive response: %v", err)
	}
	if string(readBuf[:n]) != "hello voice" {
		t.Fatalf("unexpected response: %q", readBuf[:n])
	}

	// Verify tracker recorded the session
	snap := tracker.Snapshot()
	if len(snap.Active) != 1 {
		t.Fatalf("tracker should have 1 active connection, got %d", len(snap.Active))
	}
	if snap.Active[0].DstHost != "voice.game.com" {
		t.Fatalf("tracked host = %q, want voice.game.com", snap.Active[0].DstHost)
	}
	if snap.Active[0].Up == 0 {
		t.Fatal("tracked upstream bytes should be > 0")
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

func TestUDPRelayRejectsDomainRule(t *testing.T) {
	fakePrefix := netip.MustParsePrefix("198.18.0.0/16")
	fakeIP := netip.MustParseAddr("198.18.0.10")

	lookupFakeIP := func(ip netip.Addr) (string, bool) {
		if ip == fakeIP {
			return "ads.tracker.com", true
		}
		return "", false
	}
	resolve := func(ctx context.Context, domain string) (netip.Addr, error) {
		return netip.MustParseAddr("127.0.0.1"), nil
	}

	tracker := NewTracker()
	r := NewUDPRelay(UDPRelayOptions{
		ListenAddr:   "127.0.0.1:0",
		FakeIPRange:  &fakePrefix,
		LookupFakeIP: lookupFakeIP,
		Resolve:      resolve,
		Tracker:      tracker,
	})
	laddr, _ := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	conn, _ := net.ListenUDP("udp4", laddr)
	r.conn = conn
	defer conn.Close()

	// Set a reject rule for ads.tracker.com
	r.SetRouting(BuildRoutingPolicy("direct", &namedDialer{}, nil, []RouteRule{
		{Type: "domain-suffix", Value: "tracker.com", Action: RouteReject},
	}))

	origDst := netip.AddrPortFrom(fakeIP, 12345)
	client := netip.MustParseAddrPort("192.168.1.50:54321")
	r.handlePacket([]byte("track-me"), client, origDst)

	if r.SessionCount() != 0 {
		t.Fatal("session created for rejected domain")
	}
	snap := tracker.Snapshot()
	if len(snap.Recent) != 1 || !snap.Recent[0].Rejected {
		t.Fatalf("tracker should record one rejected connection, got %+v", snap.Recent)
	}
}

func TestUDPRelaySrcIPRejectBlocksDevice(t *testing.T) {
	fakePrefix := netip.MustParsePrefix("198.18.0.0/16")
	fakeIP := netip.MustParseAddr("198.18.0.20")

	lookupFakeIP := func(ip netip.Addr) (string, bool) {
		if ip == fakeIP {
			return "voice.game.com", true
		}
		return "", false
	}
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

	r.SetRouting(BuildRoutingPolicy("direct", &namedDialer{}, nil, []RouteRule{
		{Type: "src-ip", Value: "192.168.1.50", Action: RouteReject},
	}))

	origDst := netip.AddrPortFrom(fakeIP, 12345)
	blocked := netip.MustParseAddrPort("192.168.1.50:54321")
	r.handlePacket([]byte("blocked"), blocked, origDst)
	if r.SessionCount() != 0 {
		t.Fatal("session created for rejected device")
	}

	allowed := netip.MustParseAddrPort("192.168.1.51:54321")
	r.handlePacket([]byte("allowed"), allowed, origDst)
	if r.SessionCount() != 1 {
		t.Fatal("session not created for allowed device")
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

	tracker := NewTracker()
	r := NewUDPRelay(UDPRelayOptions{
		ListenAddr:   "127.0.0.1:0",
		FakeIPRange:  &fakePrefix,
		LookupFakeIP: lookupFakeIP,
		Resolve:      resolve,
		Tracker:      tracker,
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
		s.lastActive.Store(time.Now().Add(-3 * time.Minute).UnixNano())
	}
	r.mu.Unlock()

	r.cleanup()

	if r.SessionCount() != 0 {
		t.Fatal("stale session not cleaned up")
	}

	// Tracker should have moved session to recent
	snap := tracker.Snapshot()
	if len(snap.Active) != 0 {
		t.Fatal("cleaned session should not be active in tracker")
	}
	if len(snap.Recent) != 1 {
		t.Fatalf("cleaned session should appear in recent, got %d", len(snap.Recent))
	}
}
