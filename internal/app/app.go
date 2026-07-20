// Package app is the single facade that the console and cobra commands both use.
// Every user-visible action (start, stop, set egress, ...) lives here.
//
// v4 进程模型：relay/DNS 都是进程内服务，不再有外部引擎。
//   - `gateway start`  spawn 一个脱离终端的 `gateway run` 守护进程后返回
//   - `gateway run`    守护主体：防火墙 → DNS → relay → loopback API → 阻塞
//   - 配置变更         写盘 + POST /api/reload；守护进程同时轮询 mtime 兜底
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/dns"
	"github.com/tght/lan-proxy-gateway/internal/firewall"
	"github.com/tght/lan-proxy-gateway/internal/gateway"
	"github.com/tght/lan-proxy-gateway/internal/platform"
	"github.com/tght/lan-proxy-gateway/internal/relay"
)

// App wires together config, gateway and platform; daemon-side services
// (relay/dns/api) exist only inside the `run` process.
type App struct {
	Cfg     *config.Config
	Paths   config.Paths
	Gateway *gateway.Gateway
	Plat    platform.Platform

	// cfgMu guards Cfg against daemon goroutines (config watcher, API
	// handlers, supervisor) swapping/reading it concurrently. CLI/console
	// processes are effectively single-threaded but use the accessors too.
	cfgMu          sync.RWMutex
	health         *healthState
	supervisorOnce sync.Once
}

// getCfg returns the current config under read lock (daemon-safe).
func (a *App) getCfg() *config.Config {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.Cfg
}

// setCfg swaps the live config (daemon hot-apply path).
func (a *App) setCfg(cfg *config.Config) {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	a.Cfg = cfg
}

// New builds an App. It loads the config from disk; if missing, it returns one
// populated with defaults (so TUI / CLI can walk the user through install).
func New() (*App, error) {
	cfg, paths, err := config.Load()
	if errors.Is(err, config.ErrNotConfigured) {
		cfg = config.Default()
	} else if err != nil {
		return nil, err
	}
	gw := gateway.New()
	gw.SetStatePath(paths.StateFile)
	return &App{
		Cfg:     cfg,
		Paths:   paths,
		Gateway: gw,
		Plat:    platform.Current(),
		health:  &healthState{healthy: true},
	}, nil
}

// Configured reports whether gateway.yaml exists on disk.
func (a *App) Configured() bool {
	_, err := config.LoadFrom(a.Paths.ConfigFile)
	return err == nil
}

// Save persists the current config.
func (a *App) Save() error {
	return config.Save(a.Cfg, a.Paths.ConfigFile)
}

// ---------- egress facade ----------

// SetEgress validates and saves a new egress config. When switching to proxy
// mode the upstream proxy is probed first (unless probe=false); a failed probe
// aborts the change. If the daemon is running it picks the change up live.
func (a *App) SetEgress(ctx context.Context, e config.EgressConfig, probe bool) error {
	next := *a.Cfg
	next.Egress = e
	config.Normalize(&next)
	if err := config.Validate(&next); err != nil {
		return err
	}
	if probe && e.Mode == config.EgressProxy {
		d, err := buildDialer(e)
		if err != nil {
			return err
		}
		probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := relay.Probe(probeCtx, d, probeTarget); err != nil {
			return fmt.Errorf("上游代理连通性测试失败: %w", err)
		}
	}
	a.Cfg.Egress = e
	if err := a.Save(); err != nil {
		return err
	}
	a.pokeReload()
	return nil
}

// TestEgress probes the currently configured egress end-to-end.
func (a *App) TestEgress(ctx context.Context) error {
	return a.TestEgressConfig(ctx, a.Cfg.Egress)
}

// TestEgressConfig probes a candidate egress config without saving it.
func (a *App) TestEgressConfig(ctx context.Context, e config.EgressConfig) error {
	d, err := buildDialer(e)
	if err != nil {
		return err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return relay.Probe(probeCtx, d, probeTarget)
}

// pokeReload asks a running daemon to re-read the config; best-effort.
func (a *App) pokeReload() {
	if !a.Running() {
		return
	}
	_ = apiClient(a.Cfg.Runtime.APIPort).Reload(context.Background())
}

// ---------- shared helpers ----------

// probeTarget is the connectivity probe endpoint (HTTP) used for egress checks.
const probeTarget = "www.apple.com:80"

// buildDialer constructs the egress dialer for a config.
func buildDialer(e config.EgressConfig) (relay.Dialer, error) {
	const timeout = 15 * time.Second
	if e.Mode != config.EgressProxy {
		return relay.NewDirectDialer(timeout), nil
	}
	p := e.Proxy
	addr := net.JoinHostPort(p.Host, strconv.Itoa(p.Port))
	switch p.Type {
	case config.ProxyTypeSOCKS5:
		return relay.NewSOCKS5Dialer(addr, p.Username, p.Password, timeout), nil
	case config.ProxyTypeHTTP:
		return relay.NewHTTPConnectDialer(addr, p.Username, p.Password, timeout), nil
	default:
		return nil, fmt.Errorf("不支持的代理类型: %q", p.Type)
	}
}

// firewallConfig derives the desired firewall rule set from the config.
func firewallConfig(cfg *config.Config) firewall.Config {
	proxy := cfg.Egress.Mode == config.EgressProxy
	return firewall.Config{
		RedirPort: cfg.Runtime.RedirPort,
		DNSPort:   cfg.DNS.Port,
		// Relay TCP in both modes. On a single-interface macOS gateway, pf NAT
		// does not translate packets that enter and leave on the same interface,
		// so kernel-forwarded direct connections never receive replies.
		TCPRedirect: true,
		DNSHijack:   cfg.DNS.Enabled && cfg.DNS.Hijack,
		QUICBlock:   proxy && cfg.QUICBlock,
	}
}

// dnsOptions derives the DNS server options from the config.
func dnsOptions(cfg *config.Config, logger *slog.Logger) dns.Options {
	fakeRange, _ := netip.ParsePrefix(cfg.Runtime.FakeIPRange)
	return dns.Options{
		Addr:          net.JoinHostPort("", strconv.Itoa(cfg.DNS.Port)),
		Upstreams:     cfg.DNS.Upstreams,
		FakeIPRange:   fakeRange,
		FakeIPEnabled: cfg.Egress.Mode == config.EgressProxy && cfg.DNS.FakeIP,
		FakeIPFilter:  cfg.DNS.FakeIPFilter,
		Logger:        logger,
	}
}

// ---------- status ----------

// Status is a read-only snapshot for UI rendering and `gateway status --json`.
type Status struct {
	Configured bool           `json:"configured"`
	Running    bool           `json:"running"`
	Egress     string         `json:"egress"`
	Proxy      string         `json:"proxy,omitempty"` // "socks5 127.0.0.1:7897" when egress=proxy
	DNS        DNSStatus      `json:"dns"`
	QUICBlock  bool           `json:"quic_block"`
	Gateway    gateway.Status `json:"gateway"`
	Ports      PortsStatus    `json:"ports"`
	ConfigFile string         `json:"config_file"`
	LogFile    string         `json:"log_file"`
}

type DNSStatus struct {
	Enabled bool `json:"enabled"`
	Port    int  `json:"port"`
	Hijack  bool `json:"hijack"`
	FakeIP  bool `json:"fake_ip"`
}

type PortsStatus struct {
	Redir int `json:"redir"`
	API   int `json:"api"`
	DNS   int `json:"dns"`
}

// Status returns the current runtime status (no blocking network calls).
func (a *App) Status() Status {
	gs, _ := a.Gateway.Status()
	st := Status{
		Configured: a.Configured(),
		Running:    a.Running(),
		Egress:     a.Cfg.Egress.Mode,
		DNS: DNSStatus{
			Enabled: a.Cfg.DNS.Enabled,
			Port:    a.Cfg.DNS.Port,
			Hijack:  a.Cfg.DNS.Hijack,
			FakeIP:  a.Cfg.DNS.FakeIP && a.Cfg.Egress.Mode == config.EgressProxy,
		},
		QUICBlock: a.Cfg.QUICBlock && a.Cfg.Egress.Mode == config.EgressProxy,
		Gateway:   gs,
		Ports: PortsStatus{
			Redir: a.Cfg.Runtime.RedirPort,
			API:   a.Cfg.Runtime.APIPort,
			DNS:   a.Cfg.DNS.Port,
		},
		ConfigFile: a.Paths.ConfigFile,
		LogFile:    a.Paths.LogFile,
	}
	if a.Cfg.Egress.Mode == config.EgressProxy {
		p := a.Cfg.Egress.Proxy
		st.Proxy = fmt.Sprintf("%s %s:%d", p.Type, p.Host, p.Port)
	}
	return st
}
