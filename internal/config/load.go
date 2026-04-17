package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrNotConfigured is returned when the config file does not yet exist.
var ErrNotConfigured = errors.New("gateway.yaml not found; run `gateway install` first")

// Paths resolves the runtime directory and the config file path.
type Paths struct {
	Root       string // ~/.config/lan-proxy-gateway on unix
	ConfigFile string // Root/gateway.yaml
	MihomoDir  string // Root/mihomo (working dir for the engine)
	CacheDir   string // ~/.cache/lan-proxy-gateway on linux, ~/Library/Caches on mac
}

// CacheDir returns the user-level cache directory for geodata etc.
// Survives `rm -rf ~/.config/lan-proxy-gateway` so we don't redownload on reinstall.
func resolveCacheDir(home string) string {
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "lan-proxy-gateway", "Cache")
		}
		return filepath.Join(home, "AppData", "Local", "lan-proxy-gateway", "Cache")
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Caches", "lan-proxy-gateway")
	}
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "lan-proxy-gateway")
	}
	return filepath.Join(home, ".cache", "lan-proxy-gateway")
}

// ResolvePaths returns the default paths for this platform and user.
//
// When invoked via sudo we still want to write into the calling user's home
// (not /root), so the user can read/delete their config normally afterwards.
func ResolvePaths() (Paths, error) {
	home := SudoUserHome()
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return Paths{}, fmt.Errorf("locate home directory: %w", err)
		}
		home = h
	}
	var root string
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			root = filepath.Join(appData, "lan-proxy-gateway")
		} else {
			root = filepath.Join(home, "AppData", "Roaming", "lan-proxy-gateway")
		}
	} else {
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			root = filepath.Join(xdg, "lan-proxy-gateway")
		} else {
			root = filepath.Join(home, ".config", "lan-proxy-gateway")
		}
	}
	return Paths{
		Root:       root,
		ConfigFile: filepath.Join(root, "gateway.yaml"),
		MihomoDir:  filepath.Join(root, "mihomo"),
		CacheDir:   resolveCacheDir(home),
	}, nil
}

// Load reads gateway.yaml from disk, applying v1→v2 migration if needed.
// If migration happened, it rewrites the file in v2 shape on disk so subsequent
// runs don't re-migrate (and don't re-spam the warning).
// Returns ErrNotConfigured if the file does not exist.
func Load() (*Config, Paths, error) {
	paths, err := ResolvePaths()
	if err != nil {
		return nil, paths, err
	}
	cfg, migrated, err := loadFrom(paths.ConfigFile)
	if err != nil {
		return nil, paths, err
	}
	if len(migrated) > 0 {
		fmt.Fprintln(os.Stderr, "配置已从 v1 升级到 v2：")
		for _, n := range migrated {
			fmt.Fprintf(os.Stderr, "  • %s\n", n)
		}
		// 写回 v2 形态，下次启动不再迁移。
		if err := Save(cfg, paths.ConfigFile); err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠ 写回 v2 形态失败: %v（下次还会再迁移一次）\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "  ✓ 已把 %s 重写为 v2 形态\n", paths.ConfigFile)
		}
	}
	return cfg, paths, nil
}

// LoadFrom is the silent variant — suitable for Configured() probes that
// shouldn't cause migration warnings to appear.
func LoadFrom(path string) (*Config, error) {
	cfg, _, err := loadFrom(path)
	return cfg, err
}

func loadFrom(path string) (*Config, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, ErrNotConfigured
		}
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}
	return parse(data)
}

// Parse (exported) converts raw YAML into Config, silently migrating v1.
// Preferred for in-memory tests. Use Load() for the filesystem entry point.
func Parse(data []byte) (*Config, error) {
	cfg, _, err := parse(data)
	return cfg, err
}

func parse(data []byte) (*Config, []string, error) {
	var probe struct {
		Version int `yaml:"version"`
	}
	_ = yaml.Unmarshal(data, &probe)

	if probe.Version >= Version {
		cfg := Default()
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, nil, fmt.Errorf("parse gateway.yaml: %w", err)
		}
		Normalize(cfg)
		if err := Validate(cfg); err != nil {
			return nil, nil, err
		}
		return cfg, nil, nil
	}

	cfg, notes, err := MigrateV1(data)
	if err != nil {
		return nil, nil, err
	}
	Normalize(cfg)
	if err := Validate(cfg); err != nil {
		return nil, nil, err
	}
	return cfg, notes, nil
}

// Save writes the config back to disk with mode 0600.
// When run under sudo, the file and its parent directory are chowned back to
// the calling user so non-root operations (e.g. `./gateway status`) can still
// read it.
func Save(cfg *Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	Normalize(cfg)
	if err := Validate(cfg); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	ReclaimToSudoUser(filepath.Dir(path))
	return nil
}

// Normalize fills in missing defaults so downstream code can rely on invariants.
func Normalize(cfg *Config) {
	if cfg.Version == 0 {
		cfg.Version = Version
	}
	if cfg.Traffic.Mode == "" {
		cfg.Traffic.Mode = ModeRule
	}
	if cfg.Source.Type == "" {
		cfg.Source.Type = SourceTypeNone
	}
	if cfg.Source.External.Server == "" {
		cfg.Source.External.Server = "127.0.0.1"
	}
	if cfg.Source.External.Port == 0 {
		cfg.Source.External.Port = 7890
	}
	if cfg.Source.External.Kind == "" {
		cfg.Source.External.Kind = "http"
	}
	if cfg.Source.External.Name == "" {
		cfg.Source.External.Name = "本机已有代理"
	}
	if cfg.Source.Subscription.Name == "" {
		cfg.Source.Subscription.Name = "subscription"
	}
	if cfg.Runtime.Ports.Mixed == 0 {
		cfg.Runtime.Ports.Mixed = 7890
	}
	if cfg.Runtime.Ports.Redir == 0 {
		cfg.Runtime.Ports.Redir = 7892
	}
	if cfg.Runtime.Ports.API == 0 {
		cfg.Runtime.Ports.API = 9090
	}
	if cfg.Gateway.DNS.Port == 0 {
		cfg.Gateway.DNS.Port = 53
	}
	if cfg.Runtime.LogLevel == "" {
		cfg.Runtime.LogLevel = "warning"
	}
}

// Validate checks the config is internally consistent.
func Validate(cfg *Config) error {
	switch cfg.Traffic.Mode {
	case ModeRule, ModeGlobal, ModeDirect:
	default:
		return fmt.Errorf("traffic.mode 必须是 rule/global/direct，当前: %q", cfg.Traffic.Mode)
	}
	switch cfg.Source.Type {
	case SourceTypeExternal, SourceTypeSubscription, SourceTypeFile, SourceTypeRemote, SourceTypeNone:
	default:
		return fmt.Errorf("source.type 必须是 external/subscription/file/remote/none，当前: %q", cfg.Source.Type)
	}
	switch cfg.Source.Type {
	case SourceTypeExternal:
		if cfg.Source.External.Port <= 0 {
			return errors.New("source.external.port 必须 > 0")
		}
		k := strings.ToLower(cfg.Source.External.Kind)
		if k != "http" && k != "socks5" {
			return fmt.Errorf("source.external.kind 必须是 http/socks5，当前: %q", cfg.Source.External.Kind)
		}
	case SourceTypeSubscription:
		if cfg.Source.Subscription.URL == "" {
			return errors.New("source.subscription.url 不能为空")
		}
	case SourceTypeFile:
		if cfg.Source.File.Path == "" {
			return errors.New("source.file.path 不能为空")
		}
	case SourceTypeRemote:
		if cfg.Source.Remote.Server == "" || cfg.Source.Remote.Port <= 0 {
			return errors.New("source.remote 必须指定 server 和 port")
		}
	}
	return nil
}
