package dns

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// ---------- decide() ----------

var (
	lanClient = netip.MustParseAddr("192.168.1.50")
	loopback  = netip.MustParseAddr("127.0.0.1")
	prefix    = netip.MustParsePrefix("198.18.0.0/16")
)

func TestDecide(t *testing.T) {
	filter := newSuffixFilter(nil)
	cases := []struct {
		name   string
		client netip.Addr
		qname  string
		qtype  uint16
		fakeOn bool
		want   action
	}{
		{"fake A for LAN client", lanClient, "example.com.", dns.TypeA, true, actionFakeIP},
		{"no fake when disabled", lanClient, "example.com.", dns.TypeA, false, actionForward},
		{"loopback always real", loopback, "example.com.", dns.TypeA, true, actionForward},
		{"filtered suffix forwards", lanClient, "router.lan.", dns.TypeA, true, actionForward},
		{"filtered exact", lanClient, "lan.", dns.TypeA, true, actionForward},
		{"AAAA suppressed even off", lanClient, "example.com.", dns.TypeAAAA, false, actionNODATA},
		{"AAAA suppressed for loopback", loopback, "example.com.", dns.TypeAAAA, true, actionNODATA},
		{"PTR in fake range from map", lanClient, "7.0.18.198.in-addr.arpa.", dns.TypePTR, true, actionPTRFromMap},
		{"PTR outside fake range forwards", lanClient, "1.1.1.8.in-addr.arpa.", dns.TypePTR, true, actionForward},
		{"TXT forwards", lanClient, "example.com.", dns.TypeTXT, true, actionForward},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := decide(c.client, c.qname, c.qtype, c.fakeOn, filter, prefix)
			if got != c.want {
				t.Fatalf("decide = %v, want %v", got, c.want)
			}
		})
	}
}

func TestDecideUserFilter(t *testing.T) {
	filter := newSuffixFilter([]string{"corp.example", "Skip.Me."})
	if !filter.match("host.corp.example.") {
		t.Fatal("user suffix should match subdomain")
	}
	if !filter.match("skip.me.") {
		t.Fatal("user suffix should be case-insensitive")
	}
	if filter.match("notskip.me.") {
		t.Fatal("must respect label boundary")
	}
	if filter.match("example.com.") {
		t.Fatal("non-listed name should not match")
	}
}

func TestParseReverseIPv4(t *testing.T) {
	ip, err := parseReverseIPv4("7.0.18.198.in-addr.arpa.")
	if err != nil || ip != netip.MustParseAddr("198.18.0.7") {
		t.Fatalf("got %v %v", ip, err)
	}
	if _, err := parseReverseIPv4("example.com."); err == nil {
		t.Fatal("want error for non-reverse name")
	}
}

// ---------- fakeIPPool ----------

func TestFakeIPPoolStableAndReverse(t *testing.T) {
	p := newFakeIPPool(prefix, time.Minute, 100)
	now := time.Now()
	a1 := p.Get("a.com.", now)
	a2 := p.Get("b.com.", now)
	a1again := p.Get("a.com.", now)
	if a1 != a1again {
		t.Fatal("same name must get stable ip")
	}
	if a1 == a2 {
		t.Fatal("different names must differ")
	}
	if !prefix.Contains(a1) {
		t.Fatalf("%s outside %s", a1, prefix)
	}
	if name, ok := p.Lookup(a1, now); !ok || name != "a.com." {
		t.Fatalf("reverse lookup = %q %v", name, ok)
	}
}

func TestFakeIPPoolIdleExpiry(t *testing.T) {
	p := newFakeIPPool(prefix, 20*time.Millisecond, 100)
	now := time.Now()
	ip := p.Get("a.com.", now)
	if _, ok := p.Lookup(ip, now.Add(30*time.Millisecond)); ok {
		t.Fatal("expired mapping should be gone")
	}
}

func TestFakeIPPoolLRUEviction(t *testing.T) {
	p := newFakeIPPool(prefix, time.Hour, 3)
	now := time.Now()
	first := p.Get("a.com.", now)
	p.Get("b.com.", now.Add(time.Second))
	p.Get("c.com.", now.Add(2*time.Second))
	// touch a.com so it is not the LRU
	p.Lookup(first, now.Add(3*time.Second))
	p.Get("d.com.", now.Add(4*time.Second)) // evicts b.com (LRU)
	if _, ok := p.Lookup(first, now.Add(5*time.Second)); !ok {
		t.Fatal("recently used entry must survive eviction")
	}
	if p.Len() > 3 {
		t.Fatalf("pool size %d exceeds cap", p.Len())
	}
}

func TestDefaultFakeIPRetentionCoversLongDeviceCaches(t *testing.T) {
	now := time.Now()
	s := New(Options{FakeIPRange: prefix})
	ip := s.pool.Get("cached.example.", now)
	if _, ok := s.pool.Lookup(ip, now.Add(24*time.Hour)); !ok {
		t.Fatal("default retention must survive a device cache lasting 24 hours")
	}
}

func TestFakeIPSnapshotKeepsConcurrentDirtyState(t *testing.T) {
	now := time.Now()
	p := newFakeIPPool(prefix, time.Hour, 100)
	p.Get("a.example.", now)
	p.snapshot(now)
	if p.dirty.Load() {
		t.Fatal("snapshot should mark persisted state clean")
	}
	p.Get("b.example.", now.Add(time.Second))
	if !p.dirty.Load() {
		t.Fatal("mutation after snapshot must remain dirty for the next flush")
	}
}

func TestFakeIPTouchMarksCacheDirtyAfterPersistenceWindow(t *testing.T) {
	now := time.Now()
	p := newFakeIPPool(prefix, time.Hour, 100)
	ip := p.Get("a.example.", now)
	p.snapshot(now)
	if _, ok := p.Lookup(ip, now.Add(cacheTouchInterval-time.Second)); !ok {
		t.Fatal("mapping unexpectedly missing")
	}
	if p.dirty.Load() {
		t.Fatal("short-lived touch should not force a full cache rewrite")
	}
	if _, ok := p.Lookup(ip, now.Add(cacheTouchInterval)); !ok {
		t.Fatal("mapping unexpectedly missing")
	}
	if !p.dirty.Load() {
		t.Fatal("touch after persistence window must schedule a cache rewrite")
	}
}

func TestFakeIPCacheSurvivesRestart(t *testing.T) {
	cachePath := t.TempDir() + "/fakeip-cache.json"
	now := time.Now().Round(time.Second)
	s1 := New(Options{FakeIPRange: prefix, FakeIPEnabled: true, CachePath: cachePath})
	ipA := s1.pool.Get("a.example.", now)
	ipB := s1.pool.Get("b.example.", now.Add(time.Second))
	s1.Shutdown()
	info, err := os.Stat(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("cache mode = %o, want 600", info.Mode().Perm())
	}

	s2 := New(Options{FakeIPRange: prefix, FakeIPEnabled: true, CachePath: cachePath})
	if name, ok := s2.LookupFakeIP(ipA); !ok || name != "a.example." {
		t.Fatalf("restored lookup = %q %v", name, ok)
	}
	if got := s2.pool.Get("b.example.", now.Add(3*time.Second)); got != ipB {
		t.Fatalf("stable restored IP = %s, want %s", got, ipB)
	}
	if got := s2.pool.Get("c.example.", now.Add(4*time.Second)); got == ipA || got == ipB {
		t.Fatalf("allocation cursor reused restored IP %s", got)
	}
}

func TestFakeIPCacheDropsExpiredAndInvalidEntries(t *testing.T) {
	cachePath := t.TempDir() + "/fakeip-cache.json"
	now := time.Now().Round(time.Second)
	snapshot := fakeIPCacheFile{
		Version: fakeIPCacheVersion,
		Prefix:  prefix.String(),
		Offset:  20,
		Entries: []fakeIPCacheEntry{
			{IP: netip.MustParseAddr("198.18.0.7"), Name: "fresh.example.", LastSeen: now.Add(-time.Hour)},
			{IP: netip.MustParseAddr("198.18.0.8"), Name: "expired.example.", LastSeen: now.Add(-2 * time.Hour)},
			{IP: netip.MustParseAddr("203.0.113.1"), Name: "outside.example.", LastSeen: now},
		},
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	s := New(Options{FakeIPRange: prefix, IdleTTL: 90 * time.Minute, CachePath: cachePath})
	if _, ok := s.pool.Lookup(netip.MustParseAddr("198.18.0.7"), now); !ok {
		t.Fatal("fresh entry should be restored")
	}
	if _, ok := s.pool.Lookup(netip.MustParseAddr("198.18.0.8"), now); ok {
		t.Fatal("expired entry should be dropped")
	}
	if s.pool.Len() != 1 {
		t.Fatalf("restored pool size = %d, want 1", s.pool.Len())
	}
}

func TestFakeIPCacheCorruptionDoesNotPreventServer(t *testing.T) {
	cachePath := t.TempDir() + "/fakeip-cache.json"
	if err := os.WriteFile(cachePath, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := New(Options{FakeIPRange: prefix, CachePath: cachePath})
	if got := s.pool.Get("example.com.", time.Now()); !prefix.Contains(got) {
		t.Fatalf("new mapping %s outside pool", got)
	}
}

// ---------- fake upstream + handler e2e ----------

func startFakeUpstream(t *testing.T, answers map[string]string) string {
	t.Helper()
	mux := dns.NewServeMux()
	mux.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		q := r.Question[0]
		if ip, ok := answers[q.Name]; ok && q.Qtype == dns.TypeA {
			m := new(dns.Msg).SetReply(r)
			m.Answer = []dns.RR{&dns.A{
				Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
				A:   net.ParseIP(ip).To4(),
			}}
			_ = w.WriteMsg(m)
			return
		}
		_ = w.WriteMsg(new(dns.Msg).SetRcode(r, dns.RcodeNameError))
	})
	pc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	srv := &dns.Server{PacketConn: pc, Net: "udp", Handler: mux}
	go srv.ActivateAndServe()
	t.Cleanup(func() { srv.Shutdown() })
	return pc.LocalAddr().String()
}

type fakeRW struct {
	remote net.Addr
	msg    *dns.Msg
}

func (f *fakeRW) LocalAddr() net.Addr         { return &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 53} }
func (f *fakeRW) RemoteAddr() net.Addr        { return f.remote }
func (f *fakeRW) WriteMsg(m *dns.Msg) error   { f.msg = m; return nil }
func (f *fakeRW) Write(b []byte) (int, error) { return len(b), nil }
func (f *fakeRW) Close() error                { return nil }
func (f *fakeRW) TsigStatus() error           { return nil }
func (f *fakeRW) TsigTimersOnly(bool)         {}
func (f *fakeRW) Hijack()                     {}

func queryMsg(name string, qtype uint16) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), qtype)
	return m
}

func lanRemote() net.Addr { return &net.UDPAddr{IP: net.ParseIP("192.168.1.50"), Port: 53000} }

func TestHandlerFakeIPAndLookup(t *testing.T) {
	upstream := startFakeUpstream(t, map[string]string{"real.example.": "93.184.216.34"})
	s := New(Options{Upstreams: []string{upstream}, FakeIPEnabled: true})

	rw := &fakeRW{remote: lanRemote()}
	s.serveDNS(rw, queryMsg("www.example.com", dns.TypeA))
	if rw.msg == nil || len(rw.msg.Answer) != 1 {
		t.Fatal("want one fake answer")
	}
	a, ok := rw.msg.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("answer type %T", rw.msg.Answer[0])
	}
	if a.Hdr.Ttl != 1 {
		t.Fatalf("fake TTL = %d, want 1", a.Hdr.Ttl)
	}
	fakeIP := netip.MustParseAddr(a.A.String())
	if !s.FakeIPRange().Contains(fakeIP) {
		t.Fatalf("%s outside fake range", fakeIP)
	}
	if name, ok := s.LookupFakeIP(fakeIP); !ok || name != "www.example.com." {
		t.Fatalf("relay lookup = %q %v", name, ok)
	}

	// PTR for the fake ip resolves back from the map
	rw2 := &fakeRW{remote: lanRemote()}
	rev, _ := dns.ReverseAddr(fakeIP.String())
	s.serveDNS(rw2, queryMsg(rev, dns.TypePTR))
	if len(rw2.msg.Answer) != 1 {
		t.Fatalf("want PTR answer, got %+v", rw2.msg.Answer)
	}
	if rw2.msg.Answer[0].(*dns.PTR).Ptr != "www.example.com." {
		t.Fatalf("PTR = %s", rw2.msg.Answer[0].(*dns.PTR).Ptr)
	}
}

func TestRealIPLookupFromForwardedAnswer(t *testing.T) {
	s := New(Options{})
	now := time.Now()
	msg := new(dns.Msg)
	msg.Answer = []dns.RR{&dns.A{
		Hdr: dns.RR_Header{Name: "media.example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 120},
		A:   net.ParseIP("203.0.113.25").To4(),
	}}
	s.rememberRealAnswers(msg, now)
	name, ok := s.LookupRealIP(netip.MustParseAddr("203.0.113.25"))
	if !ok || name != "media.example.com." {
		t.Fatalf("lookup = %q, %v", name, ok)
	}
}

func TestHandlerAAAASuppressed(t *testing.T) {
	s := New(Options{Upstreams: []string{"127.0.0.1:1"}, FakeIPEnabled: true})
	rw := &fakeRW{remote: lanRemote()}
	s.serveDNS(rw, queryMsg("example.com", dns.TypeAAAA))
	if rw.msg == nil || rw.msg.Rcode != dns.RcodeSuccess || len(rw.msg.Answer) != 0 {
		t.Fatalf("want NODATA, got %+v", rw.msg)
	}
}

func TestHandlerFilteredForwards(t *testing.T) {
	upstream := startFakeUpstream(t, map[string]string{"real.example.": "93.184.216.34"})
	s := New(Options{Upstreams: []string{upstream}, FakeIPEnabled: true})
	rw := &fakeRW{remote: lanRemote()}
	s.serveDNS(rw, queryMsg("real.example", dns.TypeA))
	// "real.example" is not in the filter, so with fake on it would be fake;
	// use loopback to force forwarding and verify the upstream answer arrives.
	rw2 := &fakeRW{remote: &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 53000}}
	s.serveDNS(rw2, queryMsg("real.example", dns.TypeA))
	if len(rw2.msg.Answer) != 1 || rw2.msg.Answer[0].(*dns.A).A.String() != "93.184.216.34" {
		t.Fatalf("want real upstream answer, got %+v", rw2.msg)
	}
}

func TestForwardRacesAndToleratesDeadUpstream(t *testing.T) {
	alive := startFakeUpstream(t, map[string]string{"real.example.": "93.184.216.34"})
	s := New(Options{Upstreams: []string{"127.0.0.1:1", alive}})
	rw := &fakeRW{remote: &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 53000}}
	s.serveDNS(rw, queryMsg("real.example", dns.TypeA))
	if len(rw.msg.Answer) != 1 {
		t.Fatalf("want answer despite dead upstream, got %+v", rw.msg)
	}
}

// e2e over real sockets on ephemeral ports
func TestServerEndToEnd(t *testing.T) {
	upstream := startFakeUpstream(t, map[string]string{"real.example.": "93.184.216.34"})
	s := New(Options{Upstreams: []string{upstream}, FakeIPEnabled: true})

	pc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(pc.LocalAddr().String())
	l, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: atoi(port)})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Serve(ctx, pc, l)
	time.Sleep(50 * time.Millisecond)

	c := &dns.Client{Net: "udp", Timeout: 2 * time.Second}
	resp, _, err := c.Exchange(queryMsg("real.example", dns.TypeA), "127.0.0.1:"+port)
	if err != nil {
		t.Fatal(err)
	}
	// loopback client → real answer, not fake
	if len(resp.Answer) != 1 || resp.Answer[0].(*dns.A).A.String() != "93.184.216.34" {
		t.Fatalf("got %+v", resp.Answer)
	}
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}
