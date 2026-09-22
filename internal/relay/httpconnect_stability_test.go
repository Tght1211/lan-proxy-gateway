package relay

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

func TestHTTPConnectBufferedHalfClose(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := make(chan error, 1)
	go func() {
		c, e := ln.Accept()
		if e != nil {
			done <- e
			return
		}
		defer c.Close()
		r := bufio.NewReader(c)
		for {
			line, e := r.ReadString('\n')
			if e != nil {
				done <- e
				return
			}
			if line == "\r\n" {
				break
			}
		}
		fmt.Fprint(c, "HTTP/1.1 200 Connection Established\r\n\r\nready")
		data, e := io.ReadAll(r)
		if e != nil {
			done <- e
			return
		}
		_, e = fmt.Fprintf(c, "reply:%s", data)
		done <- e
	}()
	d := NewHTTPConnectDialer(ln.Addr().String(), "", "", time.Second)
	c, err := d.DialContext(context.Background(), "tcp", "example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(time.Second))
	ready := make([]byte, 5)
	if _, err = io.ReadFull(c, ready); err != nil {
		t.Fatal(err)
	}
	wrapped := &explicitConn{Conn: c, tracked: NewTracker().Open("client", "host", 443, true, "tcp", "http-proxy")}
	fmt.Fprint(wrapped, "request")
	if err = wrapped.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(wrapped)
	if err != nil || string(data) != "reply:request" {
		t.Fatalf("half-close lost response: %q, %v", data, err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}

func TestUpstreamHandshakeHonorsCancellation(t *testing.T) {
	for _, kind := range []string{"http", "socks5"} {
		t.Run(kind, func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer ln.Close()
			accepted := make(chan net.Conn, 1)
			go func() {
				c, e := ln.Accept()
				if e == nil {
					accepted <- c
				}
			}()
			var d Dialer = NewHTTPConnectDialer(ln.Addr().String(), "", "", 2*time.Second)
			if kind == "socks5" {
				d = NewSOCKS5Dialer(ln.Addr().String(), "", "", 2*time.Second)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() {
				c, e := d.DialContext(ctx, "tcp", "example.com:443")
				if c != nil {
					c.Close()
				}
				result <- e
			}()
			c := <-accepted
			defer c.Close()
			cancel()
			select {
			case e := <-result:
				if e == nil {
					t.Fatal("canceled dial succeeded")
				}
			case <-time.After(300 * time.Millisecond):
				t.Fatal("handshake ignored context cancellation")
			}
		})
	}
}

func TestHTTPConnectRejectionDoesNotDrainUnfinishedBody(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	release := make(chan struct{})
	defer close(release)
	go func() {
		c, e := ln.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		r := bufio.NewReader(c)
		for {
			line, e := r.ReadString('\n')
			if e != nil {
				return
			}
			if line == "\r\n" {
				break
			}
		}
		fmt.Fprint(c, "HTTP/1.1 407 Proxy Authentication Required\r\nContent-Length: 100\r\n\r\nx")
		<-release
	}()
	start := time.Now()
	c, err := NewHTTPConnectDialer(ln.Addr().String(), "", "", 2*time.Second).DialContext(context.Background(), "tcp", "example.com:443")
	if c != nil {
		c.Close()
	}
	if err == nil {
		t.Fatal("rejection accepted")
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("waited for unfinished rejection body")
	}
}
