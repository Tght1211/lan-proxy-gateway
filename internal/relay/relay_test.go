package relay

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------- fake SOCKS5 server ----------

type socks5Script struct {
	wantUser, wantPass string
	repCode            byte // reply code to send; 0 = success (echoes after)
	gotDomain          chan string
}

func startFakeSOCKS5(t *testing.T, script socks5Script) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveSOCKS5(conn, script)
		}
	}()
	return ln.Addr().String()
}

func serveSOCKS5(conn net.Conn, script socks5Script) {
	defer conn.Close()
	buf := make([]byte, 512)
	// greeting
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return
	}
	n := int(buf[1])
	if _, err := io.ReadFull(conn, buf[:n]); err != nil {
		return
	}
	if script.wantUser != "" {
		conn.Write([]byte{0x05, 0x02})
		// auth: VER ULEN USER PLEN PASS
		if _, err := io.ReadFull(conn, buf[:2]); err != nil {
			return
		}
		ulen := int(buf[1])
		if _, err := io.ReadFull(conn, buf[:ulen]); err != nil {
			return
		}
		user := string(buf[:ulen])
		if _, err := io.ReadFull(conn, buf[:1]); err != nil {
			return
		}
		plen := int(buf[0])
		if _, err := io.ReadFull(conn, buf[:plen]); err != nil {
			return
		}
		pass := string(buf[:plen])
		if user != script.wantUser || pass != script.wantPass {
			conn.Write([]byte{0x01, 0x01})
			return
		}
		conn.Write([]byte{0x01, 0x00})
	} else {
		conn.Write([]byte{0x05, 0x00})
	}
	// request
	if _, err := io.ReadFull(conn, buf[:4]); err != nil {
		return
	}
	atyp := buf[3]
	var host string
	switch atyp {
	case 0x01:
		if _, err := io.ReadFull(conn, buf[:4]); err != nil {
			return
		}
		host = net.IP(buf[:4]).String()
	case 0x03:
		if _, err := io.ReadFull(conn, buf[:1]); err != nil {
			return
		}
		l := int(buf[0])
		if _, err := io.ReadFull(conn, buf[:l]); err != nil {
			return
		}
		host = string(buf[:l])
	case 0x04:
		if _, err := io.ReadFull(conn, buf[:16]); err != nil {
			return
		}
		host = "::"
	}
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return
	}
	if script.gotDomain != nil {
		script.gotDomain <- host
	}
	// reply: VER REP RSV ATYP=1 0.0.0.0:0
	conn.Write([]byte{0x05, script.repCode, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	if script.repCode == 0 {
		io.Copy(conn, conn) // echo
	}
}

func TestSOCKS5DialerDomainAndEcho(t *testing.T) {
	gotDomain := make(chan string, 1)
	addr := startFakeSOCKS5(t, socks5Script{gotDomain: gotDomain})

	d := NewSOCKS5Dialer(addr, "", "", 5*time.Second)
	conn, err := d.DialContext(context.Background(), "tcp", "example.com:443")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	select {
	case host := <-gotDomain:
		if host != "example.com" {
			t.Fatalf("proxy got host %q, want example.com", host)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("proxy never got CONNECT request")
	}

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	out := make([]byte, 4)
	if _, err := io.ReadFull(conn, out); err != nil {
		t.Fatalf("echo read: %v", err)
	}
	if string(out) != "ping" {
		t.Fatalf("echo = %q", out)
	}
}

func TestSOCKS5DialerAuth(t *testing.T) {
	addr := startFakeSOCKS5(t, socks5Script{wantUser: "u1", wantPass: "p1"})

	ok := NewSOCKS5Dialer(addr, "u1", "p1", 5*time.Second)
	conn, err := ok.DialContext(context.Background(), "tcp", "1.2.3.4:80")
	if err != nil {
		t.Fatalf("auth dial: %v", err)
	}
	conn.Close()

	bad := NewSOCKS5Dialer(addr, "u1", "wrong", 5*time.Second)
	if _, err := bad.DialContext(context.Background(), "tcp", "1.2.3.4:80"); err == nil {
		t.Fatal("want auth failure")
	}
}

func TestSOCKS5DialerRepError(t *testing.T) {
	addr := startFakeSOCKS5(t, socks5Script{repCode: 0x05})
	d := NewSOCKS5Dialer(addr, "", "", 5*time.Second)
	_, err := d.DialContext(context.Background(), "tcp", "example.com:443")
	if err == nil || !strings.Contains(err.Error(), "拒绝") {
		t.Fatalf("want refusal error, got %v", err)
	}
}

// ---------- fake HTTP CONNECT server ----------

func startFakeHTTPProxy(t *testing.T, statusLine string, preamble string, wantAuth string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				br := bufio.NewReader(conn)
				line, err := br.ReadString('\n')
				if err != nil || !strings.HasPrefix(line, "CONNECT ") {
					return
				}
				authed := false
				hostHeaders := 0
				for {
					h, err := br.ReadString('\n')
					if err != nil {
						return
					}
					if h == "\r\n" {
						break
					}
					if strings.HasPrefix(strings.ToLower(h), "host:") {
						hostHeaders++
					}
					if wantAuth != "" && strings.HasPrefix(h, "Proxy-Authorization: Basic "+wantAuth) {
						authed = true
					}
				}
				if hostHeaders != 1 {
					fmt.Fprintf(conn, "HTTP/1.1 400 Bad Request\r\nContent-Length: 0\r\n\r\n")
					return
				}
				if wantAuth != "" && !authed {
					fmt.Fprintf(conn, "HTTP/1.1 407 Proxy Authentication Required\r\nContent-Length: 0\r\n\r\n")
					return
				}
				fmt.Fprintf(conn, "%s\r\n\r\n", statusLine)
				if preamble != "" {
					// bytes that belong to the tunnel, sent immediately so the
					// client's bufio reader likely buffers them with the headers
					conn.Write([]byte(preamble))
				}
				io.Copy(conn, conn)
			}()
		}
	}()
	return ln.Addr().String()
}

func TestHTTPConnectDialer(t *testing.T) {
	addr := startFakeHTTPProxy(t, "HTTP/1.1 200 Connection Established", "TUNNEL-DATA", "")
	d := NewHTTPConnectDialer(addr, "", "", 5*time.Second)
	conn, err := d.DialContext(context.Background(), "tcp", "example.com:443")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	got := make([]byte, len("TUNNEL-DATA"))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read preamble: %v", err)
	}
	if string(got) != "TUNNEL-DATA" {
		t.Fatalf("preamble = %q (buffered bytes lost?)", got)
	}
	// echo still works after the preamble
	if _, err := conn.Write([]byte("hi")); err != nil {
		t.Fatal(err)
	}
	out := make([]byte, 2)
	if _, err := io.ReadFull(conn, out); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPConnectDialerAuth(t *testing.T) {
	// "u:p" base64 = dTpw
	addr := startFakeHTTPProxy(t, "HTTP/1.1 200 OK", "", "dTpw")
	ok := NewHTTPConnectDialer(addr, "u", "p", 5*time.Second)
	conn, err := ok.DialContext(context.Background(), "tcp", "x:1")
	if err != nil {
		t.Fatalf("auth dial: %v", err)
	}
	conn.Close()

	bad := NewHTTPConnectDialer(addr, "u", "nope", 5*time.Second)
	if _, err := bad.DialContext(context.Background(), "tcp", "x:1"); err == nil {
		t.Fatal("want 407 error")
	}
}

func TestServerSetDialerDifferentImplementations(t *testing.T) {
	s := New(Options{Dialer: NewSOCKS5Dialer("127.0.0.1:1080", "", "", time.Second)})

	// atomic.Value panics when an interface changes concrete type. These swaps
	// exercise the live configuration path across every dialer implementation.
	s.SetDialer(NewDirectDialer(time.Second), false)
	s.SetDialer(NewHTTPConnectDialer("127.0.0.1:8080", "", "", time.Second), true)

	holder := s.dialer.Load()
	if holder == nil || holder.dialer == nil {
		t.Fatal("dialer was not stored")
	}
}

// ---------- pipe ----------

func TestPipeHalfClose(t *testing.T) {
	// a <-> pipe <-> b over real TCP so CloseWrite semantics apply.
	mkPair := func(t *testing.T) (end, other *net.TCPConn) {
		t.Helper()
		ln, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1")})
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		type res struct {
			c *net.TCPConn
		}
		ch := make(chan res, 1)
		go func() {
			c, err := ln.AcceptTCP()
			if err == nil {
				ch <- res{c}
			}
		}()
		d, err := net.DialTCP("tcp", nil, ln.Addr().(*net.TCPAddr))
		if err != nil {
			t.Fatal(err)
		}
		accepted := <-ch
		return d, accepted.c
	}

	client, a := mkPair(t)   // client side; a is the relay's client conn
	upstream, b := mkPair(t) // upstream side; b is the relay's upstream conn
	defer client.Close()
	defer upstream.Close()

	tc := NewTracker().Open("1.1.1.1", "h", 80, false)
	done := make(chan struct{})
	go func() { pipe(a, b, tc); close(done) }()

	if _, err := client.Write([]byte("req")); err != nil {
		t.Fatal(err)
	}
	if err := client.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	out := make([]byte, 3)
	if _, err := io.ReadFull(upstream, out); err != nil {
		t.Fatalf("upstream read: %v", err)
	}
	// EOF must propagate to upstream as half-close
	_ = upstream.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := upstream.Read(make([]byte, 1)); err != io.EOF {
		t.Fatalf("upstream want EOF, got %v", err)
	}
	// upstream can still write back
	if _, err := upstream.Write([]byte("resp!")); err != nil {
		t.Fatalf("upstream write after EOF: %v", err)
	}
	buf := make([]byte, 5)
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadFull(client, buf); err != nil {
		t.Fatalf("client read after half-close: %v", err)
	}
	if string(buf) != "resp!" {
		t.Fatalf("client got %q", buf)
	}
	upstream.Close()
	a.Close()
	b.Close()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("pipe did not finish")
	}
	if tc.Up() != 3 || tc.Down() != 5 {
		t.Fatalf("counters up=%d down=%d, want 3/5", tc.Up(), tc.Down())
	}
}

func TestPipeDrainTimeoutStartsAfterFirstDirectionFinishes(t *testing.T) {
	client, a := net.Pipe()
	upstream, b := net.Pipe()
	defer client.Close()
	defer upstream.Close()

	tc := NewTracker().Open("1.1.1.1", "h", 443, true)
	done := make(chan struct{})
	go func() {
		pipeWithDrainTimeout(a, b, tc, 40*time.Millisecond)
		close(done)
	}()

	// The timeout must not cap the total lifetime of a healthy connection.
	time.Sleep(80 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("pipe stopped before either direction reached EOF")
	default:
	}

	_ = client.Close()
	_ = upstream.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pipe did not stop after both directions closed")
	}
}

func TestTrackerConcurrent(t *testing.T) {
	tr := NewTracker()
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := tr.Open("10.0.0.1", "h", 443, true)
			c.AddUp(int64(i))
			c.AddDown(1)
			c.Close()
		}(i)
	}
	wg.Wait()
	snap := tr.Snapshot()
	if len(snap.Active) != 0 {
		t.Fatalf("active = %d after close", len(snap.Active))
	}
	wantUp := int64(64 * 63 / 2)
	if snap.UpTotal != wantUp || snap.DownTotal != 64 {
		t.Fatalf("totals up=%d down=%d, want %d/64", snap.UpTotal, snap.DownTotal, wantUp)
	}
}

// ---------- server end-to-end with fake OrigDST ----------

func TestServerEndToEnd(t *testing.T) {
	// echo target that the fake resolver points at
	echoLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echoLn.Close()
	go func() {
		for {
			c, err := echoLn.Accept()
			if err != nil {
				return
			}
			go io.Copy(c, c)
		}
	}()
	_, echoPort, _ := net.SplitHostPort(echoLn.Addr().String())
	var portNum int
	fmt.Sscanf(echoPort, "%d", &portNum)

	orig := netip.MustParseAddrPort(fmt.Sprintf("127.0.0.1:%d", portNum))
	srv := New(Options{
		ListenAddr: "127.0.0.1:0",
		OrigDST:    OrigDSTFunc(func(c *net.TCPConn) (netip.AddrPort, error) { return orig, nil }),
		Dialer:     NewDirectDialer(2 * time.Second),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.ListenAndServe(ctx)
	deadline := time.Now().Add(3 * time.Second)
	for srv.Addr() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if srv.Addr() == "" {
		t.Fatal("server did not bind")
	}

	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	out := make([]byte, 5)
	if _, err := io.ReadFull(conn, out); err != nil {
		t.Fatalf("relay read: %v", err)
	}
	if string(out) != "hello" {
		t.Fatalf("got %q", out)
	}

	snap := srv.Tracker().Snapshot()
	if len(snap.Active) != 1 || snap.Active[0].DstPort != portNum {
		t.Fatalf("tracker = %+v", snap.Active)
	}
}

func TestServerFakeIPLookup(t *testing.T) {
	echoLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echoLn.Close()
	go func() {
		for {
			c, err := echoLn.Accept()
			if err != nil {
				return
			}
			go io.Copy(c, c)
		}
	}()

	// custom dialer records the address it was asked for and redirects to echo
	var mu sync.Mutex
	var dialed string
	rec := dialerFunc(func(ctx context.Context, network, addr string) (net.Conn, error) {
		mu.Lock()
		dialed = addr
		mu.Unlock()
		var d net.Dialer
		return d.DialContext(ctx, network, echoLn.Addr().String())
	})

	fakeIP := netip.MustParseAddr("198.18.0.7")
	orig := netip.AddrPortFrom(fakeIP, 443)
	prefix := netip.MustParsePrefix("198.18.0.0/16")
	srv := New(Options{
		ListenAddr: "127.0.0.1:0",
		OrigDST:    OrigDSTFunc(func(c *net.TCPConn) (netip.AddrPort, error) { return orig, nil }),
		Dialer:     rec,
	})
	srv.SetFakeIP(&prefix, func(a netip.Addr) (string, bool) {
		if a == fakeIP {
			return "example.com", true
		}
		return "", false
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.ListenAndServe(ctx)
	deadline := time.Now().Add(3 * time.Second)
	for srv.Addr() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	conn.Write([]byte("x"))
	out := make([]byte, 1)
	if _, err := io.ReadFull(conn, out); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if dialed != "example.com:443" {
		t.Fatalf("dialer got %q, want fake-ip domain example.com:443", dialed)
	}
}

func TestServerFakeIPMissingResets(t *testing.T) {
	fakeIP := netip.MustParseAddr("198.18.0.9")
	orig := netip.AddrPortFrom(fakeIP, 443)
	prefix := netip.MustParsePrefix("198.18.0.0/16")
	srv := New(Options{
		ListenAddr: "127.0.0.1:0",
		OrigDST:    OrigDSTFunc(func(c *net.TCPConn) (netip.AddrPort, error) { return orig, nil }),
		Dialer:     NewDirectDialer(time.Second),
	})
	srv.SetFakeIP(&prefix, func(a netip.Addr) (string, bool) { return "", false })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.ListenAndServe(ctx)
	deadline := time.Now().Add(3 * time.Second)
	for srv.Addr() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Fatal("want connection reset on missing fake-ip mapping")
	}
}

func TestMissingFakeIPWarningRateLimit(t *testing.T) {
	srv := New(Options{})
	ip := netip.MustParseAddr("198.18.0.9")
	now := time.Now()
	if !srv.shouldLogMissingFakeIP(ip, now) {
		t.Fatal("first miss should log")
	}
	if srv.shouldLogMissingFakeIP(ip, now.Add(30*time.Second)) {
		t.Fatal("repeated miss inside interval should be suppressed")
	}
	if !srv.shouldLogMissingFakeIP(ip, now.Add(time.Minute)) {
		t.Fatal("miss after interval should log again")
	}
	if !srv.shouldLogMissingFakeIP(netip.MustParseAddr("198.18.0.10"), now) {
		t.Fatal("different fake IP should log independently")
	}
}

type dialerFunc func(ctx context.Context, network, addr string) (net.Conn, error)

func (f dialerFunc) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return f(ctx, network, addr)
}

// ---------- proxy→direct fallback ----------

func startEcho(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go io.Copy(c, c)
		}
	}()
	return ln
}

func serveAndWait(t *testing.T, srv *Server) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.ListenAndServe(ctx)
	deadline := time.Now().Add(3 * time.Second)
	for srv.Addr() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if srv.Addr() == "" {
		t.Fatal("server did not bind")
	}
}

func fallbackTestServer(t *testing.T, echoLn net.Listener, proxy, direct Dialer, rules []RouteRule, learned chan<- string) *Server {
	t.Helper()
	orig := netip.MustParseAddrPort(echoLn.Addr().String())
	echoIP := orig.Addr()
	srv := New(Options{
		ListenAddr: "127.0.0.1:0",
		OrigDST:    OrigDSTFunc(func(c *net.TCPConn) (netip.AddrPort, error) { return orig, nil }),
		Dialer:     proxy,
		ViaProxy:   true,
		OnFallbackSuccess: func(host string) {
			select {
			case learned <- host:
			default:
			}
		},
	})
	// routeHost becomes a domain so the fallback learner callback applies
	srv.SetRealIPLookup(func(ip netip.Addr) (string, bool) {
		if ip == echoIP {
			return "example.com", true
		}
		return "", false
	})
	srv.SetRouting(RouteProxy, direct, proxy, rules)
	serveAndWait(t, srv)
	return srv
}

func TestServerFallbackToDirect(t *testing.T) {
	echoLn := startEcho(t)
	proxy := dialerFunc(func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("proxy boom")
	})
	direct := dialerFunc(func(ctx context.Context, network, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, echoLn.Addr().String())
	})
	learned := make(chan string, 1)
	srv := fallbackTestServer(t, echoLn, proxy, direct, nil, learned)

	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("hi")); err != nil {
		t.Fatal(err)
	}
	out := make([]byte, 2)
	if _, err := io.ReadFull(conn, out); err != nil {
		t.Fatalf("fallback echo read: %v", err)
	}

	select {
	case host := <-learned:
		if host != "example.com" {
			t.Fatalf("learned host = %q, want example.com", host)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("fallback success callback never fired")
	}

	snap := srv.Tracker().Snapshot()
	if len(snap.Active) != 1 {
		t.Fatalf("active = %+v", snap.Active)
	}
	c := snap.Active[0]
	if c.ViaProxy || !c.Fallback || c.DstHost != "example.com" {
		t.Fatalf("fallback conn = %+v, want direct + fallback marked", c)
	}
}

func TestServerNoFallbackWithExplicitProxyRule(t *testing.T) {
	echoLn := startEcho(t)
	proxy := dialerFunc(func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("proxy boom")
	})
	direct := dialerFunc(func(ctx context.Context, network, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, echoLn.Addr().String())
	})
	learned := make(chan string, 1)
	rules := []RouteRule{{Type: "domain-suffix", Value: "example.com", Action: RouteProxy}}
	srv := fallbackTestServer(t, echoLn, proxy, direct, rules, learned)

	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Fatal("explicit proxy rule must not fall back to direct")
	}

	select {
	case host := <-learned:
		t.Fatalf("fallback callback fired for explicit-rule host %q", host)
	case <-time.After(300 * time.Millisecond):
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snap := srv.Tracker().Snapshot()
		if len(snap.Recent) == 1 {
			rec := snap.Recent[0]
			if rec.Status != "dial_failed" || !rec.ViaProxy {
				t.Fatalf("recent = %+v, want proxy dial_failed", rec)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("dial failure never recorded")
}

func TestServerFallbackBothFail(t *testing.T) {
	echoLn := startEcho(t)
	proxy := dialerFunc(func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("proxy boom")
	})
	direct := dialerFunc(func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("direct boom")
	})
	learned := make(chan string, 1)
	srv := fallbackTestServer(t, echoLn, proxy, direct, nil, learned)

	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _ = conn.Read(make([]byte, 1))

	select {
	case host := <-learned:
		t.Fatalf("fallback callback fired on double failure for %q", host)
	case <-time.After(300 * time.Millisecond):
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snap := srv.Tracker().Snapshot()
		if len(snap.Recent) == 1 {
			rec := snap.Recent[0]
			if rec.Status != "dial_failed" || !rec.ViaProxy || rec.Fallback {
				t.Fatalf("recent = %+v, want proxy dial_failed without fallback flag", rec)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("dial failure never recorded")
}
