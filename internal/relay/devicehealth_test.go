package relay

import (
	"testing"
	"time"
)

func TestDeviceHealthTripsOnDistinctHostBurst(t *testing.T) {
	d := newDeviceHealth()
	now := time.Now()
	for i, host := range []string{"a.test", "b.test", "c.test", "d.test"} {
		if d.recordProxyFailure("192.168.1.50", host, now.Add(time.Duration(i)*time.Second)) {
			t.Fatal("breaker tripped before threshold")
		}
	}
	if !d.recordProxyFailure("192.168.1.50", "e.test", now.Add(4*time.Second)) {
		t.Fatal("fifth distinct failure must trip device breaker")
	}
	if !d.directDecision("192.168.1.50", now.Add(time.Minute)) {
		t.Fatal("tripped device must use direct egress")
	}
	if d.directDecision("192.168.1.50", now.Add(deviceDirectFor+time.Minute)) {
		t.Fatal("device must leave temporary direct mode after cooldown")
	}
}

func TestDeviceHealthDeduplicatesAndExpiresHosts(t *testing.T) {
	d := newDeviceHealth()
	now := time.Now()
	for i := 0; i < 10; i++ {
		d.recordProxyFailure("192.168.1.50", "same.test", now.Add(time.Duration(i)*time.Second))
	}
	if d.directDecision("192.168.1.50", now.Add(time.Minute)) {
		t.Fatal("one noisy host must not trip a device-wide breaker")
	}
	later := now.Add(deviceFailWindow + 20*time.Second)
	for i, host := range []string{"b.test", "c.test", "d.test", "e.test"} {
		d.recordProxyFailure("192.168.1.50", host, later.Add(time.Duration(i)*time.Second))
	}
	if d.directDecision("192.168.1.50", later.Add(time.Minute)) {
		t.Fatal("expired host must not count toward a later burst")
	}
}

func TestDeviceHealthSuccessRemovesCandidate(t *testing.T) {
	d := newDeviceHealth()
	now := time.Now()
	d.recordProxyFailure("192.168.1.50", "a.test", now)
	d.recordProxyOK("192.168.1.50", "a.test", now.Add(time.Second))
	snap := d.snapshot(now.Add(time.Second))
	if len(snap.Devices) != 0 {
		t.Fatalf("successful host must leave no pending candidate: %+v", snap.Devices)
	}
}
