// UDP fake-IP relay: intercepts UDP packets destined for the fake-IP range,
// resolves the real destination via domain lookup, and forwards directly.
// Game voice, video calls and similar UDP traffic regain connectivity without
// needing a full UDP proxy (which HTTP/SOCKS5 cannot provide anyway).
//
// @author buchi
// @since 2026-08-08
package relay

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"time"
)

const (
	udpSessionTTL     = 2 * time.Minute
	udpCleanInterval  = 30 * time.Second
	udpReadBuf        = 64 * 1024 // 64 KiB per read — covers jumbo datagrams
	udpMaxSessions    = 8192
	udpResolveTimeout = 3 * time.Second
)

// UDPRelayOptions configures the UDP fake-IP relay.
type UDPRelayOptions struct {
	ListenAddr   string // e.g. ":17893"
	FakeIPRange  *netip.Prefix
	LookupFakeIP func(netip.Addr) (string, bool)
	// Resolve translates a domain to a real IP. Typically wraps net.Resolver.
	Resolve func(ctx context.Context, domain string) (netip.Addr, error)
	Logger  *slog.Logger
}

// UDPRelay transparently forwards UDP packets from LAN devices whose
// destinations fall in the fake-IP range to the real server IPs.
type UDPRelay struct {
	listenAddr   string
	fakeRange    *netip.Prefix
	lookupFakeIP func(netip.Addr) (string, bool)
	resolve      func(ctx context.Context, domain string) (netip.Addr, error)
	logger       *slog.Logger

	mu             sync.Mutex
	sessions       map[sessionKey]*udpSession
	conn           *net.UDPConn
	done           chan struct{}
	origDstEnabled bool // Linux: IP_RECVORIGDSTADDR set
}

// sessionKey identifies a UDP "connection" from a LAN device.
type sessionKey struct {
	clientAddr netip.AddrPort // device IP:port
	fakeAddr   netip.AddrPort // original fake-IP destination:port
}

// udpSession tracks one NAT mapping between a LAN device and the real server.
type udpSession struct {
	upstream   *net.UDPConn   // local socket talking to the real server
	clientAddr netip.AddrPort // where to send responses back
	lastActive time.Time
}

func NewUDPRelay(opts UDPRelayOptions) *UDPRelay {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	addr := opts.ListenAddr
	if addr == "" {
		addr = ":17893"
	}
	return &UDPRelay{
		listenAddr:   addr,
		fakeRange:    opts.FakeIPRange,
		lookupFakeIP: opts.LookupFakeIP,
		resolve:      opts.Resolve,
		logger:       logger,
		sessions:     make(map[sessionKey]*udpSession),
		done:         make(chan struct{}),
	}
}

// SetFakeIP updates the lookup function live (mirrors relay.Server.SetFakeIP).
func (r *UDPRelay) SetFakeIP(prefix *netip.Prefix, lookup func(netip.Addr) (string, bool)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fakeRange = prefix
	r.lookupFakeIP = lookup
}

// ListenAndServe binds the UDP listener and processes packets until ctx is cancelled.
func (r *UDPRelay) ListenAndServe(ctx context.Context) error {
	laddr, err := net.ResolveUDPAddr("udp4", r.listenAddr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		return err
	}
	r.conn = conn
	r.logger.Info("UDP fake-IP 转发已启动", "listen", conn.LocalAddr().String())

	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-r.done:
		}
	}()
	go r.cleanupLoop(ctx)

	return r.readLoop()
}

// Close stops the relay.
func (r *UDPRelay) Close() error {
	select {
	case <-r.done:
	default:
		close(r.done)
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

// Addr returns the bound address (after ListenAndServe).
func (r *UDPRelay) Addr() string {
	if r.conn == nil {
		return ""
	}
	return r.conn.LocalAddr().String()
}

func (r *UDPRelay) readLoop() error {
	buf := make([]byte, udpReadBuf)
	for {
		n, clientAddr, origDst, err := r.readFromRedirected(buf)
		if err != nil {
			select {
			case <-r.done:
				return nil
			default:
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return err
		}
		if origDst == (netip.AddrPort{}) {
			continue
		}
		r.handlePacket(buf[:n], clientAddr, origDst)
	}
}

func (r *UDPRelay) handlePacket(data []byte, clientAddr, origDst netip.AddrPort) {
	r.mu.Lock()
	prefix := r.fakeRange
	lookup := r.lookupFakeIP
	r.mu.Unlock()

	if prefix == nil || !prefix.Contains(origDst.Addr()) {
		return
	}
	if lookup == nil {
		return
	}

	key := sessionKey{clientAddr: clientAddr, fakeAddr: origDst}

	r.mu.Lock()
	sess, exists := r.sessions[key]
	if exists {
		sess.lastActive = time.Now()
		r.mu.Unlock()
		_, _ = sess.upstream.Write(data)
		return
	}
	r.mu.Unlock()

	domain, ok := lookup(origDst.Addr())
	if !ok {
		r.logger.Debug("UDP fake-IP 映射缺失", "fake_ip", origDst.Addr(), "client", clientAddr)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), udpResolveTimeout)
	realIP, err := r.resolve(ctx, domain)
	cancel()
	if err != nil {
		r.logger.Warn("UDP 域名解析失败", "domain", domain, "err", err)
		return
	}

	realDst := netip.AddrPortFrom(realIP, origDst.Port())
	realUDPAddr := net.UDPAddrFromAddrPort(realDst)

	upConn, err := net.DialUDP("udp4", nil, realUDPAddr)
	if err != nil {
		r.logger.Warn("UDP 上游拨号失败", "domain", domain, "real", realDst, "err", err)
		return
	}

	sess = &udpSession{
		upstream:   upConn,
		clientAddr: clientAddr,
		lastActive: time.Now(),
	}

	r.mu.Lock()
	if len(r.sessions) >= udpMaxSessions {
		r.mu.Unlock()
		upConn.Close()
		r.logger.Warn("UDP 会话数已满", "max", udpMaxSessions)
		return
	}
	r.sessions[key] = sess
	r.mu.Unlock()

	r.logger.Debug("UDP 新会话", "client", clientAddr, "fake", origDst, "domain", domain, "real", realDst)

	_, _ = upConn.Write(data)

	go r.readUpstream(key, sess)
}

// readUpstream reads responses from the real server and sends them back to the LAN device.
func (r *UDPRelay) readUpstream(key sessionKey, sess *udpSession) {
	defer func() {
		sess.upstream.Close()
		r.mu.Lock()
		delete(r.sessions, key)
		r.mu.Unlock()
	}()

	buf := make([]byte, udpReadBuf)
	for {
		_ = sess.upstream.SetReadDeadline(time.Now().Add(udpSessionTTL))
		n, err := sess.upstream.Read(buf)
		if err != nil {
			return
		}
		sess.lastActive = time.Now()
		dst := net.UDPAddrFromAddrPort(sess.clientAddr)
		_, _ = r.conn.WriteToUDP(buf[:n], dst)
	}
}

// cleanupLoop periodically removes stale sessions.
func (r *UDPRelay) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(udpCleanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.done:
			return
		case <-ticker.C:
			r.cleanup()
		}
	}
}

func (r *UDPRelay) cleanup() {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, sess := range r.sessions {
		if now.Sub(sess.lastActive) > udpSessionTTL {
			sess.upstream.Close()
			delete(r.sessions, key)
		}
	}
}

// defaultResolve is the fallback domain→IP resolver using the system resolver.
func defaultResolve(ctx context.Context, domain string) (netip.Addr, error) {
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip4", domain)
	if err != nil {
		return netip.Addr{}, err
	}
	if len(addrs) == 0 {
		return netip.Addr{}, &net.DNSError{Err: "no A records", Name: domain}
	}
	return addrs[0], nil
}

// NewUpstreamResolver creates a resolver that queries the given upstream DNS
// servers directly (bypassing the local fake-IP DNS to avoid loops).
func NewUpstreamResolver(upstreams []string) func(context.Context, string) (netip.Addr, error) {
	if len(upstreams) == 0 {
		return defaultResolve
	}
	dialer := &net.Dialer{Timeout: udpResolveTimeout}
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			var lastErr error
			for _, up := range upstreams {
				if _, _, err := net.SplitHostPort(up); err != nil {
					up = net.JoinHostPort(up, "53")
				}
				conn, err := dialer.DialContext(ctx, "udp", up)
				if err != nil {
					lastErr = err
					continue
				}
				return conn, nil
			}
			return nil, lastErr
		},
	}
	return func(ctx context.Context, domain string) (netip.Addr, error) {
		addrs, err := resolver.LookupNetIP(ctx, "ip4", domain)
		if err != nil {
			return netip.Addr{}, err
		}
		if len(addrs) == 0 {
			return netip.Addr{}, &net.DNSError{Err: "no A records", Name: domain}
		}
		return addrs[0], nil
	}
}

// SessionCount returns the number of active UDP sessions (for status/stats).
func (r *UDPRelay) SessionCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.sessions)
}

// readFromRedirected is platform-specific: it reads a UDP packet and recovers
// the original destination address. Implemented in udprelay_{darwin,linux}.go.
func (r *UDPRelay) readFromRedirected(buf []byte) (n int, clientAddr netip.AddrPort, origDst netip.AddrPort, err error) {
	return r.readFromRedirectedPlatform(buf)
}

// UDPRelayStats exposes runtime info for status APIs.
type UDPRelayStats struct {
	Sessions int    `json:"sessions"`
	Listen   string `json:"listen"`
}

func (r *UDPRelay) Stats() UDPRelayStats {
	return UDPRelayStats{
		Sessions: r.SessionCount(),
		Listen:   r.Addr(),
	}
}

// FormatUDPListenAddr formats a UDP relay listen address from a port number.
func FormatUDPListenAddr(port int) string {
	return ":" + strconv.Itoa(port)
}
