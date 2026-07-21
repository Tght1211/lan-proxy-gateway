package dns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"time"
)

const fakeIPCacheVersion = 1

type fakeIPCacheFile struct {
	Version int                `json:"version"`
	Prefix  string             `json:"prefix"`
	Offset  uint32             `json:"offset"`
	Entries []fakeIPCacheEntry `json:"entries"`
}

type fakeIPCacheEntry struct {
	IP       netip.Addr `json:"ip"`
	Name     string     `json:"name"`
	LastSeen time.Time  `json:"last_seen"`
}

func (s *Server) loadFakeIPCache(now time.Time) (int, error) {
	if s.cachePath == "" {
		return 0, nil
	}
	data, err := os.ReadFile(s.cachePath)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var cached fakeIPCacheFile
	if err := json.Unmarshal(data, &cached); err != nil {
		return 0, fmt.Errorf("decode cache: %w", err)
	}
	if cached.Version != fakeIPCacheVersion {
		return 0, fmt.Errorf("unsupported cache version %d", cached.Version)
	}
	if cached.Prefix != s.prefix.String() {
		return 0, fmt.Errorf("cache prefix %s does not match %s", cached.Prefix, s.prefix)
	}
	snapshot := fakeIPPoolSnapshot{Offset: cached.Offset, Entries: make([]fakeIPSnapshotEntry, 0, len(cached.Entries))}
	for _, entry := range cached.Entries {
		snapshot.Entries = append(snapshot.Entries, fakeIPSnapshotEntry{
			IP: entry.IP, Name: entry.Name, LastSeen: entry.LastSeen,
		})
	}
	return s.pool.restore(snapshot, now), nil
}

func (s *Server) flushFakeIPCache(now time.Time) error {
	if s.cachePath == "" || !s.pool.dirty.Load() {
		return nil
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if !s.pool.dirty.Load() {
		return nil
	}
	snapshot := s.pool.snapshot(now)
	cached := fakeIPCacheFile{
		Version: fakeIPCacheVersion,
		Prefix:  s.prefix.String(),
		Offset:  snapshot.Offset,
		Entries: make([]fakeIPCacheEntry, 0, len(snapshot.Entries)),
	}
	for _, entry := range snapshot.Entries {
		cached.Entries = append(cached.Entries, fakeIPCacheEntry{
			IP: entry.IP, Name: entry.Name, LastSeen: entry.LastSeen,
		})
	}
	data, err := json.Marshal(cached)
	if err == nil {
		err = writeFileAtomic(s.cachePath, data, 0o600)
	}
	if err != nil {
		s.pool.dirty.Store(true)
	}
	return err
}

func (s *Server) persistFakeIPCache(ctx context.Context) {
	if s.cachePath == "" {
		return
	}
	ticker := time.NewTicker(cacheFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := s.flushFakeIPCache(now); err != nil {
				s.logger.Warn("fake-ip 缓存保存失败", "path", s.cachePath, "err", err)
			}
		}
	}
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) (retErr error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".fakeip-cache-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		_ = f.Close()
		if retErr != nil {
			_ = os.Remove(tmp)
		}
	}()
	if err := f.Chmod(mode); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
