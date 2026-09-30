package relay

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// Read the public payload so this regression also covers the API contract.
func deviceTrafficTime(t *testing.T, tracker *Tracker) *time.Time {
	t.Helper()
	data, err := json.Marshal(tracker.Snapshot().Devices)
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		LastTrafficAt *time.Time `json:"last_traffic_at"`
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("device rows = %d, want 1", len(rows))
	}
	return rows[0].LastTrafficAt
}

func TestTrackerDeviceActivityUsesBytesNotConnectionLifecycle(t *testing.T) {
	tr := NewTracker()
	c := tr.Open("192.168.2.5", "example.com", 443, false, "tcp")
	c.startedAt = time.Now().Add(-48 * time.Hour)
	if got := deviceTrafficTime(t, tr); got != nil {
		t.Fatal("opening an idle connection counts as traffic")
	}
	before := time.Now()
	c.AddUp(10)
	first := deviceTrafficTime(t, tr)
	if first == nil || first.Before(before) {
		t.Fatal("actual bytes must report a current last_traffic_at")
	}
	c.AddUp(0)
	c.AddDown(0)
	if got := deviceTrafficTime(t, tr); got == nil || !got.Equal(*first) {
		t.Fatal("zero bytes or polling refreshed activity")
	}
	c.Close()
	if got := deviceTrafficTime(t, tr); got == nil || !got.Equal(*first) {
		t.Fatal("closing an idle connection refreshed activity")
	}
	second := tr.Open("192.168.2.5", "other.example", 443, true, "tcp")
	second.AddDown(40)
	latest := deviceTrafficTime(t, tr)
	if latest == nil || !latest.After(*first) {
		t.Fatal("new downstream bytes must reactivate a device")
	}
}

func TestTrackerArchivesAndAggregates(t *testing.T) {
	tr := NewTracker()
	c := tr.Open("192.168.1.20", "r1---sn.googlevideo.com", 443, true, "tcp")
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
	if len(snap.DeviceServices) != 1 || snap.DeviceServices[0].Device != "192.168.1.20" ||
		len(snap.DeviceServices[0].Services) != 1 || snap.DeviceServices[0].Services[0].Name != "YouTube" {
		t.Fatalf("device services = %+v", snap.DeviceServices)
	}
}

func TestTrackerSnapshotIncludesActiveConnectionsInAggregates(t *testing.T) {
	tr := NewTracker()
	c := tr.Open("192.168.1.30", "www.youtube.com", 443, true, "tcp")
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
	if got := snap.DeviceServices[0].Services[0].Down; got != 880 {
		t.Fatalf("active device service down = %d, want 880", got)
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
	c := tr.Open("192.168.1.20", "example.com", 443, false, "tcp")
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
		"203.0.113.10":             "未解析域名",
		"r1---sn.googlevideo.com.": "YouTube",
		"sns-img-qc.xhscdn.com":    "小红书",
		"v3-dy-o.zjcdn.com":        "zjcdn.com",
		"asset.example.co.uk":      "example.co.uk",
	}
	for host, want := range tests {
		if got := ClassifyService(host); got != want {
			t.Errorf("ClassifyService(%q) = %q, want %q", host, got, want)
		}
	}
}
