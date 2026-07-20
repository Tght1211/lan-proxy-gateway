package relay

import (
	"errors"
	"net"
	"net/netip"
)

// OrigDSTResolver recovers the original destination of a transparently
// redirected TCP connection.
type OrigDSTResolver interface {
	Resolve(c *net.TCPConn) (netip.AddrPort, error)
}

// ErrOrigDSTNotSupported is returned on platforms without a transparent
// original-destination recovery implementation (e.g. Windows).
var ErrOrigDSTNotSupported = errors.New("当前平台不支持透明转发目标恢复")

// OrigDSTFunc adapts a function (mainly for tests) to OrigDSTResolver.
type OrigDSTFunc func(c *net.TCPConn) (netip.AddrPort, error)

func (f OrigDSTFunc) Resolve(c *net.TCPConn) (netip.AddrPort, error) { return f(c) }
