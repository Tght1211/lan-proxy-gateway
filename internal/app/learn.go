package app

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	fallbackLearnWindow    = 24 * time.Hour
	fallbackLearnThreshold = 3
	fallbackLearnMaxHosts  = 512
)

// fallbackLearner counts successful proxy→direct fallback dials per host in a
// rolling 24h window. Suggestions persist across restarts and require an explicit
// user action before becoming routing rules.
type fallbackLearner struct {
	mu      sync.Mutex
	path    string
	logger  *slog.Logger
	counts  map[string][]time.Time
	ignored map[string]bool
	now     func() time.Time // test hook
}

func newFallbackLearner(path string, logger *slog.Logger) *fallbackLearner {
	l := &fallbackLearner{
		path:    path,
		logger:  logger,
		counts:  map[string][]time.Time{},
		ignored: map[string]bool{},
		now:     time.Now,
	}
	l.load()
	return l
}

// FallbackCandidate is one host observed via proxy→direct fallback that has
// not yet reached the auto-learn threshold.
type FallbackCandidate struct {
	Host   string    `json:"host"`
	Count  int       `json:"count"`
	LastAt time.Time `json:"last_at"`
}

// Snapshot lists in-window candidates, most recent first.
func (l *fallbackLearner) Snapshot() []FallbackCandidate {
	cutoff := l.now().Add(-fallbackLearnWindow)
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]FallbackCandidate, 0, len(l.counts))
	for host, times := range l.counts {
		pruned := pruneLearnTimes(times, cutoff)
		if len(pruned) == 0 {
			continue
		}
		l.counts[host] = pruned
		out = append(out, FallbackCandidate{Host: host, Count: len(pruned), LastAt: pruned[len(pruned)-1]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastAt.After(out[j].LastAt) })
	return out
}

// Record notes a fallback with observed response data; it never modifies routing.
func (l *fallbackLearner) Record(host string) {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" {
		return
	}
	now := l.now()
	cutoff := now.Add(-fallbackLearnWindow)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.ignored[host] {
		return
	}
	times := append(pruneLearnTimes(l.counts[host], cutoff), now)
	if len(times) > 256 {
		times = times[len(times)-256:]
	}
	l.counts[host] = times
	l.evictLocked(cutoff)
	l.saveLocked()
}

func pruneLearnTimes(times []time.Time, cutoff time.Time) []time.Time {
	out := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			out = append(out, t)
		}
	}
	return out
}

// evictLocked drops fully-expired hosts, then the oldest-activity hosts when
// the map grows past the cap.
func (l *fallbackLearner) evictLocked(cutoff time.Time) {
	for host, times := range l.counts {
		if pruned := pruneLearnTimes(times, cutoff); len(pruned) == 0 {
			delete(l.counts, host)
		} else {
			l.counts[host] = pruned
		}
	}
	for len(l.counts) > fallbackLearnMaxHosts {
		var oldest string
		var oldestAt time.Time
		for host, times := range l.counts {
			last := times[len(times)-1]
			if oldest == "" || last.Before(oldestAt) {
				oldest, oldestAt = host, last
			}
		}
		delete(l.counts, oldest)
	}
}

func (l *fallbackLearner) load() {
	data, err := os.ReadFile(l.path)
	if err != nil {
		return
	}
	var state struct {
		Counts  map[string][]time.Time `json:"counts"`
		Ignored map[string]bool        `json:"ignored"`
	}
	if err := json.Unmarshal(data, &state); err != nil || state.Counts == nil {
		// Migrate the original host-to-timestamps file without losing candidates.
		if err := json.Unmarshal(data, &state.Counts); err != nil {
			l.logger.Warn("自动学习状态文件损坏，已忽略", "path", l.path, "err", err)
			return
		}
	}
	if state.Ignored != nil {
		l.ignored = state.Ignored
	}
	counts := state.Counts
	cutoff := l.now().Add(-fallbackLearnWindow)
	l.mu.Lock()
	for host, times := range counts {
		if pruned := pruneLearnTimes(times, cutoff); len(pruned) > 0 {
			l.counts[host] = pruned
		}
	}
	l.mu.Unlock()
}

func (l *fallbackLearner) saveLocked() {
	data, err := json.Marshal(struct {
		Counts  map[string][]time.Time `json:"counts"`
		Ignored map[string]bool        `json:"ignored"`
	}{l.counts, l.ignored})
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return
	}
	if err := os.WriteFile(l.path, data, 0o600); err != nil && l.logger != nil {
		l.logger.Warn("自动学习状态写入失败", "path", l.path, "err", err)
	}
}
