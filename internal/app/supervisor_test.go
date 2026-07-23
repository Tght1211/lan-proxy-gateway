package app

import (
	"errors"
	"testing"
	"time"
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
