package dns

import (
	"net/netip"
	"sync"
	"time"
)

// fakeIPEntry maps one allocated fake address to its domain.
type fakeIPEntry struct {
	ip       netip.Addr
	name     string
	lastSeen time.Time
}

// fakeIPPool hands out stable fake addresses from a prefix (198.18.0.0/16 by
// default) and remembers the fakeIP → domain mapping so the relay can recover
// the original domain when a device connects to the fake address.
//
// Answers carry TTL=1 (devices re-query constantly, keeping mappings warm);
// the pool retains mappings for idleTTL of no use and evicts LRU past maxSize.
type fakeIPPool struct {
	mu       sync.Mutex
	prefix   netip.Prefix
	base     uint32 // prefix address as u32
	bits     int    // host bits in the prefix
	offset   uint32 // next allocation offset (never 0 or all-ones)
	idleTTL  time.Duration
	maxSize  int
	byName   map[string]*fakeIPEntry
	byIP     map[netip.Addr]*fakeIPEntry
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
		return e.ip
	}
	p.evictLocked(now)
	e := &fakeIPEntry{ip: p.allocLocked(), name: name, lastSeen: now}
	p.byName[name] = e
	p.byIP[e.ip] = e
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
}
