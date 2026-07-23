package app

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
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
// rolling 24h window. Reaching the threshold promotes the host to a learned
// direct routing rule via the promote callback. Counts persist to disk so a
// daemon restart doesn't reset progress; the threshold is deliberately
// conservative to avoid固化 a one-off proxy hiccup into a permanent route.
type fallbackLearner struct {
	mu      sync.Mutex
	path    string
	logger  *slog.Logger
	counts  map[string][]time.Time
	promote func(host string)
	now     func() time.Time // test hook
}

func newFallbackLearner(path string, logger *slog.Logger) *fallbackLearner {
	l := &fallbackLearner{
		path:   path,
		logger: logger,
		counts: map[string][]time.Time{},
		now:    time.Now,
	}
	l.load()
	return l
}

// Record notes one successful fallback for host and promotes at threshold.
func (l *fallbackLearner) Record(host string) {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" {
		return
	}
	now := l.now()
	cutoff := now.Add(-fallbackLearnWindow)
	var promoteHost string
	l.mu.Lock()
	times := pruneLearnTimes(l.counts[host], cutoff)
	times = append(times, now)
	if len(times) >= fallbackLearnThreshold {
		delete(l.counts, host)
		promoteHost = host
	} else {
		l.counts[host] = times
		l.evictLocked(cutoff)
	}
	l.saveLocked()
	l.mu.Unlock()
	if promoteHost != "" && l.promote != nil {
		l.promote(promoteHost)
	}
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
	var counts map[string][]time.Time
	if err := json.Unmarshal(data, &counts); err != nil {
		l.logger.Warn("自动学习状态文件损坏，已忽略", "path", l.path, "err", err)
		return
	}
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
	data, err := json.Marshal(l.counts)
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
