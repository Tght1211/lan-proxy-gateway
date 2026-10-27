package relay

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"
)

func TestEgressMonitorSwitchDialerTypes(t *testing.T) {
	m := newEgressMonitor(newProxyHealth())
	m.setProxy("127.0.0.1:7897", NewHTTPConnectDialer("127.0.0.1:7897", "", "", time.Second))
	m.setProxy("127.0.0.1:7897", NewSOCKS5Dialer("127.0.0.1:7897", "", "", time.Second))
	m.setProxy("", NewDirectDialer(time.Second))
}

func TestEgressMonitorPortRecoveryWhileHostProbeBlocked(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	started := make(chan struct{})
	var once sync.Once
	m := newEgressMonitor(newProxyHealth())
	m.setProxy(ln.Addr().String(), dialerFunc(func(ctx context.Context, _, _ string) (net.Conn, error) {
		once.Do(func() { close(started) })
		<-ctx.Done()
		return nil, ctx.Err()
	}))
	m.down.Store(true)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.runLoopWithIntervals(ctx, func() []string { return []string{"slow.example"} }, nil, 20*time.Millisecond, time.Millisecond)
	}()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("monitor did not stop promptly")
		}
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("host probe never started")
	}
	deadline := time.Now().Add(time.Second)
	for m.isDown() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if m.isDown() {
		t.Fatal("slow host probe blocked local proxy recovery")
	}
}

func TestEgressMonitorSnapshotConcurrentOutage(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	m := newEgressMonitor(newProxyHealth())
	m.setProxy(addr, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.runLoopWithIntervals(ctx, func() []string { return nil }, nil, time.Millisecond, time.Hour)
	}()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		s := m.snapshot()
		if s.ProxyDown && !s.Since.IsZero() {
			return
		}
	}
	t.Fatal("outage not detected")
}
