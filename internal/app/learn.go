package app

import (
	"encoding/json"
	"github.com/tght/lan-proxy-gateway/internal/relay"
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
	fallbackLearnThreshold = 1
	fallbackLearnMaxHosts  = 512
)

// fallbackLearner stores direct-failed/proxy-response evidence and learning exclusions.
// Version 2 evidence must never be mixed with legacy proxy-failed/direct evidence.
type LearningSettings struct {
	Enabled              bool `json:"enabled"`
	Confirmations        int  `json:"confirmations"`
	AutoSave             bool `json:"auto_save"`
	DirectWaitSeconds    int  `json:"direct_wait_seconds"`
	ProxyWaitSeconds     int  `json:"proxy_wait_seconds"`
	MaxDirectWaitSeconds int  `json:"max_direct_wait_seconds"`
	CooldownSeconds      int  `json:"cooldown_seconds"`
	MemoryMinutes        int  `json:"memory_minutes"`
}

func defaultLearningSettings() LearningSettings {
	return LearningSettings{Enabled: true, Confirmations: 1, AutoSave: true, DirectWaitSeconds: 5, ProxyWaitSeconds: 5, MaxDirectWaitSeconds: 30, CooldownSeconds: 30, MemoryMinutes: 10}
}

// Old state files and callers omit the new fields; retain their historical defaults.
func (s LearningSettings) withResponseDefaults() LearningSettings {
	d := defaultLearningSettings()
	if s.DirectWaitSeconds == 0 {
		s.DirectWaitSeconds = d.DirectWaitSeconds
	}
	if s.ProxyWaitSeconds == 0 {
		s.ProxyWaitSeconds = d.ProxyWaitSeconds
	}
	if s.MaxDirectWaitSeconds == 0 {
		s.MaxDirectWaitSeconds = d.MaxDirectWaitSeconds
	}
	if s.CooldownSeconds == 0 {
		s.CooldownSeconds = d.CooldownSeconds
	}
	if s.MemoryMinutes == 0 {
		s.MemoryMinutes = d.MemoryMinutes
	}
	return s
}

func (l *fallbackLearner) ResponsePolicy() relay.ResponsePolicy {
	s := l.Settings().withResponseDefaults()
	return relay.ResponsePolicy{DirectWait: time.Duration(s.DirectWaitSeconds) * time.Second, ProxyWait: time.Duration(s.ProxyWaitSeconds) * time.Second, MaxDirectWait: time.Duration(s.MaxDirectWaitSeconds) * time.Second, Cooldown: time.Duration(s.CooldownSeconds) * time.Second, MemoryTTL: time.Duration(s.MemoryMinutes) * time.Minute}
}

type fallbackLearner struct {
	settings LearningSettings
	mu       sync.Mutex
	path     string
	logger   *slog.Logger
	counts   map[string][]time.Time
	ignored  map[string]bool
	now      func() time.Time // test hook
}

func newFallbackLearner(path string, logger *slog.Logger) *fallbackLearner {
	l := &fallbackLearner{
		path:     path,
		settings: defaultLearningSettings(),
		logger:   logger,
		counts:   map[string][]time.Time{},
		ignored:  map[string]bool{},
		now:      time.Now,
	}
	l.load()
	return l
}

// FallbackCandidate is proxy-response evidence pending successful rule persistence.
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
	if !l.settings.Enabled || l.ignored[host] {
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
		Settings *LearningSettings      `json:"settings"`
		Version  int                    `json:"version"`
		Counts   map[string][]time.Time `json:"counts"`
		Ignored  map[string]bool        `json:"ignored"`
	}
	if err := json.Unmarshal(data, &state); err != nil || state.Counts == nil {
		// Migrate the original host-to-timestamps file without losing candidates.
		if err := json.Unmarshal(data, &state.Counts); err != nil {
			l.logger.Warn("自动学习状态文件损坏，已忽略", "path", l.path, "err", err)
			return
		}
	}
	if state.Settings != nil && state.Settings.Confirmations >= 1 && state.Settings.Confirmations <= 10 {
		l.settings = state.Settings.withResponseDefaults()
	}
	if state.Ignored != nil {
		l.ignored = state.Ignored
	}
	counts := state.Counts
	if state.Version != 2 {
		counts = nil
	} // Opposite-direction evidence is not transferable.
	cutoff := l.now().Add(-fallbackLearnWindow)
	l.mu.Lock()
	for host, times := range counts {
		if pruned := pruneLearnTimes(times, cutoff); len(pruned) > 0 {
			l.counts[host] = pruned
		}
	}
	l.mu.Unlock()
}

func (l *fallbackLearner) saveLocked() (result error) {
	defer func() {
		if result != nil && l.logger != nil {
			l.logger.Warn("自动学习状态写入失败", "err", result)
		}
	}()
	data, err := json.Marshal(struct {
		Settings LearningSettings       `json:"settings"`
		Version  int                    `json:"version"`
		Counts   map[string][]time.Time `json:"counts"`
		Ignored  map[string]bool        `json:"ignored"`
	}{l.settings, 2, l.counts, l.ignored})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(l.path), ".learning-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), l.path)
}

func (l *fallbackLearner) Settings() LearningSettings {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.settings
}
