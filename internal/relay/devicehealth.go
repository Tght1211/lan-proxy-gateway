package relay

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// Device-level circuit breaker. It complements per-host learning: a burst of
// failures across different domains is treated as a device-wide compatibility
// problem and temporarily moves that device to direct egress.
const (
	deviceFailThreshold = 5
	deviceFailWindow    = 2 * time.Minute
	deviceDirectFor     = 15 * time.Minute
	deviceHealthMax     = 1024
)

type deviceHealthEntry struct {
	failures    map[string]time.Time
	directSince time.Time
	directUntil time.Time
	lastSeen    time.Time
}

type deviceHealth struct {
	mu      sync.Mutex
	devices map[string]*deviceHealthEntry
}

// DeviceAdaptiveState is exposed by the status API for UI explanation.
type DeviceAdaptiveState struct {
	Device       string     `json:"device"`
	Mode         string     `json:"mode"` // observing | direct
	FailureCount int        `json:"failure_count"`
	Hosts        []string   `json:"hosts,omitempty"`
	Since        *time.Time `json:"since,omitempty"`
	Until        *time.Time `json:"until,omitempty"`
}

type DeviceAdaptiveSnapshot struct {
	Threshold     int                   `json:"threshold"`
	WindowSeconds int                   `json:"window_seconds"`
	DirectSeconds int                   `json:"direct_seconds"`
	Devices       []DeviceAdaptiveState `json:"devices,omitempty"`
}

func newDeviceHealth() *deviceHealth {
	return &deviceHealth{devices: map[string]*deviceHealthEntry{}}
}

// recordProxyFailure returns true only when this failure trips the breaker.
// Distinct hosts are counted so one noisy endpoint cannot reroute a device.
func (d *deviceHealth) recordProxyFailure(device, host string, now time.Time) bool {
	device = strings.TrimSpace(device)
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if device == "" || host == "" {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.pruneLocked(now)
	e := d.devices[device]
	if e == nil {
		if len(d.devices) >= deviceHealthMax {
			d.evictOldestLocked()
		}
		e = &deviceHealthEntry{failures: map[string]time.Time{}}
		d.devices[device] = e
	}
	e.lastSeen = now
	if now.Before(e.directUntil) {
		return false
	}
	e.failures[host] = now
	if len(e.failures) < deviceFailThreshold {
		return false
	}
	e.failures = map[string]time.Time{}
	e.directSince = now
	e.directUntil = now.Add(deviceDirectFor)
	return true
}

func (d *deviceHealth) recordProxyOK(device, host string, now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if e := d.devices[device]; e != nil {
		delete(e.failures, strings.ToLower(strings.TrimSuffix(host, ".")))
		e.lastSeen = now
	}
}

func (d *deviceHealth) directDecision(device string, now time.Time) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	e := d.devices[device]
	if e == nil {
		return false
	}
	if now.Before(e.directUntil) {
		return true
	}
	if !e.directUntil.IsZero() {
		e.directSince = time.Time{}
		e.directUntil = time.Time{}
		e.failures = map[string]time.Time{}
	}
	return false
}

func (d *deviceHealth) snapshot(now time.Time) DeviceAdaptiveSnapshot {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.pruneLocked(now)
	out := DeviceAdaptiveSnapshot{
		Threshold:     deviceFailThreshold,
		WindowSeconds: int(deviceFailWindow / time.Second),
		DirectSeconds: int(deviceDirectFor / time.Second),
	}
	for device, e := range d.devices {
		active := now.Before(e.directUntil)
		if !active && len(e.failures) == 0 {
			continue
		}
		state := DeviceAdaptiveState{Device: device, Mode: "observing", FailureCount: len(e.failures)}
		for host := range e.failures {
			state.Hosts = append(state.Hosts, host)
		}
		sort.Strings(state.Hosts)
		if active {
			state.Mode = "direct"
			since, until := e.directSince, e.directUntil
			state.Since, state.Until = &since, &until
		}
		out.Devices = append(out.Devices, state)
	}
	sort.Slice(out.Devices, func(i, j int) bool {
		if out.Devices[i].Mode != out.Devices[j].Mode {
			return out.Devices[i].Mode == "direct"
		}
		return out.Devices[i].Device < out.Devices[j].Device
	})
	return out
}

func (d *deviceHealth) pruneLocked(now time.Time) {
	cutoff := now.Add(-deviceFailWindow)
	for device, e := range d.devices {
		for host, at := range e.failures {
			if at.Before(cutoff) {
				delete(e.failures, host)
			}
		}
		active := now.Before(e.directUntil)
		if !active && len(e.failures) == 0 && e.lastSeen.Before(cutoff) {
			delete(d.devices, device)
		}
	}
}

func (d *deviceHealth) evictOldestLocked() {
	var oldest string
	var oldestAt time.Time
	for device, e := range d.devices {
		if oldest == "" || e.lastSeen.Before(oldestAt) {
			oldest, oldestAt = device, e.lastSeen
		}
	}
	delete(d.devices, oldest)
}
