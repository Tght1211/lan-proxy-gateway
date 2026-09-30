package relay

import (
	"net"
	"net/netip"
	"strings"
	"sync"
)

// Explicit rules and fail-closed settings always override automatic discovery.
func (s *Server) preferDirect(viaProxy, matched bool, p *routingPolicy) bool {
	action, _ := s.proxyFailAction.Load().(string)
	return viaProxy && !matched && p != nil && p.direct != nil && p.proxy != nil && (action == "" || action == "direct")
}

// Learn only after receiving upstream bytes, never merely after a successful dial.
func (s *Server) learnProxyOnResponse(conn net.Conn, host string) net.Conn {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if _, err := netip.ParseAddr(host); err == nil || host == "" || s.onProxyFallbackSuccess == nil {
		return conn
	}
	return &proxyLearningConn{Conn: conn, notify: func() { go s.onProxyFallbackSuccess(host) }}
}

type proxyLearningConn struct {
	net.Conn
	once   sync.Once
	notify func()
}

func (c *proxyLearningConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		c.once.Do(c.notify)
	}
	return n, err
}
func (c *proxyLearningConn) CloseWrite() error {
	if conn, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return conn.CloseWrite()
	}
	return nil
}
