package relay

import "testing"

func TestIngressUsageSurvivesCloseWithoutDoubleCounting(t *testing.T) {
	tracker := NewTracker()
	gateway := tracker.Open("192.168.1.2", "example.com", 443, true, "tcp")
	http := tracker.Open("192.168.1.2", "example.com", 443, true, "tcp", "http-proxy")
	gateway.AddUp(10)
	http.AddUp(20)
	http.AddDown(30)
	check := func() {
		t.Helper()
		s := tracker.Snapshot()
		if s.UpTotal != 30 || s.DownTotal != 30 || len(s.Ingress) != 2 {
			t.Fatalf("invalid totals: %+v", s)
		}
		for _, usage := range s.Ingress {
			wantUp, wantDown := int64(10), int64(0)
			if usage.Name == "http-proxy" {
				wantUp, wantDown = 20, 30
			}
			if usage.Up != wantUp || usage.Down != wantDown {
				t.Fatalf("invalid ingress usage: %+v", usage)
			}
		}
	}
	check()
	http.Close()
	http.Close()
	check()
	gateway.Close()
	check()
	for _, info := range tracker.Snapshot().Recent {
		if info.Up == 20 && info.Ingress != "http-proxy" {
			t.Fatal("HTTP ingress lost on close")
		}
		if info.Up == 10 && info.Ingress != "gateway" {
			t.Fatal("default gateway ingress missing")
		}
	}
}
