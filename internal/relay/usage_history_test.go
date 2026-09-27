package relay

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestHistorySurvivesRestartAndSeparatesDaysAndIngress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	tracker := NewTracker()
	if err := tracker.EnableHistory(path); err != nil {
		t.Fatal(err)
	}
	c := tracker.Open("192.168.1.2", "example.com", 443, true, "tcp", "http-proxy")
	c.AddUp(120)
	c.AddDown(300)
	yesterday := time.Now().AddDate(0, 0, -1)
	tracker.recordUsage(c, 10, 20, 0, yesterday)
	d := tracker.Open("192.168.1.2", "example.com", 443, true, "tcp")
	d.AddDown(50)
	// Save while connections are live; closing must not count bytes twice.
	if err := tracker.SaveHistory(); err != nil {
		t.Fatal(err)
	}
	c.Close()
	d.Close()
	if err := tracker.SaveHistory(); err != nil {
		t.Fatal(err)
	}
	restarted := NewTracker()
	if err := restarted.EnableHistory(path); err != nil {
		t.Fatal(err)
	}
	rows := restarted.UsageHistory()
	if len(rows) != 3 {
		t.Fatalf("rows: %+v", rows)
	}
	var up, down, connections int64
	for _, row := range rows {
		up += row.Up
		down += row.Down
		connections += row.Connections
	}
	if up != 130 || down != 370 || connections != 2 {
		t.Fatalf("incorrect totals %d/%d/%d", up, down, connections)
	}
	fresh := restarted.Open("192.168.1.3", "example.com", 80, false, "tcp")
	fresh.AddDown(7)
	fresh.Close()
	if err := restarted.SaveHistory(); err != nil {
		t.Fatal(err)
	}
	if len(restarted.UsageHistory()) != 4 {
		t.Fatal("lost previous devices")
	}
}

func TestHistoryConcurrentWritesAndCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	tracker := NewTracker()
	if err := tracker.EnableHistory(path); err != nil {
		t.Fatal(err)
	}
	c := tracker.Open("device", "example.com", 443, true, "tcp")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.AddDown(10)
				tracker.UsageHistory()
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 10; j++ {
			if err := tracker.SaveHistory(); err != nil {
				t.Error(err)
			}
		}
	}()
	wg.Wait()
	c.Close()
	if got := tracker.UsageHistory()[0].Down; got != 8000 {
		t.Fatalf("down=%d", got)
	}
	if err := os.WriteFile(path, []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	broken := NewTracker()
	if err := broken.EnableHistory(path); err == nil {
		t.Fatal("accepted corrupt history")
	}
	if err := broken.SaveHistory(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "invalid" {
		t.Fatal("overwrote corrupt history")
	}
}

func TestUsageTracksActualProxyPeerAndDirectSeparately(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	tr := NewTracker()
	if err := tr.EnableHistory(path); err != nil {
		t.Fatal(err)
	}
	proxy := tr.OpenWithEgress("device-a", "Example.COM.", 443, true, "tcp", "127.0.0.1:7897")
	proxy.AddUp(100)
	proxy.AddDown(900)
	direct := tr.OpenWithEgress("device-a", "example.com", 443, false, "tcp", "203.0.113.1:443")
	direct.MarkFallback()
	direct.AddDown(300)
	other := tr.OpenWithEgress("device-a", "example.com", 443, true, "tcp", "127.0.0.1:7898")
	other.AddDown(50)
	deviceB := tr.OpenWithEgress("device-b", "example.com", 443, true, "tcp", "127.0.0.1:7897")
	deviceB.AddDown(40)
	if proxy.info().ProxyEndpoint != "127.0.0.1:7897" || direct.info().ProxyEndpoint != "" {
		t.Fatal("incorrect actual peer")
	}
	if err := tr.SaveHistory(); err != nil {
		t.Fatal(err)
	}
	proxy.Close()
	direct.Close()
	other.Close()
	deviceB.Close()
	if err := tr.SaveHistory(); err != nil {
		t.Fatal(err)
	}
	loaded := NewTracker()
	if err := loaded.EnableHistory(path); err != nil {
		t.Fatal(err)
	}
	rows := loaded.UsageHistory()
	if len(rows) != 4 {
		t.Fatalf("rows %+v", rows)
	}
	for _, r := range rows {
		if r.Destination != "example.com" {
			t.Fatal("domain not normalized")
		}
		if r.Device != "device-a" {
			continue
		}
		switch {
		case r.Egress == "direct":
			if r.Down != 300 || r.ProxyEndpoint != "" {
				t.Fatalf("fallback counted as proxy: %+v", r)
			}
		case r.ProxyEndpoint == "127.0.0.1:7897":
			if r.Up != 100 || r.Down != 900 || r.Connections != 1 {
				t.Fatalf("wrong proxy totals: %+v", r)
			}
		case r.ProxyEndpoint == "127.0.0.1:7898":
			if r.Down != 50 {
				t.Fatal("proxy endpoints merged")
			}
		default:
			t.Fatalf("unknown row %+v", r)
		}
	}
}

func TestLegacyUsageRemainsUnclassified(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	data := `[{"date":"2026-09-28","device":"device-a","ingress":"gateway","up":10,"down":20,"connections":1,"last_seen":"2026-09-28T00:00:00Z"}]`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	tr := NewTracker()
	if err := tr.EnableHistory(path); err != nil {
		t.Fatal(err)
	}
	old := tr.UsageHistory()[0]
	if old.Egress != "" || old.ProxyEndpoint != "" || old.Destination != "" || old.Up+old.Down != 30 {
		t.Fatalf("guessed legacy attribution: %+v", old)
	}
	c := tr.OpenWithEgress("device-a", "example.com", 443, true, "tcp", "127.0.0.1:7897")
	c.AddDown(7)
	c.Close()
	if len(tr.UsageHistory()) != 2 {
		t.Fatal("legacy totals mixed into new proxy usage")
	}
}
