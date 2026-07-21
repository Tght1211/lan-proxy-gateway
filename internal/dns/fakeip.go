package dns

import (
	"net/netip"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// fakeIPEntry maps one allocated fake address to its domain.
type fakeIPEntry struct {
	ip            netip.Addr
	name          string
	lastSeen      time.Time
	persistedSeen time.Time
}

// fakeIPPool hands out stable fake addresses from a prefix (198.18.0.0/16 by
// default) and remembers the fakeIP → domain mapping so the relay can recover
// the original domain when a device connects to the fake address.
//
// Answers carry TTL=1 (devices re-query constantly, keeping mappings warm);
// the pool retains mappings for idleTTL of no use and evicts LRU past maxSize.
type fakeIPPool struct {
	mu      sync.Mutex
	prefix  netip.Prefix
	base    uint32 // prefix address as u32
	bits    int    // host bits in the prefix
	offset  uint32 // next allocation offset (never 0 or all-ones)
	idleTTL time.Duration
	maxSize int
	byName  map[string]*fakeIPEntry
	byIP    map[netip.Addr]*fakeIPEntry
	dirty   atomic.Bool
}

func newFakeIPPool(prefix netip.Prefix, idleTTL time.Duration, maxSize int) *fakeIPPool {
	a4 := prefix.Addr().As4()
	base := uint32(a4[0])<<24 | uint32(a4[1])<<16 | uint32(a4[2])<<8 | uint32(a4[3])
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &fakeIPPool{
		prefix:  prefix,
		base:    base,
		bits:    32 - prefix.Bits(),
		offset:  1,
		idleTTL: idleTTL,
		maxSize: maxSize,
		byName:  map[string]*fakeIPEntry{},
		byIP:    map[netip.Addr]*fakeIPEntry{},
	}
}

func (p *fakeIPPool) addrAt(off uint32) netip.Addr {
	v := p.base + off
	return netip.AddrFrom4([4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)})
}

// maxOffset excludes the all-ones host part (broadcast-ish) for tidiness.
func (p *fakeIPPool) maxOffset() uint32 {
	if p.bits >= 32 {
		return 1
	}
	return (uint32(1) << p.bits) - 1
}

// Get returns the stable fake address for name, allocating if needed.
func (p *fakeIPPool) Get(name string, now time.Time) netip.Addr {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e, ok := p.byName[name]; ok {
		e.lastSeen = now
		if now.Sub(e.persistedSeen) >= cacheTouchInterval {
			p.dirty.Store(true)
		}
		return e.ip
	}
	p.evictLocked(now)
	e := &fakeIPEntry{ip: p.allocLocked(), name: name, lastSeen: now}
	p.byName[name] = e
	p.byIP[e.ip] = e
	p.dirty.Store(true)
	return e.ip
}

// Lookup resolves a fake address back to its domain.
func (p *fakeIPPool) Lookup(ip netip.Addr, now time.Time) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.byIP[ip]
	if !ok {
		return "", false
	}
	if p.idleTTL > 0 && now.Sub(e.lastSeen) > p.idleTTL {
		p.removeLocked(e)
		return "", false
	}
	e.lastSeen = now
	if now.Sub(e.persistedSeen) >= cacheTouchInterval {
		p.dirty.Store(true)
	}
	return e.name, true
}

// Len reports the number of live mappings (for stats/tests).
func (p *fakeIPPool) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.byIP)
}

func (p *fakeIPPool) allocLocked() netip.Addr {
	max := p.maxOffset()
	start := p.offset
	for {
		off := p.offset
		p.offset++
		if p.offset >= max {
			p.offset = 1
		}
		ip := p.addrAt(off)
		if _, taken := p.byIP[ip]; !taken {
			return ip
		}
		if p.offset == start {
			// full circle: force-evict the LRU entry
			p.evictLRULocked()
		}
	}
}

// evictLocked drops idle-expired entries, then enforces the size cap by LRU.
func (p *fakeIPPool) evictLocked(now time.Time) {
	if p.idleTTL > 0 {
		for _, e := range p.byIP {
			if now.Sub(e.lastSeen) > p.idleTTL {
				p.removeLocked(e)
			}
		}
	}
	for len(p.byIP) >= p.maxSize {
		p.evictLRULocked()
	}
}

func (p *fakeIPPool) evictLRULocked() {
	var oldest *fakeIPEntry
	for _, e := range p.byIP {
		if oldest == nil || e.lastSeen.Before(oldest.lastSeen) {
			oldest = e
		}
	}
	if oldest != nil {
		p.removeLocked(oldest)
	}
}

func (p *fakeIPPool) removeLocked(e *fakeIPEntry) {
	delete(p.byIP, e.ip)
	delete(p.byName, e.name)
	p.dirty.Store(true)
}

type fakeIPPoolSnapshot struct {
	Offset  uint32
	Entries []fakeIPSnapshotEntry
}

type fakeIPSnapshotEntry struct {
	IP       netip.Addr
	Name     string
	LastSeen time.Time
}

func (p *fakeIPPool) snapshot(now time.Time) fakeIPPoolSnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.evictLocked(now)
	entries := make([]fakeIPSnapshotEntry, 0, len(p.byIP))
	for _, e := range p.byIP {
		entries = append(entries, fakeIPSnapshotEntry{IP: e.ip, Name: e.name, LastSeen: e.lastSeen})
		e.persistedSeen = e.lastSeen
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].IP.Less(entries[j].IP) })
	p.dirty.Store(false)
	return fakeIPPoolSnapshot{Offset: p.offset, Entries: entries}
}

func (p *fakeIPPool) restore(snapshot fakeIPPoolSnapshot, now time.Time) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, item := range snapshot.Entries {
		if item.Name == "" || !item.IP.Is4() || !p.prefix.Contains(item.IP) {
			continue
		}
		if item.LastSeen.IsZero() || item.LastSeen.After(now) {
			item.LastSeen = now
		}
		if p.idleTTL > 0 && now.Sub(item.LastSeen) > p.idleTTL {
			continue
		}
		if _, exists := p.byIP[item.IP]; exists {
			continue
		}
		if _, exists := p.byName[item.Name]; exists {
			continue
		}
		e := &fakeIPEntry{
			ip: item.IP, name: item.Name, lastSeen: item.LastSeen, persistedSeen: item.LastSeen,
		}
		p.byIP[e.ip] = e
		p.byName[e.name] = e
		if len(p.byIP) >= p.maxSize {
			break
		}
	}
	if snapshot.Offset > 0 && snapshot.Offset < p.maxOffset() {
		p.offset = snapshot.Offset
	}
	p.dirty.Store(false)
	return len(p.byIP)
}
