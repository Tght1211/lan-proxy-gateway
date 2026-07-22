package relay

import (
	"context"
	"testing"
	"time"
)

func TestTrackerArchivesAndAggregates(t *testing.T) {
	tr := NewTracker()
	c := tr.Open("192.168.1.20", "r1---sn.googlevideo.com", 443, true)
	c.AddUp(100)
	c.AddDown(900)
	c.Close()

	snap := tr.Snapshot()
	if len(snap.Active) != 0 || len(snap.Recent) != 1 {
		t.Fatalf("active=%d recent=%d", len(snap.Active), len(snap.Recent))
	}
	if snap.Recent[0].Service != "YouTube" || snap.Recent[0].EndedAt == nil {
		t.Fatalf("recent = %+v", snap.Recent[0])
	}
	if len(snap.Devices) != 1 || snap.Devices[0].Down != 900 {
		t.Fatalf("devices = %+v", snap.Devices)
	}
	if len(snap.Services) != 1 || snap.Services[0].Name != "YouTube" {
		t.Fatalf("services = %+v", snap.Services)
	}
}

func TestTrackerSnapshotIncludesActiveConnectionsInAggregates(t *testing.T) {
	tr := NewTracker()
	c := tr.Open("192.168.1.30", "www.youtube.com", 443, true)
	c.AddUp(120)
	c.AddDown(880)

	snap := tr.Snapshot()
	if len(snap.Active) != 1 {
		t.Fatalf("active=%d, want 1", len(snap.Active))
	}
	if len(snap.Devices) != 1 || snap.Devices[0].Name != "192.168.1.30" || snap.Devices[0].Down != 880 {
		t.Fatalf("devices = %+v", snap.Devices)
	}
	if len(snap.Services) != 1 || snap.Services[0].Name != "YouTube" || snap.Services[0].Connections != 1 {
		t.Fatalf("services = %+v", snap.Services)
	}

	c.Close()
	snap = tr.Snapshot()
	if len(snap.Devices) != 1 || snap.Devices[0].Connections != 1 || snap.Devices[0].Down != 880 {
		t.Fatalf("completed device aggregate counted incorrectly: %+v", snap.Devices)
	}
}

func TestTrackerSamplesTraffic(t *testing.T) {
	tr := NewTracker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tr.StartSampling(ctx, 5*time.Millisecond)
	c := tr.Open("192.168.1.20", "example.com", 443, false)
	c.AddDown(512)
	time.Sleep(12 * time.Millisecond)
	c.Close()

	snap := tr.Snapshot()
	if len(snap.Traffic) == 0 {
		t.Fatal("expected traffic samples")
	}
	var down int64
	for _, point := range snap.Traffic {
		down += point.Down
	}
	if down != 512 {
		t.Fatalf("sampled down = %d, want 512", down)
	}
}

func TestClassifyService(t *testing.T) {
	tests := map[string]string{
		"www.youtube.com":          "YouTube",
		"video.nflxvideo.net":      "Netflix",
		"api.github.com":           "GitHub",
		"assets.example.org":       "example.org",
		"203.0.113.10":             "IP 地址流量",
		"r1---sn.googlevideo.com.": "YouTube",
		"sns-img-qc.xhscdn.com":    "小红书",
		"v3-dy-o.zjcdn.com":        "zjcdn.com",
		"asset.example.co.uk":      "example.co.uk",
	}
	for host, want := range tests {
		if got := classifyService(host); got != want {
			t.Errorf("classifyService(%q) = %q, want %q", host, got, want)
		}
	}
}
