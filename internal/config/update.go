package config

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"
)

var ErrConfigChanged = errors.New("配置在保存前再次更新，未覆盖最新配置；请刷新后重试")

// The stable sidecar inode serializes read/modify/write across CLI and daemon
// processes. Never unlink it: writers may already be waiting on the inode.
func lockConfig(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	ReclaimToSudoUser(path + ".lock")
	for {
		err = unix.Flock(int(f.Fd()), unix.LOCK_EX)
		if err != unix.EINTR {
			break
		}
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return func() { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN); _ = f.Close() }, nil
}

// Save rejects stale loaded snapshots, including writes from settings and
// background jobs. New configurations can still be initialized with Default.
func Save(cfg *Config, path string) error {
	unlock, err := lockConfig(path)
	if err != nil {
		return err
	}
	defer unlock()
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if cfg.sourcePath == abs {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return ErrConfigChanged
		}
		if err != nil {
			return err
		}
		if sha256.Sum256(data) != cfg.sourceHash {
			return ErrConfigChanged
		}
	}
	return saveLocked(cfg, path)
}

// Update holds the cross-process lock from the latest read through the atomic
// replacement. The callback must not call Save or Update. A false result skips
// writing. Missing files fail instead of resurrecting a deleted configuration.
func Update(path string, edit func(*Config) (bool, error)) (*Config, error) {
	unlock, err := lockConfig(path)
	if err != nil {
		return nil, err
	}
	defer unlock()
	next, err := LoadFrom(path)
	if err != nil {
		return nil, err
	}
	changed, err := edit(next)
	if err != nil {
		return nil, err
	}
	if changed {
		if err := saveLocked(next, path); err != nil {
			return nil, err
		}
	}
	return next, nil
}

func saveLocked(cfg *Config, path string) error {
	Normalize(cfg)
	if err := Validate(cfg); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".gateway-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	ReclaimToSudoUser(f.Name())
	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}
	cfg.sourcePath, _ = filepath.Abs(path)
	cfg.sourceHash = sha256.Sum256(data)
	ReclaimToSudoUser(filepath.Dir(path))
	return nil
}
