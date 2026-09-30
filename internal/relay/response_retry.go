package relay

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"time"
)

const initialResponseTimeout = 5 * time.Second

// Retries only a complete TLS ClientHello flight before ANY upstream bytes
// have reached the client. Never replays HTTP requests or TLS application data.
type responseRetryConn struct {
	writeMu                                      sync.Mutex
	mu                                           sync.Mutex
	conn                                         net.Conn
	fallback                                     func(context.Context) (net.Conn, error)
	timeout                                      time.Duration
	retryTimeout                                 time.Duration
	flight                                       []byte
	started, received, retried, closed, overflow bool
	cancel                                       context.CancelFunc
	onReplay                                     func(int64)
	onResult                                     func(proxy, success bool)
}

func newResponseRetryConn(c net.Conn, fallback func(context.Context) (net.Conn, error), timeout time.Duration, result func(bool, bool)) *responseRetryConn {
	return &responseRetryConn{conn: c, fallback: fallback, timeout: timeout, retryTimeout: initialResponseTimeout, onResult: result}
}
func (c *responseRetryConn) Write(p []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return 0, net.ErrClosed
	}
	conn := c.conn
	if !c.started && !c.received {
		c.started = true
		conn.SetDeadline(time.Now().Add(c.timeout))
	}
	c.mu.Unlock()
	n, err := conn.Write(p)
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.received && !c.retried && !c.overflow {
		if len(c.flight)+n <= 65536 {
			c.flight = append(c.flight, p[:n]...)
		} else {
			c.flight = nil
			c.overflow = true
		}
	}
	return n, err
}
func (c *responseRetryConn) Read(p []byte) (int, error) {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	n, err := conn.Read(p)
	if n == 0 && err != nil {
		c.writeMu.Lock()
		defer c.writeMu.Unlock()
	}
	c.mu.Lock()
	if n > 0 {
		if !c.received {
			c.received = true
			c.flight = nil
			conn.SetDeadline(time.Time{})
			if c.onResult != nil {
				c.onResult(c.retried, true)
			}
		}
		c.mu.Unlock()
		return n, err
	}
	if err == nil || c.closed || c.received {
		c.mu.Unlock()
		return n, err
	}
	canRetry := !c.retried && c.fallback != nil && !c.overflow && replayableClientHello(c.flight)
	if !canRetry {
		if c.onResult != nil {
			c.onResult(c.retried, false)
			c.onResult = nil
		}
		c.mu.Unlock()
		return n, err
	}
	c.retried = true
	flight := append([]byte(nil), c.flight...)
	c.flight = nil
	ctx, cancel := context.WithTimeout(context.Background(), c.retryTimeout)
	c.cancel = cancel
	fallback := c.fallback
	c.mu.Unlock()
	conn.Close()
	next, dialErr := fallback(ctx)
	if dialErr == nil {
		deadline, _ := ctx.Deadline()
		next.SetDeadline(deadline)
		var replayed int64
		replayed, dialErr = io.Copy(next, bytes.NewReader(flight))
		if c.onReplay != nil {
			c.onReplay(replayed)
		}
	}
	c.mu.Lock()
	c.cancel = nil
	cancel()
	if c.closed {
		if next != nil {
			next.Close()
		}
		c.mu.Unlock()
		return 0, net.ErrClosed
	}
	if dialErr != nil {
		if next != nil {
			next.Close()
		}
		if c.onResult != nil {
			c.onResult(true, false)
			c.onResult = nil
		}
		c.mu.Unlock()
		return 0, dialErr
	}
	c.conn = next
	c.mu.Unlock()
	n, err = next.Read(p)
	c.mu.Lock()
	if n > 0 {
		c.received = true
		next.SetDeadline(time.Time{})
	}
	if c.onResult != nil {
		c.onResult(true, n > 0)
		c.onResult = nil
	}
	c.mu.Unlock()
	return n, err
}

func replayableClientHello(p []byte) bool {
	var handshake []byte
	for len(p) > 0 {
		if len(p) < 5 || p[0] != 22 || p[1] != 3 {
			return false
		}
		n := int(binary.BigEndian.Uint16(p[3:5]))
		if n == 0 || len(p) < 5+n {
			return false
		}
		handshake = append(handshake, p[5:5+n]...)
		p = p[5+n:]
	}
	if len(handshake) < 4 || handshake[0] != 1 {
		return false
	}
	n := int(handshake[1])<<16 | int(handshake[2])<<8 | int(handshake[3])
	return n == len(handshake)-4
}
func (c *responseRetryConn) Close() error {
	c.mu.Lock()
	c.closed = true
	if c.cancel != nil {
		c.cancel()
	}
	conn := c.conn
	c.mu.Unlock()
	return conn.Close()
}
func (c *responseRetryConn) LocalAddr() net.Addr {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.LocalAddr()
}
func (c *responseRetryConn) RemoteAddr() net.Addr {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.RemoteAddr()
}
func (c *responseRetryConn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.SetDeadline(t)
}
func (c *responseRetryConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.SetReadDeadline(t)
}
func (c *responseRetryConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.SetWriteDeadline(t)
}
func (c *responseRetryConn) CloseWrite() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if x, ok := c.conn.(interface{ CloseWrite() error }); ok {
		return x.CloseWrite()
	}
	return nil
}
