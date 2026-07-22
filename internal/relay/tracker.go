package relay

import (
	"context"
	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/publicsuffix"
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
	Rejected  bool       `json:"rejected,omitempty"`
	Status    string     `json:"status,omitempty"`  // "" | "rejected" | "dial_failed"
	Failure   string     `json:"failure,omitempty"` // human-readable dial failure reason
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

// DeviceServiceAggregate groups service usage for one LAN device.
type DeviceServiceAggregate struct {
	Device   string           `json:"device"`
	Services []UsageAggregate `json:"services"`
}

// Snapshot is the full tracker state for the dashboard/API.
type Snapshot struct {
	UpTotal        int64                    `json:"up_total"`
	DownTotal      int64                    `json:"down_total"`
	Active         []ConnInfo               `json:"active"`
	Recent         []ConnInfo               `json:"recent"`
	Traffic        []TrafficPoint           `json:"traffic"`
	Devices        []UsageAggregate         `json:"devices"`
	Services       []UsageAggregate         `json:"services"`
	DeviceServices []DeviceServiceAggregate `json:"device_services"`
}

// Tracker keeps live and bounded historical telemetry in memory. Nothing is
// persisted or sent off-host; restarting the daemon starts a fresh session.
type Tracker struct {
	mu             sync.Mutex
	conns          map[uint64]*TrackedConn
	recent         []ConnInfo
	traffic        []TrafficPoint
	devices        map[string]*UsageAggregate
	services       map[string]*UsageAggregate
	deviceServices map[string]map[string]*UsageAggregate
	nextID         uint64
	upTotal        atomic.Int64
	downTotal      atomic.Int64
}

func NewTracker() *Tracker {
	return &Tracker{
		conns:          map[uint64]*TrackedConn{},
		recent:         make([]ConnInfo, 0),
		traffic:        make([]TrafficPoint, 0),
		devices:        map[string]*UsageAggregate{},
		services:       map[string]*UsageAggregate{},
		deviceServices: map[string]map[string]*UsageAggregate{},
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

// RecordRejected archives a connection refused by a routing rule. It appears
// in the recent history but never counts toward device/service usage.
func (t *Tracker) RecordRejected(srcIP, dstHost string, dstPort int) {
	t.recordTerminal(srcIP, dstHost, dstPort, "rejected", "", false)
}

// RecordDialFailure archives a connection whose egress dial failed.
func (t *Tracker) RecordDialFailure(srcIP, dstHost string, dstPort int, viaProxy bool, reason string) {
	t.recordTerminal(srcIP, dstHost, dstPort, "dial_failed", reason, viaProxy)
}

func (t *Tracker) recordTerminal(srcIP, dstHost string, dstPort int, status, failure string, viaProxy bool) {
	now := time.Now()
	t.mu.Lock()
	t.nextID++
	info := ConnInfo{
		ID: t.nextID, SrcIP: srcIP, DstHost: dstHost, DstPort: dstPort,
		Service: classifyService(dstHost), StartedAt: now, EndedAt: &now,
		ViaProxy: viaProxy, Rejected: status == "rejected",
		Status: status, Failure: failure,
	}
	t.recent = appendBoundedFront(t.recent, info, maxRecentConnections)
	t.mu.Unlock()
}

func (t *Tracker) Snapshot() Snapshot {
	t.mu.Lock()
	devices := cloneAggregates(t.devices)
	services := cloneAggregates(t.services)
	deviceServices := cloneDeviceServices(t.deviceServices)
	out := Snapshot{
		Active:  make([]ConnInfo, 0, len(t.conns)),
		Recent:  append([]ConnInfo(nil), t.recent...),
		Traffic: append([]TrafficPoint(nil), t.traffic...),
	}
	for _, c := range t.conns {
		info := c.info()
		out.Active = append(out.Active, info)
		updateAggregate(devices, c.srcIP, info, c.startedAt)
		updateAggregate(services, c.service, info, c.startedAt)
		updateDeviceService(deviceServices, c.srcIP, c.service, info, c.startedAt)
	}
	out.Devices = aggregateSlice(devices)
	out.Services = aggregateSlice(services)
	out.DeviceServices = deviceServiceSlice(deviceServices)
	t.mu.Unlock()
	sort.Slice(out.Active, func(i, j int) bool { return out.Active[i].StartedAt.After(out.Active[j].StartedAt) })
	out.UpTotal = t.upTotal.Load()
	out.DownTotal = t.downTotal.Load()
	return out
}

func cloneDeviceServices(source map[string]map[string]*UsageAggregate) map[string]map[string]*UsageAggregate {
	out := make(map[string]map[string]*UsageAggregate, len(source))
	for device, services := range source {
		out[device] = cloneAggregates(services)
	}
	return out
}

func cloneAggregates(source map[string]*UsageAggregate) map[string]*UsageAggregate {
	out := make(map[string]*UsageAggregate, len(source))
	for name, item := range source {
		copy := *item
		out[name] = &copy
	}
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
	updateDeviceService(c.t.deviceServices, c.srcIP, c.service, info, now)
	c.t.mu.Unlock()
}

func updateDeviceService(target map[string]map[string]*UsageAggregate, device, service string, info ConnInfo, now time.Time) {
	services := target[device]
	if services == nil {
		services = map[string]*UsageAggregate{}
		target[device] = services
	}
	updateAggregate(services, service, info, now)
}

func deviceServiceSlice(source map[string]map[string]*UsageAggregate) []DeviceServiceAggregate {
	out := make([]DeviceServiceAggregate, 0, len(source))
	for device, services := range source {
		out = append(out, DeviceServiceAggregate{Device: device, Services: aggregateSlice(services)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Device < out[j].Device })
	return out
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
	if host == "" {
		return "未知目标"
	}
	if parsed := net.ParseIP(host); parsed != nil {
		return "未解析域名"
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
		{"抖音", []string{"douyin.com", "douyinvod.com", "douyinstatic.com", "amemv.com", "snssdk.com", "byteimg.com"}},
		{"小红书", []string{"xiaohongshu.com", "xhscdn.com", "xhscdn.net"}},
		{"微信", []string{"weixin.qq.com", "wechat.com", "weixinbridge.com", "qpic.cn"}},
		{"腾讯", []string{"qq.com", "gtimg.com", "qcloud.com", "myqcloud.com"}},
		{"百度", []string{"baidu.com", "bdstatic.com", "bcebos.com", "baidubce.com"}},
		{"阿里巴巴", []string{"alibaba.com", "alibabacloud.com", "alicdn.com", "aliyun.com", "taobao.com", "tmall.com"}},
		{"哔哩哔哩", []string{"bilibili.com", "bilivideo.com", "hdslb.com"}},
		{"Garmin", []string{"garmin.com", "garmin.cn"}},
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
	if domain, err := publicsuffix.EffectiveTLDPlusOne(host); err == nil {
		return domain
	}
	return host
}
