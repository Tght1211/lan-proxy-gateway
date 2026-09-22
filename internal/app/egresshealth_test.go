package app

import (
	"testing"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/relay"
)

func failedConn(device, host string, endedAgo time.Duration) relay.ConnInfo {
	at := time.Now().Add(-endedAgo)
	return relay.ConnInfo{
		SrcIP: device, DstHost: host,
		Status: "dial_failed", Failure: "连接超时",
		StartedAt: at, EndedAt: &at,
	}
}

func okConn(device, host string, endedAgo time.Duration) relay.ConnInfo {
	at := time.Now().Add(-endedAgo)
	return relay.ConnInfo{
		SrcIP: device, DstHost: host,
		StartedAt: at.Add(-time.Second), EndedAt: &at,
	}
}

func TestEgressHealthSingleFailureDoesNotAlert(t *testing.T) {
	out := buildEgressHealthJSON(relay.EgressSnapshot{}, []relay.ConnInfo{
		failedConn("192.168.1.10", "stats.example.com", time.Minute),
	}, nil)
	if len(out.DirectFailures) != 0 {
		t.Fatalf("single failure should not alert, got %+v", out.DirectFailures)
	}
	if len(out.FailureStats) != 1 || out.FailureStats[0].Count != 1 || out.FailureStats[0].Alerting {
		t.Fatalf("expected one non-alerting stat, got %+v", out.FailureStats)
	}
}

func TestEgressHealthThresholdAlerts(t *testing.T) {
	var recent []relay.ConnInfo
	for i := 0; i < egressAlertThreshold; i++ {
		recent = append(recent, failedConn("192.168.1.10", "bad.example.com", time.Duration(i+1)*time.Minute))
	}
	out := buildEgressHealthJSON(relay.EgressSnapshot{}, recent, nil)
	if len(out.DirectFailures) != 1 {
		t.Fatalf("expected 1 alert, got %+v", out.DirectFailures)
	}
	df := out.DirectFailures[0]
	if df.Count != egressAlertThreshold || df.Host != "bad.example.com" || df.Reason != "连接超时" {
		t.Fatalf("unexpected alert %+v", df)
	}
	if len(out.FailureStats) != 1 || !out.FailureStats[0].Alerting {
		t.Fatalf("stat should be alerting, got %+v", out.FailureStats)
	}
}

func TestEgressHealthRecentSuccessSuppresses(t *testing.T) {
	recent := []relay.ConnInfo{
		failedConn("192.168.1.10", "flaky.example.com", 4*time.Minute),
		failedConn("192.168.1.10", "flaky.example.com", 3*time.Minute),
		failedConn("192.168.1.10", "flaky.example.com", 2*time.Minute),
		okConn("192.168.1.10", "flaky.example.com", time.Minute),
	}
	out := buildEgressHealthJSON(relay.EgressSnapshot{}, recent, nil)
	if len(out.DirectFailures) != 0 {
		t.Fatalf("success after failures should suppress alert, got %+v", out.DirectFailures)
	}
	if len(out.FailureStats) != 1 || !out.FailureStats[0].Suppressed {
		t.Fatalf("stat should be suppressed, got %+v", out.FailureStats)
	}
}

func TestEgressHealthActiveConnSuppresses(t *testing.T) {
	recent := []relay.ConnInfo{
		failedConn("192.168.1.10", "up-again.example.com", 3*time.Minute),
		failedConn("192.168.1.10", "up-again.example.com", 2*time.Minute),
		failedConn("192.168.1.10", "up-again.example.com", time.Minute),
	}
	active := []relay.ConnInfo{{
		SrcIP: "192.168.1.10", DstHost: "up-again.example.com",
		StartedAt: time.Now().Add(-30 * time.Second),
	}}
	out := buildEgressHealthJSON(relay.EgressSnapshot{}, recent, active)
	if len(out.DirectFailures) != 0 {
		t.Fatalf("active connection should suppress alert, got %+v", out.DirectFailures)
	}
}

func TestEgressHealthOldFailuresOutsideAlertWindow(t *testing.T) {
	recent := []relay.ConnInfo{
		failedConn("192.168.1.10", "old.example.com", egressAlertWindow+time.Minute),
		failedConn("192.168.1.10", "old.example.com", egressAlertWindow+2*time.Minute),
		failedConn("192.168.1.10", "old.example.com", egressAlertWindow+3*time.Minute),
	}
	out := buildEgressHealthJSON(relay.EgressSnapshot{}, recent, nil)
	if len(out.DirectFailures) != 0 {
		t.Fatalf("failures outside alert window should not alert, got %+v", out.DirectFailures)
	}
	if len(out.FailureStats) != 1 || out.FailureStats[0].Count != 3 {
		t.Fatalf("stats window should still count them, got %+v", out.FailureStats)
	}
}

func TestEgressHealthStatsWindowExpiry(t *testing.T) {
	recent := []relay.ConnInfo{
		failedConn("192.168.1.10", "gone.example.com", egressStatsWindow+time.Minute),
	}
	out := buildEgressHealthJSON(relay.EgressSnapshot{}, recent, nil)
	if len(out.FailureStats) != 0 {
		t.Fatalf("failures older than stats window should vanish, got %+v", out.FailureStats)
	}
}

func TestEgressHealthProxyFailuresIgnored(t *testing.T) {
	at := time.Now().Add(-time.Minute)
	recent := []relay.ConnInfo{
		{SrcIP: "192.168.1.10", DstHost: "x.example.com", ViaProxy: true,
			Status: "dial_failed", Failure: "代理拨号失败", StartedAt: at, EndedAt: &at},
	}
	out := buildEgressHealthJSON(relay.EgressSnapshot{}, recent, nil)
	if len(out.FailureStats) != 0 {
		t.Fatalf("proxy failures are not direct failures, got %+v", out.FailureStats)
	}
}
