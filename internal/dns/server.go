// Package dns serves the LAN's DNS on the gateway host. In proxy mode it
// answers A queries with fake-ip addresses (so the relay can recover the
// original domain and hand it to the upstream proxy); in direct mode it is a
// plain racing forwarder. Loopback clients always get real answers.
package dns

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
)

// DefaultFakeIPRange is the fake-ip answer pool (mihomo-compatible range).
const DefaultFakeIPRange = "198.18.0.0/16"

const (
	defaultFakeIPIdleTTL = 7 * 24 * time.Hour
	defaultFakeIPMaxSize = 60000
	cacheFlushInterval   = 5 * time.Minute
	cacheTouchInterval   = 5 * time.Minute
)

// Stats are the server's counters, exposed via the status API.
type Stats struct {
	Queries      int64 `json:"queries"`
	FakeAnswered int64 `json:"fake_answered"`
	Forwarded    int64 `json:"forwarded"`
	Failures     int64 `json:"failures"`
	PoolSize     int   `json:"pool_size"`
}

// Options configures the DNS server.
type Options struct {
	Addr           string // listen address, default ":53"
	Upstreams      []string
	FakeIPRange    netip.Prefix
	FakeIPEnabled  bool
	FakeIPFilter   []string // extra suffixes that must resolve for real
	IdleTTL        time.Duration
	MaxPoolEntries int
	CachePath      string
	Logger         *slog.Logger
}

// Server is a concurrent UDP+TCP DNS server.
type Server struct {
	addr   string
	filter *suffixFilter
	prefix netip.Prefix
	logger *slog.Logger

	pool      *fakeIPPool
	forwarder *forwarder
	fakeOn    atomic.Bool
	cachePath string
	cacheMu   sync.Mutex

	queries      atomic.Int64
	fakeAnswered atomic.Int64
	forwarded    atomic.Int64
	failures     atomic.Int64

	mu      sync.Mutex
	udp     *dns.Server
	tcp     *dns.Server
	started bool
}

func New(opts Options) *Server {
	prefix := opts.FakeIPRange
	if !prefix.Addr().IsValid() || prefix.Addr().IsUnspecified() {
		prefix = netip.MustParsePrefix(DefaultFakeIPRange)
	}
	idle := opts.IdleTTL
	if idle <= 0 {
		idle = defaultFakeIPIdleTTL
	}
	maxEntries := opts.MaxPoolEntries
	if maxEntries <= 0 {
		maxEntries = defaultFakeIPMaxSize
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	addr := opts.Addr
	if addr == "" {
		addr = ":53"
	}
	s := &Server{
		addr:      addr,
		filter:    newSuffixFilter(opts.FakeIPFilter),
		prefix:    prefix,
		logger:    logger,
		pool:      newFakeIPPool(prefix, idle, maxEntries),
		forwarder: newForwarder(opts.Upstreams),
		cachePath: opts.CachePath,
	}
	if count, err := s.loadFakeIPCache(time.Now()); err != nil {
		logger.Warn("fake-ip 缓存恢复失败，使用空映射", "path", opts.CachePath, "err", err)
	} else if count > 0 {
		logger.Info("fake-ip 缓存已恢复", "entries", count)
	}
	s.fakeOn.Store(opts.FakeIPEnabled)
	return s
}

// SetFakeIPEnabled toggles fake-ip answering live (proxy ↔ direct switch).
func (s *Server) SetFakeIPEnabled(on bool) { s.fakeOn.Store(on) }

// SetUpstreams replaces the upstream resolver list.
func (s *Server) SetUpstreams(upstreams []string) { s.forwarder.Set(upstreams) }

// LookupFakeIP resolves a fake address to its domain; used by the relay.
func (s *Server) LookupFakeIP(ip netip.Addr) (string, bool) {
	return s.pool.Lookup(ip, time.Now())
}

// FakeIPRange returns the configured fake-ip prefix.
func (s *Server) FakeIPRange() netip.Prefix { return s.prefix }

// Stats returns current counters.
func (s *Server) Stats() Stats {
	return Stats{
		Queries:      s.queries.Load(),
		FakeAnswered: s.fakeAnswered.Load(),
		Forwarded:    s.forwarded.Load(),
		Failures:     s.failures.Load(),
		PoolSize:     s.pool.Len(),
	}
}

// ListenAndServe binds the configured address (UDP and TCP) and serves until
// ctx is cancelled, then shuts both listeners down.
func (s *Server) ListenAndServe(ctx context.Context) error {
	ua, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		return err
	}
	pc, err := net.ListenUDP("udp", ua)
	if err != nil {
		return err
	}
	ta, err := net.ResolveTCPAddr("tcp", s.addr)
	if err != nil {
		pc.Close()
		return err
	}
	l, err := net.ListenTCP("tcp", ta)
	if err != nil {
		pc.Close()
		return err
	}
	return s.Serve(ctx, pc, l)
}

// Serve runs the handler on pre-bound listeners (useful for tests with
// ephemeral ports). Blocks until ctx is cancelled or a listener fails.
func (s *Server) Serve(ctx context.Context, pc *net.UDPConn, l *net.TCPListener) error {
	s.mu.Lock()
	s.udp = &dns.Server{PacketConn: pc, Net: "udp", Handler: dns.HandlerFunc(s.serveDNS)}
	s.tcp = &dns.Server{Listener: l, Net: "tcp", Handler: dns.HandlerFunc(s.serveDNS)}
	s.started = true
	s.mu.Unlock()

	errCh := make(chan error, 2)
	go func() { errCh <- s.udp.ActivateAndServe() }()
	go func() { errCh <- s.tcp.ActivateAndServe() }()
	go s.persistFakeIPCache(ctx)
	s.logger.Info("DNS 服务已启动", "udp", pc.LocalAddr(), "tcp", l.Addr(), "fake_ip", s.fakeOn.Load())

	select {
	case <-ctx.Done():
		s.Shutdown()
		return nil
	case err := <-errCh:
		s.Shutdown()
		return err
	}
}

// Shutdown stops both listeners.
func (s *Server) Shutdown() {
	s.mu.Lock()
	udp, tcp := s.udp, s.tcp
	s.mu.Unlock()
	if udp != nil {
		_ = udp.Shutdown()
	}
	if tcp != nil {
		_ = tcp.Shutdown()
	}
	if err := s.flushFakeIPCache(time.Now()); err != nil {
		s.logger.Warn("fake-ip 缓存保存失败", "path", s.cachePath, "err", err)
	}
}

func (s *Server) serveDNS(w dns.ResponseWriter, r *dns.Msg) {
	s.queries.Add(1)
	if len(r.Question) == 0 {
		m := new(dns.Msg).SetRcode(r, dns.RcodeFormatError)
		_ = w.WriteMsg(m)
		return
	}
	q := r.Question[0]
	clientIP := remoteIP(w.RemoteAddr())
	now := time.Now()

	act := decide(clientIP, q.Name, q.Qtype, s.fakeOn.Load(), s.filter, s.prefix)
	s.logger.Debug("DNS 查询", "client", clientIP, "name", q.Name, "qtype", dns.TypeToString[q.Qtype], "action", act)
	switch act {
	case actionFakeIP:
		ip := s.pool.Get(q.Name, now)
		m := new(dns.Msg).SetReply(r)
		m.Compress = true
		m.Answer = []dns.RR{&dns.A{
			Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 1},
			A:   net.ParseIP(ip.String()).To4(),
		}}
		s.fakeAnswered.Add(1)
		_ = w.WriteMsg(m)

	case actionNODATA:
		m := new(dns.Msg).SetReply(r)
		m.Compress = true
		_ = w.WriteMsg(m)

	case actionPTRFromMap:
		ip, _ := parseReverseIPv4(q.Name)
		m := new(dns.Msg).SetReply(r)
		m.Compress = true
		if name, ok := s.pool.Lookup(ip, now); ok {
			m.Answer = []dns.RR{&dns.PTR{
				Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: 60},
				Ptr: name,
			}}
		}
		_ = w.WriteMsg(m)

	default: // actionForward
		s.forward(w, r)
	}
}

func (s *Server) forward(w dns.ResponseWriter, r *dns.Msg) {
	s.forwarded.Add(1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := s.forwarder.exchange(ctx, r)
	if err != nil {
		s.failures.Add(1)
		s.logger.Debug("DNS 转发失败", "q", firstQuestion(r), "err", err)
		m := new(dns.Msg).SetRcode(r, dns.RcodeServerFailure)
		_ = w.WriteMsg(m)
		return
	}
	resp.Id = r.Id
	resp.Compress = true
	if err := w.WriteMsg(resp); err != nil {
		var netErr net.Error
		if !errors.As(err, &netErr) {
			s.logger.Debug("DNS 应答写入失败", "err", err)
		}
	}
}

func firstQuestion(r *dns.Msg) string {
	if len(r.Question) == 0 {
		return ""
	}
	return r.Question[0].Name
}

func remoteIP(a net.Addr) netip.Addr {
	host, _, err := net.SplitHostPort(a.String())
	if err != nil {
		host = a.String()
	}
	ip, err := netip.ParseAddr(strings.Trim(host, "[]"))
	if err != nil {
		return netip.AddrFrom4([4]byte{127, 0, 0, 1})
	}
	return ip
}
