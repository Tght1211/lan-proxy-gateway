package relay

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestExplicitProxyRoutingAndTracking(t *testing.T) {
	var directCalls, proxyCalls int
	dial := func(counter *int) Dialer {
		return dialerFunc(func(ctx context.Context, network, target string) (net.Conn, error) {
			*counter++
			if target != "example.com:443" {
				t.Errorf("domain was not preserved: %s", target)
			}
			client, server := net.Pipe()
			go func() { defer server.Close(); server.Write([]byte("ok")) }()
			return client, nil
		})
	}
	s := New(Options{})
	direct, proxy := dial(&directCalls), dial(&proxyCalls)
	s.SetRouting("proxy", direct, proxy, nil)
	conn, err := s.DialExplicit(context.Background(), "192.168.1.2", "example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 2)
	conn.Read(buffer)
	conn.Close()
	if proxyCalls != 0 || directCalls != 1 {
		t.Fatal("wrong default route")
	}
	snapshot := s.Tracker().Snapshot()
	if len(snapshot.Recent) != 1 || snapshot.Recent[0].Ingress != "http-proxy" || snapshot.Recent[0].Down != 2 || snapshot.Recent[0].ViaProxy {
		t.Fatalf("tracking failed: %+v", snapshot.Recent)
	}
	s.SetRouting("proxy", direct, proxy, []RouteRule{{Type: "src-ip", Value: "192.168.1.2", Action: "direct"}})
	conn, err = s.DialExplicit(context.Background(), "192.168.1.2", "example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	if directCalls != 2 {
		t.Fatal("device override ignored")
	}
	s.SetRouting("proxy", direct, proxy, []RouteRule{{Type: "domain", Value: "example.com", Action: "reject"}})
	if _, err = s.DialExplicit(context.Background(), "192.168.1.2", "example.com:443"); err == nil {
		t.Fatal("reject ignored")
	}
	if s.Tracker().Snapshot().Recent[0].Ingress != "http-proxy" {
		t.Fatal("rejected ingress missing")
	}
	if directCalls != 2 || proxyCalls != 0 {
		t.Fatal("rejected request dialed out")
	}
}
func TestExplicitProxyFailurePolicy(t *testing.T) {
	failing := dialerFunc(func(context.Context, string, string) (net.Conn, error) { return nil, errors.New("offline") })
	directCalls := 0
	direct := dialerFunc(func(context.Context, string, string) (net.Conn, error) {
		directCalls++
		a, b := net.Pipe()
		b.Close()
		return a, nil
	})
	s := New(Options{})
	s.SetRouting("proxy", direct, failing, nil)
	s.SetProxyFailAction("reject")
	if _, err := s.DialExplicit(context.Background(), "192.168.1.2", "example.com:443"); err == nil {
		t.Fatal("fail-closed ignored")
	}
	if s.Tracker().Snapshot().Recent[0].Ingress != "http-proxy" {
		t.Fatal("failed ingress missing")
	}
	if directCalls != 0 {
		t.Fatal("unexpected direct fallback")
	}
	s.SetProxyFailAction("direct")
	conn, err := s.DialExplicit(context.Background(), "192.168.1.2", "example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	if directCalls != 1 {
		t.Fatal("fallback not applied")
	}
}

func TestExplicitTimeoutFallsBackWithFreshBudget(t *testing.T) {
	directCalls := 0
	stalled := dialerFunc(func(ctx context.Context, _, _ string) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	direct := dialerFunc(func(ctx context.Context, _, _ string) (net.Conn, error) {
		directCalls++
		if err := ctx.Err(); err != nil {
			t.Errorf("fallback inherited expired context: %v", err)
		}
		a, b := net.Pipe()
		b.Close()
		return a, nil
	})
	s := New(Options{})
	s.SetRouting("proxy", stalled, direct, nil)
	s.SetProxyFailAction("direct")
	conn, err := s.dialExplicit(context.Background(), "192.168.1.20", "example.com:443", 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	if directCalls != 1 {
		t.Fatalf("direct calls=%d", directCalls)
	}
	if recent := s.Tracker().Snapshot().Recent; len(recent) != 1 || !recent[0].Fallback || !recent[0].ViaProxy {
		t.Fatalf("tracking: %+v", recent)
	}
}

func TestExplicitCallerCancellationPreventsFallback(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(fmt.Sprint("deadline=", deadline), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			if deadline {
				ctx, cancel = context.WithTimeout(context.Background(), 10*time.Millisecond)
			}
			defer cancel()
			stalled := dialerFunc(func(ctx context.Context, _, _ string) (net.Conn, error) {
				if !deadline {
					cancel()
				}
				<-ctx.Done()
				return nil, ctx.Err()
			})
			direct := dialerFunc(func(context.Context, string, string) (net.Conn, error) {
				t.Error("fallback after caller cancellation")
				return nil, errors.New("unexpected dial")
			})
			s := New(Options{})
			s.SetRouting("proxy", stalled, direct, nil)
			s.SetProxyFailAction("direct")
			if _, err := s.dialExplicit(ctx, "192.168.1.20", "example.com:443", time.Second); !errors.Is(err, ctx.Err()) {
				t.Fatalf("got %v; want %v", err, ctx.Err())
			}
		})
	}
}

func TestDirectFailureLearnsProxyOnlyAfterResponse(t *testing.T) {
	for _, response := range []bool{false, true} {
		t.Run(fmt.Sprint(response), func(t *testing.T) {
			learned := make(chan string, 2)
			s := New(Options{OnProxyFallbackSuccess: func(host string) { learned <- host }})
			order := ""
			direct := dialerFunc(func(context.Context, string, string) (net.Conn, error) {
				order += "d"
				return nil, errors.New("unreachable")
			})
			proxy := dialerFunc(func(context.Context, string, string) (net.Conn, error) {
				order += "p"
				a, b := net.Pipe()
				go func() {
					defer b.Close()
					if response {
						b.Write([]byte("ok"))
					}
				}()
				return a, nil
			})
			s.SetRouting("proxy", direct, proxy, nil)
			c, err := s.DialExplicit(context.Background(), "192.168.1.2", "unknown.example:443")
			if err != nil {
				t.Fatal(err)
			}
			select {
			case <-learned:
				t.Fatal("learned from dial alone")
			default:
			}
			c.Read(make([]byte, 10))
			c.Close()
			if order != "dp" {
				t.Fatalf("order %s", order)
			}
			select {
			case host := <-learned:
				if !response || host != "unknown.example" {
					t.Fatalf("unexpected learning: %s", host)
				}
			case <-time.After(100 * time.Millisecond):
				if response {
					t.Fatal("missing learning")
				}
			}
			recent := s.Tracker().Snapshot().Recent
			if len(recent) != 1 || !recent[0].ViaProxy || !recent[0].Fallback {
				t.Fatalf("wrong route telemetry: %+v", recent)
			}
		})
	}
}

func TestExplicitFallbackUsesProxyBudget(t *testing.T) {
	failing := dialerFunc(func(context.Context, string, string) (net.Conn, error) { return nil, errors.New("direct unavailable") })
	proxy := dialerFunc(func(ctx context.Context, _, _ string) (net.Conn, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) < 3*time.Second {
			t.Errorf("proxy received direct timeout instead of configured budget: %v", time.Until(deadline))
		}
		a, b := net.Pipe()
		b.Close()
		return a, nil
	})
	s := New(Options{ResponsePolicy: func() ResponsePolicy { return ResponsePolicy{DirectWait: time.Second, ProxyWait: 4 * time.Second} }})
	s.SetRouting("proxy", failing, proxy, nil)
	c, err := s.DialExplicit(context.Background(), "192.168.2.5", "example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
}
