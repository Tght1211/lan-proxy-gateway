package relay

import (
	"context"
	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxRecentConnections = 240
	maxTrafficPoints     = 360
)

// ConnInfo is a point-in-time view of one relayed connection.
type ConnInfo struct {
	ID        uint64     `json:"id"`
	SrcIP     string     `json:"src_ip"`
	DstHost   string     `json:"dst_host"`
	DstPort   int        `json:"dst_port"`
	Service   string     `json:"service"`
	Up        int64      `json:"up"`
	Down      int64      `json:"down"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	ViaProxy  bool       `json:"via_proxy"`
}

// TrafficPoint is one five-second throughput sample.
type TrafficPoint struct {
	At   time.Time `json:"at"`
	Up   int64     `json:"up"`
	Down int64     `json:"down"`
}

// UsageAggregate groups completed connections for one device or service.
type UsageAggregate struct {
	Name        string    `json:"name"`
	Up          int64     `json:"up"`
	Down        int64     `json:"down"`
	Connections int64     `json:"connections"`
	LastSeen    time.Time `json:"last_seen"`
}

// Snapshot is the full tracker state for the dashboard/API.
type Snapshot struct {
	UpTotal   int64            `json:"up_total"`
	DownTotal int64            `json:"down_total"`
	Active    []ConnInfo       `json:"active"`
	Recent    []ConnInfo       `json:"recent"`
	Traffic   []TrafficPoint   `json:"traffic"`
	Devices   []UsageAggregate `json:"devices"`
	Services  []UsageAggregate `json:"services"`
}

// Tracker keeps live and bounded historical telemetry in memory. Nothing is
// persisted or sent off-host; restarting the daemon starts a fresh session.
type Tracker struct {
	mu        sync.Mutex
	conns     map[uint64]*TrackedConn
	recent    []ConnInfo
	traffic   []TrafficPoint
	devices   map[string]*UsageAggregate
	services  map[string]*UsageAggregate
	nextID    uint64
	upTotal   atomic.Int64
	downTotal atomic.Int64
}

func NewTracker() *Tracker {
	return &Tracker{
		conns:    map[uint64]*TrackedConn{},
		recent:   make([]ConnInfo, 0),
		traffic:  make([]TrafficPoint, 0),
		devices:  map[string]*UsageAggregate{},
		services: map[string]*UsageAggregate{},
	}
}

// StartSampling records throughput deltas until ctx is cancelled.
func (t *Tracker) StartSampling(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	lastUp, lastDown := t.upTotal.Load(), t.downTotal.Load()
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case at := <-ticker.C:
				up, down := t.upTotal.Load(), t.downTotal.Load()
				t.mu.Lock()
				t.traffic = appendBounded(t.traffic, TrafficPoint{At: at, Up: up - lastUp, Down: down - lastDown}, maxTrafficPoints)
				t.mu.Unlock()
				lastUp, lastDown = up, down
			}
		}
	}()
}

// Open registers a new connection; Close on the returned handle archives it.
func (t *Tracker) Open(srcIP, dstHost string, dstPort int, viaProxy bool) *TrackedConn {
	t.mu.Lock()
	t.nextID++
	c := &TrackedConn{
		t:         t,
		id:        t.nextID,
		srcIP:     srcIP,
		dstHost:   dstHost,
		dstPort:   dstPort,
		service:   classifyService(dstHost),
		viaProxy:  viaProxy,
		startedAt: time.Now(),
	}
	t.conns[c.id] = c
	t.mu.Unlock()
	return c
}

func (t *Tracker) Snapshot() Snapshot {
	t.mu.Lock()
	out := Snapshot{
		Active:   make([]ConnInfo, 0, len(t.conns)),
		Recent:   append([]ConnInfo(nil), t.recent...),
		Traffic:  append([]TrafficPoint(nil), t.traffic...),
		Devices:  aggregateSlice(t.devices),
		Services: aggregateSlice(t.services),
	}
	for _, c := range t.conns {
		out.Active = append(out.Active, c.info())
	}
	t.mu.Unlock()
	sort.Slice(out.Active, func(i, j int) bool { return out.Active[i].StartedAt.After(out.Active[j].StartedAt) })
	out.UpTotal = t.upTotal.Load()
	out.DownTotal = t.downTotal.Load()
	return out
}

// TrackedConn is one live connection's counters.
type TrackedConn struct {
	t         *Tracker
	id        uint64
	srcIP     string
	dstHost   string
	dstPort   int
	service   string
	viaProxy  bool
	startedAt time.Time
	up        atomic.Int64
	down      atomic.Int64
	closed    atomic.Bool
}

func (c *TrackedConn) AddUp(n int64) {
	c.up.Add(n)
	c.t.upTotal.Add(n)
}

func (c *TrackedConn) AddDown(n int64) {
	c.down.Add(n)
	c.t.downTotal.Add(n)
}

func (c *TrackedConn) Close() {
	if !c.closed.CompareAndSwap(false, true) {
		return
	}
	now := time.Now()
	info := c.info()
	info.EndedAt = &now
	c.t.mu.Lock()
	delete(c.t.conns, c.id)
	c.t.recent = appendBoundedFront(c.t.recent, info, maxRecentConnections)
	updateAggregate(c.t.devices, c.srcIP, info, now)
	updateAggregate(c.t.services, c.service, info, now)
	c.t.mu.Unlock()
}

func (c *TrackedConn) Up() int64   { return c.up.Load() }
func (c *TrackedConn) Down() int64 { return c.down.Load() }

func (c *TrackedConn) info() ConnInfo {
	return ConnInfo{
		ID: c.id, SrcIP: c.srcIP, DstHost: c.dstHost, DstPort: c.dstPort,
		Service: c.service, Up: c.up.Load(), Down: c.down.Load(),
		StartedAt: c.startedAt, ViaProxy: c.viaProxy,
	}
}

func updateAggregate(target map[string]*UsageAggregate, name string, info ConnInfo, now time.Time) {
	if name == "" {
		name = "未知"
	}
	a := target[name]
	if a == nil {
		a = &UsageAggregate{Name: name}
		target[name] = a
	}
	a.Up += info.Up
	a.Down += info.Down
	a.Connections++
	a.LastSeen = now
}

func aggregateSlice(source map[string]*UsageAggregate) []UsageAggregate {
	out := make([]UsageAggregate, 0, len(source))
	for _, item := range source {
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Up+out[i].Down > out[j].Up+out[j].Down })
	return out
}

func appendBounded[T any](items []T, item T, limit int) []T {
	items = append(items, item)
	if len(items) > limit {
		copy(items, items[len(items)-limit:])
		items = items[:limit]
	}
	return items
}

func appendBoundedFront[T any](items []T, item T, limit int) []T {
	items = append(items, item)
	copy(items[1:], items[:len(items)-1])
	items[0] = item
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

// classifyService reports an observable network service, not a process name on
// the remote device. Unknown domains are reduced to a readable registrable-like
// suffix without requiring an external classification service.
func classifyService(host string) string {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if parsed := net.ParseIP(host); parsed != nil || host == "" {
		return "未识别流量"
	}
	patterns := []struct {
		name     string
		suffixes []string
	}{
		{"YouTube", []string{"youtube.com", "googlevideo.com", "ytimg.com", "youtu.be"}},
		{"Netflix", []string{"netflix.com", "nflxvideo.net", "nflximg.net", "nflxso.net"}},
		{"Apple", []string{"apple.com", "icloud.com", "mzstatic.com", "apple-dns.net"}},
		{"Nintendo", []string{"nintendo.net", "nintendo.com"}},
		{"PlayStation", []string{"playstation.net", "playstation.com", "sonyentertainmentnetwork.com"}},
		{"Steam", []string{"steampowered.com", "steamcontent.com", "steamstatic.com"}},
		{"TikTok", []string{"tiktok.com", "tiktokcdn.com", "byteoversea.com"}},
		{"哔哩哔哩", []string{"bilibili.com", "bilivideo.com", "hdslb.com"}},
		{"GitHub", []string{"github.com", "githubusercontent.com", "githubassets.com"}},
		{"Microsoft", []string{"microsoft.com", "live.com", "windows.net", "xboxlive.com"}},
		{"Google", []string{"google.com", "googleapis.com", "gstatic.com"}},
		{"Cloudflare", []string{"cloudflare.com", "cloudflare-dns.com"}},
		{"Telegram", []string{"telegram.org", "t.me"}},
		{"Discord", []string{"discord.com", "discordapp.com", "discordapp.net"}},
		{"Spotify", []string{"spotify.com", "scdn.co"}},
		{"Meta", []string{"facebook.com", "instagram.com", "fbcdn.net"}},
	}
	for _, pattern := range patterns {
		for _, suffix := range pattern.suffixes {
			if host == suffix || strings.HasSuffix(host, "."+suffix) {
				return pattern.name
			}
		}
	}
	parts := strings.Split(host, ".")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], ".")
	}
	return host
}
