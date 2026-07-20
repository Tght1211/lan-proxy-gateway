package relay

import (
	"sync"
	"sync/atomic"
	"time"
)

// ConnInfo is a point-in-time view of one relayed connection.
type ConnInfo struct {
	ID        uint64    `json:"id"`
	SrcIP     string    `json:"src_ip"`
	DstHost   string    `json:"dst_host"`
	DstPort   int       `json:"dst_port"`
	Up        int64     `json:"up"`
	Down      int64     `json:"down"`
	StartedAt time.Time `json:"started_at"`
	ViaProxy  bool      `json:"via_proxy"`
}

// Snapshot is the full tracker state for the dashboard/API.
type Snapshot struct {
	UpTotal   int64      `json:"up_total"`
	DownTotal int64      `json:"down_total"`
	Active    []ConnInfo `json:"active"`
}

// Tracker keeps an in-memory table of live relayed connections plus global
// byte counters. It is the dashboard's data source (replacing the old
// mihomo /connections dependency).
type Tracker struct {
	mu        sync.Mutex
	conns     map[uint64]*TrackedConn
	nextID    uint64
	upTotal   atomic.Int64
	downTotal atomic.Int64
}

func NewTracker() *Tracker {
	return &Tracker{conns: map[uint64]*TrackedConn{}}
}

// Open registers a new connection; Close on the returned handle evicts it.
func (t *Tracker) Open(srcIP, dstHost string, dstPort int, viaProxy bool) *TrackedConn {
	t.mu.Lock()
	t.nextID++
	c := &TrackedConn{
		t:         t,
		id:        t.nextID,
		srcIP:     srcIP,
		dstHost:   dstHost,
		dstPort:   dstPort,
		viaProxy:  viaProxy,
		startedAt: time.Now(),
	}
	t.conns[c.id] = c
	t.mu.Unlock()
	return c
}

func (t *Tracker) Snapshot() Snapshot {
	t.mu.Lock()
	out := Snapshot{Active: make([]ConnInfo, 0, len(t.conns))}
	for _, c := range t.conns {
		out.Active = append(out.Active, c.info())
	}
	t.mu.Unlock()
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
	viaProxy  bool
	startedAt time.Time
	up        atomic.Int64
	down      atomic.Int64
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
	c.t.mu.Lock()
	delete(c.t.conns, c.id)
	c.t.mu.Unlock()
}

// Up/Down report this connection's byte counters.
func (c *TrackedConn) Up() int64   { return c.up.Load() }
func (c *TrackedConn) Down() int64 { return c.down.Load() }

func (c *TrackedConn) info() ConnInfo {
	return ConnInfo{
		ID:        c.id,
		SrcIP:     c.srcIP,
		DstHost:   c.dstHost,
		DstPort:   c.dstPort,
		Up:        c.up.Load(),
		Down:      c.down.Load(),
		StartedAt: c.startedAt,
		ViaProxy:  c.viaProxy,
	}
}
