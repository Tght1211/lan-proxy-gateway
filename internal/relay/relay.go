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

const missingFakeIPLogInterval = time.Minute

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
	// LookupRealIP labels telemetry for recently forwarded DNS answers. It
	// never changes the address passed to the egress dialer.
	LookupRealIP func(netip.Addr) (string, bool)
	// OnFallbackSuccess fires when a default-proxy connection whose proxy dial
	// failed was retried directly and succeeded. Optional; used for route
	// auto-learning.
	OnFallbackSuccess func(host string)
	Logger            *slog.Logger
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
	routing    atomic.Pointer[routingPolicy]

	fakeRange  atomic.Pointer[netip.Prefix]
	fakeLookup atomic.Value // func(netip.Addr) (string, bool)
	realLookup atomic.Value // func(netip.Addr) (string, bool)
	missingMu  sync.Mutex
	missingLog map[netip.Addr]time.Time

	onFallbackSuccess atomic.Value // func(string)

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
		origDST:    opts.OrigDST,
		tracker:    opts.Tracker,
		logger:     opts.Logger,
		done:       make(chan struct{}),
		missingLog: make(map[netip.Addr]time.Time),
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
	if opts.LookupRealIP != nil {
		s.realLookup.Store(opts.LookupRealIP)
	}
	if opts.OnFallbackSuccess != nil {
		s.onFallbackSuccess.Store(opts.OnFallbackSuccess)
	}
	return s
}

// SetRealIPLookup installs an optional DNS observation lookup used only for
// connection labels and service aggregation.
func (s *Server) SetRealIPLookup(lookup func(netip.Addr) (string, bool)) {
	if lookup != nil {
		s.realLookup.Store(lookup)
	}
}

// SetDialer swaps the egress dialer live (direct ↔ proxy switch).
// Existing connections are unaffected.
func (s *Server) SetDialer(d Dialer, viaProxy bool) {
	s.dialer.Store(&dialerHolder{dialer: d})
	s.viaProxy.Store(viaProxy)
}

// SetRouting atomically replaces the domain routing policy for new connections.
func (s *Server) SetRouting(defaultAction string, direct, proxy Dialer, rules []RouteRule) {
	s.routing.Store(&routingPolicy{
		defaultAction: defaultAction,
		direct:        direct,
		proxy:         proxy,
		rules:         compileRules(rules),
	})
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
	dstIP := orig.Addr()
	if prefix := s.fakeRange.Load(); prefix != nil && prefix.Contains(orig.Addr()) {
		lookup, _ := s.fakeLookup.Load().(func(netip.Addr) (string, bool))
		domain, ok := lookup(orig.Addr())
		if !ok {
			if s.shouldLogMissingFakeIP(orig.Addr(), time.Now()) {
				s.logger.Warn("fake-ip 映射缺失，断开连接(设备应重新解析 DNS)",
					"src", client.RemoteAddr(), "fake_ip", orig.Addr())
			}
			return
		}
		host = domain
		dstIP = netip.Addr{}
		s.logger.Debug("fake-ip 反查", "src", client.RemoteAddr(), "fake_ip", orig.Addr(), "domain", domain)
	}
	routeHost := host
	if ip, err := netip.ParseAddr(host); err == nil {
		if lookup, _ := s.realLookup.Load().(func(netip.Addr) (string, bool)); lookup != nil {
			if domain, ok := lookup(ip); ok {
				routeHost = domain
			}
		}
	}

	srcIP := ""
	if ta, ok := client.RemoteAddr().(*net.TCPAddr); ok {
		srcIP = ta.IP.String()
	}

	var dialer Dialer
	viaProxy := s.viaProxy.Load()
	rejected := false
	matchedRule := false
	var fallbackDialer Dialer
	if policy := s.routing.Load(); policy != nil {
		dialer, viaProxy, rejected, matchedRule = policy.selectDialer(routeHost, dstIP)
		// 默认出口是代理且未命中显式规则时，代理拨号失败允许直连兜底一次：
		// 国内站点被代理到海外出口常见的"连接失败"可由直连救回。
		if viaProxy && !matchedRule && policy.direct != nil {
			fallbackDialer = policy.direct
		}
	} else if holder := s.dialer.Load(); holder != nil {
		dialer = holder.dialer
	}
	if rejected {
		s.logger.Info("连接被路由规则拒绝", "src", client.RemoteAddr(), "target", host)
		s.tracker.RecordRejected(srcIP, routeHost, int(orig.Port()))
		return
	}
	if dialer == nil {
		s.logger.Error("未配置出口 dialer", "src", client.RemoteAddr())
		return
	}

	target := net.JoinHostPort(host, strconv.Itoa(int(orig.Port())))
	upstream, err := dialTarget(dialer, target)
	fellBack := false
	if err != nil && fallbackDialer != nil {
		proxyErr := err
		s.logger.Info("代理拨号失败，尝试直连回退", "src", client.RemoteAddr(), "target", target, "err", proxyErr)
		upstream, err = dialTarget(fallbackDialer, target)
		if err == nil {
			fellBack = true
			viaProxy = false
			s.notifyFallbackSuccess(routeHost)
		} else {
			s.logger.Warn("直连回退也失败", "src", client.RemoteAddr(), "target", target, "err", err)
			err = proxyErr
		}
	}
	if err != nil {
		s.logger.Warn("出口拨号失败", "src", client.RemoteAddr(), "target", target, "err", err)
		s.tracker.RecordDialFailure(srcIP, routeHost, int(orig.Port()), viaProxy, classifyDialError(err, viaProxy))
		return
	}
	defer upstream.Close()
	s.logger.Debug("出口已建立", "src", client.RemoteAddr(), "target", target, "via_proxy", viaProxy, "fallback", fellBack)
	if uc, ok := upstream.(*net.TCPConn); ok {
		_ = uc.SetNoDelay(true)
	}

	observedHost := routeHost
	tc := s.tracker.Open(srcIP, observedHost, int(orig.Port()), viaProxy)
	if fellBack {
		tc.MarkFallback()
	}
	defer tc.Close()

	pipe(client, upstream, tc)
}

func dialTarget(d Dialer, target string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return d.DialContext(ctx, "tcp", target)
}

// notifyFallbackSuccess reports a successful proxy→direct fallback for domain
// targets only; IP literals carry no reusable routing signal.
func (s *Server) notifyFallbackSuccess(host string) {
	cb, _ := s.onFallbackSuccess.Load().(func(string))
	if cb == nil {
		return
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return
	}
	// 学习器要做持久化和配置写入，不能阻塞转发链路
	go cb(host)
}

func (s *Server) shouldLogMissingFakeIP(ip netip.Addr, now time.Time) bool {
	s.missingMu.Lock()
	defer s.missingMu.Unlock()
	if last, ok := s.missingLog[ip]; ok && now.Sub(last) < missingFakeIPLogInterval {
		return false
	}
	if len(s.missingLog) >= 4096 {
		cutoff := now.Add(-missingFakeIPLogInterval)
		for addr, last := range s.missingLog {
			if last.Before(cutoff) {
				delete(s.missingLog, addr)
			}
		}
		if len(s.missingLog) >= 4096 {
			clear(s.missingLog)
		}
	}
	s.missingLog[ip] = now
	return true
}
