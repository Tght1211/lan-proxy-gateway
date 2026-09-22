package relay

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"time"
)

// DialExplicit routes an explicit HTTP proxy connection using the current
// device/domain rules. Domains remain unresolved when sent to an upstream proxy.
func (s *Server) DialExplicit(ctx context.Context, srcIP, target string) (net.Conn, error) {
	return s.dialExplicit(ctx, srcIP, target, 15*time.Second)
}

func (s *Server) dialExplicit(ctx context.Context, srcIP, target string, timeout time.Duration) (net.Conn, error) {
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		return nil, err
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return nil, fmt.Errorf("invalid destination port")
	}
	ip, _ := netip.ParseAddr(host)
	var d, fallback Dialer
	via := s.viaProxy.Load()
	if policy := s.routing.Load(); policy != nil {
		var rejected, matched bool
		d, via, rejected, matched = policy.selectDialer(srcIP, host, ip)
		if rejected {
			s.tracker.RecordRejected(srcIP, host, n, "http-proxy")
			return nil, fmt.Errorf("connection rejected by routing rule")
		}
		action, _ := s.proxyFailAction.Load().(string)
		if via && !matched && (action == "" || action == "direct") {
			fallback = policy.direct
		}
	} else if holder := s.dialer.Load(); holder != nil {
		d = holder.dialer
	}
	if d == nil {
		return nil, fmt.Errorf("no egress configured")
	}
	dial := func(d Dialer) (net.Conn, error) {
		dialCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return d.DialContext(dialCtx, "tcp", target)
	}
	conn, err := dial(d)
	fellBack := false
	if err != nil && fallback != nil && ctx.Err() == nil {
		conn, err = dial(fallback)
		if err == nil {
			via = false
			fellBack = true
		}
	}
	if err != nil {
		s.tracker.RecordDialFailure(srcIP, host, n, via, classifyDialError(err, via), "http-proxy")
		return nil, err
	}
	tracked := s.tracker.Open(srcIP, host, n, via, "tcp", "http-proxy")
	if fellBack {
		tracked.MarkFallback()
	}
	return &explicitConn{Conn: conn, tracked: tracked, onClose: func() {
		if fellBack && tracked.Down() > 0 {
			s.notifyFallbackSuccess(host)
		}
	}}, nil
}

type explicitConn struct {
	net.Conn
	tracked *TrackedConn
	once    sync.Once
	onClose func()
}

func (c *explicitConn) Read(p []byte) (int, error) {
	n, e := c.Conn.Read(p)
	c.tracked.AddDown(int64(n))
	return n, e
}
func (c *explicitConn) Write(p []byte) (int, error) {
	n, e := c.Conn.Write(p)
	c.tracked.AddUp(int64(n))
	return n, e
}
func (c *explicitConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() {
		c.tracked.Close()
		if c.onClose != nil {
			c.onClose()
		}
	})
	return err
}
func (c *explicitConn) CloseWrite() error {
	if conn, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return conn.CloseWrite()
	}
	// No half-close support: let the response drain before the owner closes.
	return nil
}
