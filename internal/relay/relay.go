package relay

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Options configures the transparent relay server.
type Options struct {
	ListenAddr string // e.g. ":17892"
	OrigDST    OrigDSTResolver
	Tracker    *Tracker
	// Dialer is the initial egress; swap live with SetDialer.
	Dialer   Dialer
	ViaProxy bool
	// FakeIPRange/LookupFakeIP enable fake-ip → domain resolution so the
	// egress proxy receives domain names instead of IPs. Optional.
	FakeIPRange  *netip.Prefix
	LookupFakeIP func(netip.Addr) (string, bool)
	Logger       *slog.Logger
}

// Server accepts transparently redirected TCP connections and relays them
// through the configured egress dialer.
type Server struct {
	origDST OrigDSTResolver
	tracker *Tracker
	logger  *slog.Logger

	listenAddr atomic.Value // string
	dialer     atomic.Pointer[dialerHolder]
	viaProxy   atomic.Bool

	fakeRange  atomic.Pointer[netip.Prefix]
	fakeLookup atomic.Value // func(netip.Addr) (string, bool)

	mu   sync.Mutex
	ln   *net.TCPListener
	done chan struct{}
}

// dialerHolder gives atomic.Pointer a stable concrete type while allowing the
// contained interface to hold different dialer implementations.
type dialerHolder struct {
	dialer Dialer
}

func New(opts Options) *Server {
	s := &Server{
		origDST: opts.OrigDST,
		tracker: opts.Tracker,
		logger:  opts.Logger,
		done:    make(chan struct{}),
	}
	if s.tracker == nil {
		s.tracker = NewTracker()
	}
	if s.logger == nil {
		s.logger = slog.Default()
	}
	addr := opts.ListenAddr
	if addr == "" {
		addr = ":17892"
	}
	s.listenAddr.Store(addr)
	if opts.Dialer != nil {
		s.dialer.Store(&dialerHolder{dialer: opts.Dialer})
	}
	s.viaProxy.Store(opts.ViaProxy)
	if opts.FakeIPRange != nil && opts.LookupFakeIP != nil {
		s.fakeRange.Store(opts.FakeIPRange)
		s.fakeLookup.Store(opts.LookupFakeIP)
	}
	return s
}

// SetDialer swaps the egress dialer live (direct ↔ proxy switch).
// Existing connections are unaffected.
func (s *Server) SetDialer(d Dialer, viaProxy bool) {
	s.dialer.Store(&dialerHolder{dialer: d})
	s.viaProxy.Store(viaProxy)
}

// SetFakeIP installs (or with nil prefix, removes) fake-ip domain lookup.
func (s *Server) SetFakeIP(prefix *netip.Prefix, lookup func(netip.Addr) (string, bool)) {
	if prefix == nil || lookup == nil {
		s.fakeRange.Store(nil)
		return
	}
	s.fakeRange.Store(prefix)
	s.fakeLookup.Store(lookup)
}

// Addr reports the bound address once ListenAndServe is running.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln == nil {
		return ""
	}
	return s.ln.Addr().String()
}

// Tracker exposes the connection tracker for stats.
func (s *Server) Tracker() *Tracker { return s.tracker }

// ListenAndServe blocks until ctx is cancelled or the listener fails.
func (s *Server) ListenAndServe(ctx context.Context) error {
	laddr, _ := net.ResolveTCPAddr("tcp4", s.listenAddr.Load().(string))
	ln, err := net.ListenTCP("tcp4", laddr)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.ln = ln
	s.mu.Unlock()

	go func() {
		select {
		case <-ctx.Done():
			_ = ln.Close()
		case <-s.done:
		}
	}()
	s.logger.Info("透明转发已启动", "listen", ln.Addr().String())

	var backoff time.Duration
	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				if backoff == 0 {
					backoff = 5 * time.Millisecond
				} else {
					backoff *= 2
				}
				if backoff > time.Second {
					backoff = time.Second
				}
				time.Sleep(backoff)
				continue
			}
			return err
		}
		backoff = 0
		go s.handle(conn)
	}
}

// Close stops the listener.
func (s *Server) Close() error {
	select {
	case <-s.done:
	default:
		close(s.done)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln != nil {
		return s.ln.Close()
	}
	return nil
}

func (s *Server) handle(client *net.TCPConn) {
	defer client.Close()
	_ = client.SetKeepAlive(true)
	_ = client.SetKeepAlivePeriod(60 * time.Second)

	orig, err := s.origDST.Resolve(client)
	if err != nil {
		s.logger.Warn("无法恢复原始目标，断开连接", "src", client.RemoteAddr(), "err", err)
		return
	}
	s.logger.Debug("接受连接", "src", client.RemoteAddr(), "orig", orig)

	host := orig.Addr().String()
	if prefix := s.fakeRange.Load(); prefix != nil && prefix.Contains(orig.Addr()) {
		lookup, _ := s.fakeLookup.Load().(func(netip.Addr) (string, bool))
		domain, ok := lookup(orig.Addr())
		if !ok {
			s.logger.Warn("fake-ip 映射缺失，断开连接(设备应重新解析 DNS)",
				"src", client.RemoteAddr(), "fake_ip", orig.Addr())
			return
		}
		host = domain
		s.logger.Debug("fake-ip 反查", "src", client.RemoteAddr(), "fake_ip", orig.Addr(), "domain", domain)
	}

	var dialer Dialer
	if holder := s.dialer.Load(); holder != nil {
		dialer = holder.dialer
	}
	if dialer == nil {
		s.logger.Error("未配置出口 dialer", "src", client.RemoteAddr())
		return
	}

	target := net.JoinHostPort(host, strconv.Itoa(int(orig.Port())))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	upstream, err := dialer.DialContext(ctx, "tcp", target)
	cancel()
	if err != nil {
		s.logger.Warn("出口拨号失败", "src", client.RemoteAddr(), "target", target, "err", err)
		return
	}
	defer upstream.Close()
	s.logger.Debug("出口已建立", "src", client.RemoteAddr(), "target", target, "via_proxy", s.viaProxy.Load())
	if uc, ok := upstream.(*net.TCPConn); ok {
		_ = uc.SetNoDelay(true)
	}

	srcIP := ""
	if ta, ok := client.RemoteAddr().(*net.TCPAddr); ok {
		srcIP = ta.IP.String()
	}
	tc := s.tracker.Open(srcIP, host, int(orig.Port()), s.viaProxy.Load())
	defer tc.Close()

	pipe(client, upstream, tc)
}
