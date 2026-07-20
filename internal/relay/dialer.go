// Package relay is the transparent TCP relay at the heart of the gateway.
// It accepts connections redirected by the OS firewall, recovers the original
// destination, and dials out through the configured egress dialer (direct,
// SOCKS5, or HTTP CONNECT).
package relay

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"time"
)

// Dialer connects to a target address (host may be a domain or IP).
type Dialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// NewDirectDialer dials targets directly via the host network.
func NewDirectDialer(timeout time.Duration) Dialer {
	return directDialer{d: &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}}
}

type directDialer struct{ d *net.Dialer }

func (d directDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return d.d.DialContext(ctx, network, addr)
}

// Probe checks end-to-end connectivity through d by issuing a minimal HTTP
// GET to the given host:port (e.g. a generate_204 endpoint) and requiring any
// HTTP response back.
func Probe(ctx context.Context, d Dialer, addr string) error {
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("拨号失败: %w", err)
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	host, _, _ := net.SplitHostPort(addr)
	if _, err := fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", host); err != nil {
		return fmt.Errorf("发送探测请求失败: %w", err)
	}
	br := bufio.NewReader(io.LimitReader(conn, 4096))
	line, err := br.ReadString('\n')
	if err != nil {
		return fmt.Errorf("读取探测响应失败: %w", err)
	}
	if len(line) < 12 || line[:5] != "HTTP/" {
		return fmt.Errorf("探测响应异常: %q", line)
	}
	return nil
}
