package app

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestHealthSnapshotMetrics(t *testing.T) {
	h := &healthState{healthy: true}
	h.record(nil, 20*time.Millisecond)
	h.record(nil, 30*time.Millisecond)
	h.record(errors.New("timeout"), 8*time.Second)

	snap := h.snapshot()
	if snap.Healthy {
		t.Fatal("latest failed probe should be unhealthy")
	}
	if snap.LatencyMS != 25 || snap.JitterMS != 10 {
		t.Fatalf("latency=%v jitter=%v", snap.LatencyMS, snap.JitterMS)
	}
	if snap.Availability < 66 || snap.Availability > 67 {
		t.Fatalf("availability=%v", snap.Availability)
	}
	if len(snap.History) != 3 {
		t.Fatalf("history=%d", len(snap.History))
	}
}

type healthTestDialer struct {
	dial func(context.Context, string, string) (net.Conn, error)
}

func (dialer healthTestDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return dialer.dial(ctx, network, address)
}

func TestDirectProbeUsesBaiduAndKeepsProxyHealthSeparate(t *testing.T) {
	application := &App{
		Cfg:    &config.Config{Egress: config.EgressConfig{Mode: config.EgressProxy}},
		health: &healthState{}, directHealth: &healthState{},
	}
	proxyError := errors.New("proxy unavailable")
	proxyDialer := healthTestDialer{dial: func(context.Context, string, string) (net.Conn, error) {
		return nil, proxyError
	}}
	if probeHealth(context.Background(), proxyDialer, probeTarget, application.health) {
		t.Fatal("failed proxy probe should not report success")
	}
	requests := make(chan *http.Request, 1)
	directDialer := healthTestDialer{dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" || address != "www.baidu.com:80" {
			t.Fatalf("unexpected direct probe destination: %s %s", network, address)
		}
		client, server := net.Pipe()
		go func() {
			defer server.Close()
			server.SetDeadline(time.Now().Add(time.Second))
			request, err := http.ReadRequest(bufio.NewReader(server))
			if err == nil {
				requests <- request
				server.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"))
			}
		}()
		return client, nil
	}}
	if !probeHealth(context.Background(), directDialer, directProbeTarget, application.directHealth) {
		t.Fatal("Baidu response should mark only direct health as healthy")
	}
	request := <-requests
	if request.Method != http.MethodGet || request.Host != "www.baidu.com" || request.URL.Path != "/" {
		t.Fatalf("unexpected probe request: %s %s %s", request.Method, request.Host, request.URL.Path)
	}
	snapshots := application.ExitHealth()
	if !snapshots["direct"].Healthy || snapshots["proxy"].Healthy || len(snapshots["direct"].History) != 1 || len(snapshots["proxy"].History) != 1 {
		t.Fatalf("exit probe results mixed: %+v", snapshots)
	}
	if snapshots["direct"].Target != directProbeTarget || snapshots["proxy"].Target != probeTarget {
		t.Fatal("probe target metadata must distinguish exits")
	}
	if application.Health().Healthy {
		t.Fatal("legacy primary health must still reflect the selected proxy")
	}
	application.setCfg(&config.Config{Egress: config.EgressConfig{Mode: config.EgressDirect}})
	if !application.Health().Healthy || application.Health().Target != directProbeTarget {
		t.Fatal("direct mode primary health must use Baidu probe history")
	}
	if _, exists := application.ExitHealth()["proxy"]; exists {
		t.Fatal("inactive proxy must not expose old health samples")
	}
}

func TestProbeHealthCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	state := &healthState{}
	dialer := healthTestDialer{dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	if probeHealth(ctx, dialer, directProbeTarget, state) {
		t.Fatal("cancelled direct probe must fail")
	}
	if snapshot := state.snapshot(); snapshot.Healthy || snapshot.FailCount != 1 || len(snapshot.History) != 1 {
		t.Fatalf("cancelled probe snapshot: %+v", snapshot)
	}
}
