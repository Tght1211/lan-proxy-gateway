package relay

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestResponseRetrySilentTLS(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	// A complete handshake record containing exactly one ClientHello message.
	hello := []byte{22, 3, 1, 0, 5, 1, 0, 0, 1, 0}
	go io.Copy(io.Discard, b)
	called := false
	c := newResponseRetryConn(a, func(ctx context.Context) (net.Conn, error) {
		called = true
		x, y := net.Pipe()
		go func() {
			defer y.Close()
			got := make([]byte, len(hello))
			io.ReadFull(y, got)
			if string(got) != string(hello) {
				t.Error("changed handshake")
			}
			y.Write([]byte("response"))
		}()
		return x, nil
	}, 20*time.Millisecond, nil)
	defer c.Close()
	if _, err := c.Write(hello); err != nil {
		t.Fatal(err)
	}
	out := make([]byte, 8)
	if _, err := io.ReadFull(c, out); err != nil {
		t.Fatal(err)
	}
	if !called || string(out) != "response" {
		t.Fatal("did not retry silent direct connection")
	}
}

func TestResponseRetryDoesNotReplayBusinessData(t *testing.T) {
	for _, request := range []string{"POST /pay HTTP/1.1\r\nContent-Length: 1\r\n\r\nx", string([]byte{23, 3, 3, 0, 1, 0})} {
		a, b := net.Pipe()
		go io.Copy(io.Discard, b)
		c := newResponseRetryConn(a, func(context.Context) (net.Conn, error) { t.Error("unsafe replay"); return nil, io.EOF }, 10*time.Millisecond, nil)
		c.Write([]byte(request))
		buf := make([]byte, 8)
		if _, err := c.Read(buf); err == nil {
			t.Error("expected response timeout")
		}
		c.Close()
		b.Close()
	}
}

func TestResponseRetryKeepsResponsiveDirect(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	go func() { buf := make([]byte, 1); b.Read(buf); b.Write([]byte("ok")) }()
	c := newResponseRetryConn(a, func(context.Context) (net.Conn, error) { t.Error("unneeded retry"); return nil, io.EOF }, time.Second, nil)
	defer c.Close()
	c.Write([]byte("x"))
	buf := make([]byte, 2)
	if _, err := io.ReadFull(c, buf); err != nil {
		t.Fatal(err)
	}
}

func TestResponseRetryBothSilentNotLearned(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	go io.Copy(io.Discard, b)
	result := make(chan bool, 1)
	c := newResponseRetryConn(a, func(ctx context.Context) (net.Conn, error) {
		// Fail without consuming a five-second proxy response budget in this unit test.
		return nil, context.DeadlineExceeded
	}, 10*time.Millisecond, func(proxy, success bool) {
		if !proxy {
			t.Error("expected proxy attempt")
		}
		result <- success
	})
	defer c.Close()
	c.Write([]byte{22, 3, 1, 0, 5, 1, 0, 0, 1, 0})
	buf := make([]byte, 1)
	if _, err := c.Read(buf); err == nil {
		t.Fatal("expected failure")
	}
	if <-result {
		t.Fatal("learned failed attempt")
	}
}

func TestResponseRetryCloseUnblocksWrite(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	c := newResponseRetryConn(a, nil, time.Hour, nil)
	done := make(chan struct{})
	go func() { c.Write([]byte("blocked")); close(done) }()
	time.Sleep(10 * time.Millisecond)
	c.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("close blocked behind write")
	}
}

func TestExplicitSilentTLSFallbackTracksAndLearns(t *testing.T) {
	learned := make(chan string, 1)
	s := New(Options{OnProxyFallbackSuccess: func(host string) { learned <- host }})
	hello := []byte{22, 3, 1, 0, 5, 1, 0, 0, 1, 0}
	direct := dialerFunc(func(context.Context, string, string) (net.Conn, error) {
		a, b := net.Pipe()
		go func() { defer b.Close(); io.Copy(io.Discard, b) }()
		return a, nil
	})
	proxy := dialerFunc(func(context.Context, string, string) (net.Conn, error) {
		a, b := net.Pipe()
		go func() { defer b.Close(); buf := make([]byte, len(hello)); io.ReadFull(b, buf); b.Write([]byte("ok")) }()
		return a, nil
	})
	s.SetRouting("proxy", direct, proxy, nil)
	// Shorten only this host's budget; production starts at five seconds.
	s.responses = map[string]responseState{"example.com": {timeout: 20 * time.Millisecond, expires: time.Now().Add(time.Minute)}}
	c, err := s.DialExplicit(context.Background(), "192.168.2.3", "example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	c.Write(hello)
	buf := make([]byte, 2)
	if _, err := io.ReadFull(c, buf); err != nil {
		t.Fatal(err)
	}
	c.Close()
	select {
	case host := <-learned:
		if host != "example.com" {
			t.Fatal(host)
		}
	case <-time.After(time.Second):
		t.Fatal("missing learning")
	}
	rows := s.Tracker().Snapshot().Recent
	if len(rows) != 1 || !rows[0].ViaProxy || !rows[0].Fallback || rows[0].Down != 2 {
		t.Fatal(rows)
	}
}

func TestResponseRetryRealTLSHandshake(t *testing.T) {
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ready")) }))
	defer origin.Close()
	a, b := net.Pipe()
	defer b.Close()
	go io.Copy(io.Discard, b)
	c := newResponseRetryConn(a, func(ctx context.Context) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", origin.Listener.Addr().String())
	}, 20*time.Millisecond, nil)
	// Trust only this local test server's generated certificate.
	roots := x509.NewCertPool()
	roots.AddCert(origin.Certificate())
	client := tls.Client(c, &tls.Config{RootCAs: roots, ServerName: "example.com"})
	defer client.Close()
	if err := client.Handshake(); err != nil {
		t.Fatal(err)
	}
}

func TestTransparentSilentTLSFallback(t *testing.T) {
	origin := startEcho(t)
	direct := dialerFunc(func(context.Context, string, string) (net.Conn, error) {
		a, b := net.Pipe()
		go func() { defer b.Close(); io.Copy(io.Discard, b) }()
		return a, nil
	})
	proxy := dialerFunc(func(ctx context.Context, network, target string) (net.Conn, error) {
		if target != "example.com:443" && !strings.HasPrefix(target, "example.com:") {
			t.Error("lost domain", target)
		}
		return (&net.Dialer{}).DialContext(ctx, network, origin.Addr().String())
	})
	s := fallbackTestServer(t, origin, proxy, direct, nil, make(chan string, 1))
	s.responseMu.Lock()
	s.responses = map[string]responseState{"example.com": {timeout: 20 * time.Millisecond, expires: time.Now().Add(time.Minute)}}
	s.responseMu.Unlock()
	c, err := net.Dial("tcp", s.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(time.Second))
	hello := []byte{22, 3, 1, 0, 5, 1, 0, 0, 1, 0}
	c.Write(hello)
	buf := make([]byte, len(hello))
	if _, err := io.ReadFull(c, buf); err != nil {
		t.Fatal(err)
	}
	rows := s.Tracker().Snapshot().Active
	if len(rows) != 1 || !rows[0].ViaProxy || !rows[0].Fallback {
		t.Fatal(rows)
	}
}
