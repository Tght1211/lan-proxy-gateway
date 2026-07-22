package dns

import (
	"net/netip"
	"strings"
	"time"

	"github.com/miekg/dns"
)

const maxRealIPEntries = 8192

type realIPEntry struct {
	name      string
	expiresAt time.Time
}

// LookupRealIP resolves a recently forwarded real DNS answer. The relay uses
// this only to label telemetry; it never changes the actual dial target.
func (s *Server) LookupRealIP(ip netip.Addr) (string, bool) {
	now := time.Now()
	s.realMu.RLock()
	entry, ok := s.realNames[ip]
	s.realMu.RUnlock()
	if !ok || now.After(entry.expiresAt) {
		if ok {
			s.realMu.Lock()
			delete(s.realNames, ip)
			s.realMu.Unlock()
		}
		return "", false
	}
	return entry.name, true
}

func (s *Server) rememberRealAnswers(msg *dns.Msg, now time.Time) {
	entries := make(map[netip.Addr]realIPEntry)
	for _, answer := range msg.Answer {
		var ip netip.Addr
		switch rr := answer.(type) {
		case *dns.A:
			parsed, ok := netip.AddrFromSlice(rr.A)
			if !ok {
				continue
			}
			ip = parsed.Unmap()
		case *dns.AAAA:
			parsed, ok := netip.AddrFromSlice(rr.AAAA)
			if !ok {
				continue
			}
			ip = parsed
		default:
			continue
		}
		ttl := time.Duration(answer.Header().Ttl) * time.Second
		if ttl < 30*time.Second {
			ttl = 30 * time.Second
		}
		if ttl > time.Hour {
			ttl = time.Hour
		}
		entries[ip] = realIPEntry{
			name:      strings.ToLower(strings.TrimSpace(answer.Header().Name)),
			expiresAt: now.Add(ttl),
		}
	}
	if len(entries) == 0 {
		return
	}
	s.realMu.Lock()
	if len(s.realNames)+len(entries) > maxRealIPEntries {
		for ip, entry := range s.realNames {
			if now.After(entry.expiresAt) {
				delete(s.realNames, ip)
			}
		}
		if len(s.realNames)+len(entries) > maxRealIPEntries {
			clear(s.realNames)
		}
	}
	for ip, entry := range entries {
		s.realNames[ip] = entry
	}
	s.realMu.Unlock()
}
