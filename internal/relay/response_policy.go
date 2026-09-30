package relay

import (
	"context"
	"net"
	"net/netip"
	"strings"
	"time"
)

type responseState struct {
	timeout           time.Duration
	cooldown, expires time.Time
}

// ResponsePolicy is read for each new attempt, so saved settings apply without restarting.
type ResponsePolicy struct {
	DirectWait, ProxyWait, MaxDirectWait, Cooldown, MemoryTTL time.Duration
}

func (s *Server) currentResponsePolicy() ResponsePolicy {
	p := ResponsePolicy{5 * time.Second, 5 * time.Second, 30 * time.Second, 30 * time.Second, 10 * time.Minute}
	if s.responsePolicy != nil {
		p = s.responsePolicy()
	}
	if p.DirectWait <= 0 {
		p.DirectWait = 5 * time.Second
	}
	if p.ProxyWait <= 0 {
		p.ProxyWait = 5 * time.Second
	}
	if p.MaxDirectWait < p.DirectWait {
		p.MaxDirectWait = p.DirectWait
	}
	if p.Cooldown <= 0 {
		p.Cooldown = 30 * time.Second
	}
	if p.MemoryTTL <= 0 {
		p.MemoryTTL = 10 * time.Minute
	}
	return p
}

func (s *Server) responsePlan(host string, now time.Time) (time.Duration, bool) {
	p := s.currentResponsePolicy()
	s.responseMu.Lock()
	defer s.responseMu.Unlock()
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	state, ok := s.responses[host]
	if !ok || !now.Before(state.expires) {
		delete(s.responses, host)
		return p.DirectWait, true
	}
	return min(state.timeout, p.MaxDirectWait), !now.Before(state.cooldown)
}
func (s *Server) responseResult(host string, proxy, success bool, now time.Time) {
	p := s.currentResponsePolicy()
	s.responseMu.Lock()
	defer s.responseMu.Unlock()
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if proxy && success {
		delete(s.responses, host)
		return
	}
	if !proxy {
		return
	} // Preserve a working slow-direct budget; never learn failure.
	if s.responses == nil {
		s.responses = map[string]responseState{}
	}
	if len(s.responses) >= 4096 {
		for h, v := range s.responses {
			if !now.Before(v.expires) {
				delete(s.responses, h)
			}
		}
		if len(s.responses) >= 4096 {
			return
		}
	}
	state := s.responses[host]
	if state.timeout == 0 {
		state.timeout = p.DirectWait
	}
	state.timeout *= 2
	if state.timeout > p.MaxDirectWait {
		state.timeout = p.MaxDirectWait
	}
	state.cooldown = now.Add(p.Cooldown)
	state.expires = now.Add(p.MemoryTTL)
	s.responses[host] = state
}
func (s *Server) watchDirectResponse(conn net.Conn, host, target string, proxy Dialer, tc *TrackedConn, budget time.Duration, allow bool) net.Conn {
	var retry func(context.Context) (net.Conn, error)
	if allow {
		retry = func(ctx context.Context) (net.Conn, error) {
			next, err := proxy.DialContext(ctx, "tcp", target)
			if err == nil {
				tc.setProxyEgress(next.RemoteAddr().String())
				tc.MarkFallback()
			}
			return next, err
		}
	}
	wrapped := newResponseRetryConn(conn, retry, budget, func(via, success bool) {
		s.responseResult(host, via, success, time.Now())
		if via && success {
			s.notifyProxyResponse(host)
		}
	})
	wrapped.onReplay = tc.AddUp
	wrapped.retryTimeout = s.currentResponsePolicy().ProxyWait
	return wrapped
}

func (s *Server) watchProxyResponse(conn net.Conn, host string) net.Conn {
	return newResponseRetryConn(conn, nil, s.currentResponsePolicy().ProxyWait, func(_ bool, success bool) {
		s.responseResult(host, true, success, time.Now())
		if success {
			s.notifyProxyResponse(host)
		}
	})
}

func (s *Server) notifyProxyResponse(host string) {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if _, err := netip.ParseAddr(host); err == nil || host == "" || s.onProxyFallbackSuccess == nil {
		return
	}
	go s.onProxyFallbackSuccess(host)
}
