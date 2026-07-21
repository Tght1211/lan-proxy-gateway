package config

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ErrNotConfigured is returned when no usable config exists (missing file,
// or a legacy config that was backed up and needs re-onboarding).
var ErrNotConfigured = errors.New("gateway.yaml not found; run `gateway install` first")

// Paths resolves the runtime directory and well-known file paths.
type Paths struct {
	Root            string // ~/.config/lan-proxy-gateway on unix
	ConfigFile      string // Root/gateway.yaml
	LogFile         string // Root/gateway.log (daemon log)
	PIDFile         string // Root/gateway.pid (daemon pid)
	StateFile       string // Root/runtime.state (firewall/ipforward rollback)
	FakeIPCacheFile string // Root/fakeip-cache.json (restart-safe DNS mappings)
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
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		root = filepath.Join(xdg, "lan-proxy-gateway")
	} else {
		root = filepath.Join(home, ".config", "lan-proxy-gateway")
	}
	return Paths{
		Root:            root,
		ConfigFile:      filepath.Join(root, "gateway.yaml"),
		LogFile:         filepath.Join(root, "gateway.log"),
		PIDFile:         filepath.Join(root, "gateway.pid"),
		StateFile:       filepath.Join(root, "runtime.state"),
		FakeIPCacheFile: filepath.Join(root, "fakeip-cache.json"),
	}, nil
}

// Load reads gateway.yaml from disk. A legacy (pre-v4) config is backed up
// and reported as ErrNotConfigured so onboarding re-runs.
func Load() (*Config, Paths, error) {
	paths, err := ResolvePaths()
	if err != nil {
		return nil, paths, err
	}
	cfg, err := loadFrom(paths.ConfigFile)
	if err != nil {
		return nil, paths, err
	}
	return cfg, paths, nil
}

// LoadFrom is the silent variant — suitable for Configured() probes.
func LoadFrom(path string) (*Config, error) {
	return loadFrom(path)
}

func loadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotConfigured
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return parseFile(path, data)
}

// Parse converts raw v4 YAML into Config. Preferred for in-memory tests.
func Parse(data []byte) (*Config, error) {
	var probe struct {
		Version int `yaml:"version"`
	}
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("parse gateway.yaml: %w", err)
	}
	if probe.Version < Version {
		return nil, fmt.Errorf("配置文件版本过旧(version=%d)，v4 需要重新初始化", probe.Version)
	}
	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse gateway.yaml: %w", err)
	}
	Normalize(cfg)
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// parseFile detects legacy configs, backs them up, and parses v4.
func parseFile(path string, data []byte) (*Config, error) {
	var probe struct {
		Version int `yaml:"version"`
	}
	_ = yaml.Unmarshal(data, &probe)

	legacy := probe.Version < Version
	if !legacy {
		// versionless or foreign keys from the mihomo era also count as legacy
		var raw map[string]any
		if err := yaml.Unmarshal(data, &raw); err == nil {
			_, hasSource := raw["source"]
			_, hasTraffic := raw["traffic"]
			legacy = hasSource || hasTraffic
		}
	}
	if legacy {
		bak, err := backupLegacy(path)
		if err != nil {
			return nil, fmt.Errorf("检测到旧版配置但备份失败: %w", err)
		}
		fmt.Fprintf(os.Stderr, "检测到旧版配置，已备份到 %s\nv4 是全新架构，请重新完成初始化向导。\n", bak)
		return nil, ErrNotConfigured
	}
	return Parse(data)
}

// backupLegacy renames path to path.pre-v4.bak (timestamped suffix on collision).
func backupLegacy(path string) (string, error) {
	bak := path + ".pre-v4.bak"
	if _, err := os.Stat(bak); err == nil {
		bak = path + ".pre-v4.bak." + time.Now().Format("20060102150405")
	}
	if err := os.Rename(path, bak); err != nil {
		return "", err
	}
	return bak, nil
}

// Save writes the config back to disk with mode 0600.
// When run under sudo, the file and its parent directory are chowned back to
// the calling user so non-root operations can still read it.
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
	if cfg.Egress.Mode == "" {
		cfg.Egress.Mode = EgressDirect
	}
	if cfg.Egress.Proxy.Type == "" {
		cfg.Egress.Proxy.Type = ProxyTypeSOCKS5
	}
	if cfg.Egress.Proxy.Host == "" {
		cfg.Egress.Proxy.Host = "127.0.0.1"
	}
	if cfg.Egress.Proxy.Port == 0 {
		cfg.Egress.Proxy.Port = 7897
	}
	if cfg.DNS.Port == 0 {
		cfg.DNS.Port = 53
	}
	if len(cfg.DNS.Upstreams) == 0 {
		cfg.DNS.Upstreams = []string{"223.5.5.5", "119.29.29.29"}
	}
	if cfg.Runtime.RedirPort == 0 {
		cfg.Runtime.RedirPort = 17892
	}
	if cfg.Runtime.APIPort == 0 {
		cfg.Runtime.APIPort = 19090
	}
	if cfg.Runtime.LogLevel == "" {
		cfg.Runtime.LogLevel = "info"
	}
	if cfg.Runtime.FakeIPRange == "" {
		cfg.Runtime.FakeIPRange = "198.18.0.0/16"
	}
}

// Validate checks the config is internally consistent.
func Validate(cfg *Config) error {
	switch cfg.Egress.Mode {
	case EgressDirect, EgressProxy:
	default:
		return fmt.Errorf("egress.mode 必须是 direct/proxy，当前: %q", cfg.Egress.Mode)
	}
	if cfg.Egress.Mode == EgressProxy {
		p := cfg.Egress.Proxy
		switch p.Type {
		case ProxyTypeHTTP, ProxyTypeSOCKS5:
		default:
			return fmt.Errorf("egress.proxy.type 必须是 http/socks5，当前: %q", p.Type)
		}
		if p.Host == "" {
			return errors.New("egress.proxy.host 不能为空")
		}
		if p.Port <= 0 || p.Port > 65535 {
			return fmt.Errorf("egress.proxy.port 不合法: %d", p.Port)
		}
	}
	for _, port := range []struct {
		name string
		v    int
	}{
		{"dns.port", cfg.DNS.Port},
		{"runtime.redir_port", cfg.Runtime.RedirPort},
		{"runtime.api_port", cfg.Runtime.APIPort},
	} {
		if port.v <= 0 || port.v > 65535 {
			return fmt.Errorf("%s 不合法: %d", port.name, port.v)
		}
	}
	for _, u := range cfg.DNS.Upstreams {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		host := u
		if ap, err := netip.ParseAddrPort(u); err == nil {
			host = ap.Addr().String()
		}
		if _, err := netip.ParseAddr(host); err != nil {
			// allow hostnames too, but not garbage
			if strings.ContainsAny(host, " \t/") {
				return fmt.Errorf("dns.upstreams 包含非法条目: %q", u)
			}
		}
	}
	if _, err := netip.ParsePrefix(cfg.Runtime.FakeIPRange); err != nil {
		return fmt.Errorf("runtime.fake_ip_range 不合法: %q", cfg.Runtime.FakeIPRange)
	}
	return nil
}
