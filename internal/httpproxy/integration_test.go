package httpproxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/relay"
)

// Exercise the actual chain: authenticated LAN proxy -> tracked routing ->
// authenticated upstream HTTP CONNECT proxy -> HTTPS origin.
func TestLayeredProxyConcurrentTransfersAndRecovery(t *testing.T) {
	payload := bytes.Repeat([]byte("proxy-stream-integrity\n"), 8192)
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		if !bytes.Equal(data, payload) {
			t.Error("upload corrupted")
		}
		w.Write(payload)
	}))
	defer origin.Close()
	_, upstream := startProxy(t, Credentials{Username: "upstream", Password: "upstream-secret"})
	routing := relay.New(relay.Options{})
	good := relay.NewHTTPConnectDialer(strings.TrimPrefix(upstream, "http://"), "upstream", "upstream-secret", time.Second)
	routing.SetRouting("proxy", relay.NewDirectDialer(time.Second), good, nil)
	routing.SetProxyFailAction("reject")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := New(routing.DialExplicit, Credentials{Username: "client", Password: "client-secret"})
	go server.Serve(listener)
	defer server.Close()
	client := proxyClient(t, "http://"+listener.Addr().String(), "client", "client-secret")
	client.Timeout = 5 * time.Second
	transfer := func() error {
		response, err := client.Post(origin.URL, "application/octet-stream", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		data, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			return err
		}
		if response.StatusCode != 200 || sha256.Sum256(data) != sha256.Sum256(payload) {
			return fmt.Errorf("corrupt response: status=%d bytes=%d", response.StatusCode, len(data))
		}
		return nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := transfer(); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	// Close pooled tunnels so the next request must negotiate with the new route.
	client.CloseIdleConnections()
	routing.SetRouting("proxy", relay.NewDirectDialer(time.Second), failedUpstream{}, nil)
	if err := transfer(); err == nil {
		t.Fatal("unavailable upstream unexpectedly succeeded")
	}
	routing.SetRouting("proxy", relay.NewDirectDialer(time.Second), good, nil)
	if err := transfer(); err != nil {
		t.Fatalf("did not recover without listener restart: %v", err)
	}
	client.CloseIdleConnections()
	deadline := time.Now().Add(time.Second)
	for len(routing.Tracker().Snapshot().Active) > 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	stats := routing.Tracker().Snapshot()
	if len(stats.Active) != 0 {
		t.Fatal("completed connections leaked")
	}
	if stats.UpTotal < int64(17*len(payload)) || stats.DownTotal < int64(17*len(payload)) {
		t.Fatal("traffic accounting lost transfers")
	}
}

type failedUpstream struct{}

func (failedUpstream) DialContext(context.Context, string, string) (net.Conn, error) {
	return nil, fmt.Errorf("upstream offline")
}
