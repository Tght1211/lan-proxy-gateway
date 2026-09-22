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
