package httpproxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func startProxy(t *testing.T, c Credentials) (*Server, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := New(func(ctx context.Context, src, target string) (net.Conn, error) {
		return (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "tcp", target)
	}, c)
	done := make(chan error, 1)
	go func() { done <- s.Serve(ln) }()
	t.Cleanup(func() {
		s.Close()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Error("proxy did not stop")
		}
	})
	return s, "http://" + ln.Addr().String()
}
func proxyClient(t *testing.T, address, user, pass string) *http.Client {
	t.Helper()
	u, _ := url.Parse(address)
	if user != "" {
		u.User = url.UserPassword(user, pass)
	}
	tr := &http.Transport{Proxy: http.ProxyURL(u), TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	t.Cleanup(tr.CloseIdleConnections)
	return &http.Client{Transport: tr, Timeout: 3 * time.Second}
}
func TestHTTPAndHTTPSAuthentication(t *testing.T) {
	for _, https := range []bool{false, true} {
		t.Run(fmt.Sprint("https=", https), func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Proxy-Authorization") != "" || r.Header.Get("X-Hop") != "" {
					t.Error("proxy-only header leaked")
				}
				b, _ := io.ReadAll(r.Body)
				fmt.Fprint(w, "origin:"+string(b))
			})
			var origin *httptest.Server
			if https {
				origin = httptest.NewTLSServer(handler)
			} else {
				origin = httptest.NewServer(handler)
			}
			defer origin.Close()
			proxy, addr := startProxy(t, Credentials{Username: "alice", Password: "secret"})
			for _, password := range []string{"", "wrong", "secret"} {
				client := proxyClient(t, addr, "alice", password)
				req, _ := http.NewRequest("POST", origin.URL, strings.NewReader("payload"))
				req.Header.Set("Connection", "X-Hop")
				req.Header.Set("X-Hop", "private")
				// HTTPS tunnel carries origin headers unchanged: hop filtering belongs to
				// the decrypted origin connection, not to CONNECT.
				if https {
					req.Header.Del("X-Hop")
					req.Header.Del("Connection")
				}
				resp, err := client.Do(req)
				if password != "secret" {
					if https {
						if err == nil {
							resp.Body.Close()
							t.Fatal("unauthorized CONNECT succeeded")
						}
						continue
					}
					if err != nil {
						t.Fatal(err)
					}
					resp.Body.Close()
					if resp.StatusCode != 407 || resp.Header.Get("Proxy-Authenticate") == "" {
						t.Fatal("missing challenge")
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				data, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if string(data) != "origin:payload" {
					t.Fatalf("unexpected response %q", data)
				}
			}
			proxy.SetCredentials(Credentials{})
			resp, err := proxyClient(t, addr, "", "").Get(origin.URL)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Fatalf("no-auth mode failed: %d", resp.StatusCode)
			}
		})
	}
}
func TestConnectBufferedDataAndShutdown(t *testing.T) {
	echo, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echo.Close()
	go func() {
		conn, err := echo.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		io.Copy(conn, conn)
	}()
	s, address := startProxy(t, Credentials{})
	conn, err := net.Dial("tcp", strings.TrimPrefix(address, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\nping", echo.Addr(), echo.Addr())
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, &http.Request{Method: "CONNECT"})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 {
		t.Fatal(response.Status)
	}
	data := make([]byte, 4)
	if _, err = io.ReadFull(reader, data); err != nil {
		t.Fatal(err)
	}
	if string(data) != "ping" {
		t.Fatalf("lost buffered tunnel data: %q", data)
	}
	s.Close()
	if _, err = reader.ReadByte(); err == nil {
		t.Fatal("tunnel survived shutdown")
	}
}
func TestRejectPublicClientAndMalformedConnect(t *testing.T) {
	s := New(func(context.Context, string, string) (net.Conn, error) { t.Fatal("must not dial"); return nil, nil }, Credentials{})
	for _, tc := range []struct {
		remote, target string
		status         int
	}{{"8.8.8.8:1234", "example.com:443", 403}, {"192.168.1.4:1234", "example.com:0", 400}} {
		r := httptest.NewRequest("CONNECT", "http://example.com", nil)
		r.Host = tc.target
		r.RemoteAddr = tc.remote
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("got %d", w.Code)
		}
	}
	s.SetCredentials(Credentials{Username: "u", Password: "p"})
	r := httptest.NewRequest("GET", "http://example.com", nil)
	r.RemoteAddr = "192.168.1.4:1234"
	r.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("u:p")))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 407 {
		t.Fatal("origin auth accepted as proxy auth")
	}
}
