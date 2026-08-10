package dns

import (
	"sync"
	"time"

	"github.com/miekg/dns"
)

// dnsCache is a simple in-memory DNS response cache with TTL expiration.
// It caches full dns.Msg responses keyed by (qname, qtype, qclass).
type dnsCache struct {
	mu       sync.RWMutex
	entries  map[cacheKey]*cacheEntry
	maxSize  int
	minTTL   time.Duration
	disabled bool
}

type cacheKey struct {
	name  string
	qtype uint16
	class uint16
}

type cacheEntry struct {
	msg       *dns.Msg
	storedAt  time.Time
	expiresAt time.Time
	origTTL   uint32
}

func newDNSCache(maxSize int) *dnsCache {
	if maxSize <= 0 {
		maxSize = 4096
	}
	return &dnsCache{
		entries: make(map[cacheKey]*cacheEntry, maxSize),
		maxSize: maxSize,
		minTTL:  5 * time.Second,
	}
}

// Get returns a cached response if present and not expired.
// The TTLs in the returned message are adjusted for elapsed time.
func (c *dnsCache) Get(q dns.Question, now time.Time) (*dns.Msg, bool) {
	if c.disabled {
		return nil, false
	}
	key := cacheKey{name: q.Name, qtype: q.Qtype, class: q.Qclass}
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || now.After(entry.expiresAt) {
		if ok {
			c.mu.Lock()
			delete(c.entries, key)
			c.mu.Unlock()
		}
		return nil, false
	}
	// Adjust TTLs for time elapsed since caching.
	msg := entry.msg.Copy()
	elapsed := uint32(now.Sub(entry.storedAt).Seconds())
	adjustTTL(msg.Answer, elapsed)
	adjustTTL(msg.Ns, elapsed)
	adjustTTL(msg.Extra, elapsed)
	return msg, true
}

// Put stores a response in the cache. Only caches successful responses
// with at least one answer and a positive TTL.
func (c *dnsCache) Put(q dns.Question, msg *dns.Msg, now time.Time) {
	if c.disabled || msg == nil || msg.Rcode != dns.RcodeSuccess || len(msg.Answer) == 0 {
		return
	}
	ttl := minAnswerTTL(msg.Answer)
	if ttl == 0 {
		return
	}
	dur := time.Duration(ttl) * time.Second
	if dur < c.minTTL {
		dur = c.minTTL
	}

	key := cacheKey{name: q.Name, qtype: q.Qtype, class: q.Qclass}
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest if at capacity.
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}
	c.entries[key] = &cacheEntry{
		msg:       msg.Copy(),
		storedAt:  now,
		expiresAt: now.Add(dur),
		origTTL:   ttl,
	}
}

// Len returns the current cache size.
func (c *dnsCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func (c *dnsCache) evictOldest() {
	var oldest cacheKey
	var oldestTime time.Time
	first := true
	for k, v := range c.entries {
		if first || v.storedAt.Before(oldestTime) {
			oldest = k
			oldestTime = v.storedAt
			first = false
		}
	}
	if !first {
		delete(c.entries, oldest)
	}
}

func minAnswerTTL(rrs []dns.RR) uint32 {
	if len(rrs) == 0 {
		return 0
	}
	min := rrs[0].Header().Ttl
	for _, rr := range rrs[1:] {
		if rr.Header().Ttl < min {
			min = rr.Header().Ttl
		}
	}
	return min
}

func adjustTTL(rrs []dns.RR, elapsed uint32) {
	for _, rr := range rrs {
		h := rr.Header()
		if h.Ttl > elapsed {
			h.Ttl -= elapsed
		} else {
			h.Ttl = 1
		}
	}
}
