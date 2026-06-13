// Package app is the single facade that the console and cobra commands both use.
// Every user-visible action (start, stop, set mode, switch source, ...) lives
// here — there is no parallel implementation in the CLI vs the TUI.
package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/engine"
	"github.com/tght/lan-proxy-gateway/internal/gateway"
	"github.com/tght/lan-proxy-gateway/internal/platform"
)

// AIBackendStatus 是给 UI 看的后端摘要，永不含明文 key。
type AIBackendStatus struct {
	ID           string
	Format       string
	Model        string
	Active       bool
	Builtin      bool
	HasKey       bool
	APIKeyMasked string // "***" 或空；绝不是明文
}

// aiStatus 把 AI 后端脱敏成 UI 摘要。
func (a *App) aiStatus() []AIBackendStatus {
	out := make([]AIBackendStatus, 0, len(a.Cfg.AI.Backends))
	for _, b := range a.Cfg.AI.Backends {
		masked := ""
		if b.APIKey != "" {
			masked = "***"
		}
		out = append(out, AIBackendStatus{
			ID:           b.ID,
			Format:       b.Format,
			Model:        b.Model,
			Active:       b.ID == a.Cfg.AI.Active,
			Builtin:      b.Builtin,
			HasKey:       b.APIKey != "",
			APIKeyMasked: masked,
		})
	}
	return out
}

// AddRule 把一条规则按裁决（direct/proxy/reject）加到 Traffic.Extras，存盘并热重载。
func (a *App) AddRule(ctx context.Context, verdict, rule string) error {
	switch verdict {
	case "direct":
		a.Cfg.Traffic.Extras.Direct = append(a.Cfg.Traffic.Extras.Direct, rule)
	case "proxy":
		a.Cfg.Traffic.Extras.Proxy = append(a.Cfg.Traffic.Extras.Proxy, rule)
	case "reject":
		a.Cfg.Traffic.Extras.Reject = append(a.Cfg.Traffic.Extras.Reject, rule)
	default:
		return fmt.Errorf("不支持的裁决: %s（应为 direct/proxy/reject）", verdict)
	}
	if err := a.Save(); err != nil {
		return err
	}
	if a.Engine != nil && a.Engine.Running() {
		return a.Engine.Reload(ctx, config.EffectiveRuntimeConfig(a.Cfg))
	}
	return nil
}

// HealthText 给 AI 执行器一个一行健康摘要。
func (a *App) HealthText() string {
	h := a.Health()
	if h.Healthy {
		return "代理源健康"
	}
	if h.LastError != "" {
		return "代理源异常: " + h.LastError
	}
	return "代理源状态未知"
}

// App wires together config, engine, gateway and platform.
type App struct {
	Cfg     *config.Config
	Paths   config.Paths
	Engine  *engine.Engine
	Gateway *gateway.Gateway
	Plat    platform.Platform

	// health 是代理源 supervisor 维护的健康看板；由 StartSupervisor 懒启动。
	health         *healthState
	supervisorOnce sync.Once
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
	bin, _ := platform.Current().ResolveMihomoPath("")
	gw := gateway.New()
	gw.SetStatePath(filepath.Join(paths.Root, "runtime.state"))
	a := &App{
		Cfg:     cfg,
		Paths:   paths,
		Engine:  engine.New(bin, paths.MihomoDir, paths.CacheDir),
		Gateway: gw,
		Plat:    platform.Current(),
	}
	// If a previous gateway session left mihomo running in the background,
	// wire the API client to it so Running()/Reload()/Stop() all work.
	a.Engine.Attach(a.Cfg)
	return a, nil
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

// Start brings up the LAN gateway and the mihomo engine.
func (a *App) Start(ctx context.Context) error {
	effective := config.EffectiveRuntimeConfig(a.Cfg)
	if effective.Gateway.Enabled {
		mode := effective.Gateway.Mode
		if mode == "" {
			mode = config.GatewayModeTUN
		}
		if err := a.Gateway.Enable(mode, effective.Runtime.Ports.Redir); err != nil {
			return fmt.Errorf("启动局域网网关失败: %w", err)
		}
	}
	if a.Engine == nil {
		return errors.New("mihomo 未找到，请先运行 `gateway install`")
	}
	if a.Engine.Running() {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	startCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return a.Engine.Start(startCtx, effective)
}

// Stop tears everything down, best-effort.
func (a *App) Stop() error {
	var firstErr error
	if err := a.restoreLocalDNSIfLoopback(); err != nil && firstErr == nil {
		firstErr = err
	}
	if a.Engine != nil {
		if err := a.Engine.Stop(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if a.Gateway != nil {
		if err := a.Gateway.Disable(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (a *App) restoreLocalDNSIfLoopback() error {
	if a.Plat == nil {
		return nil
	}
	loopback, err := a.Plat.LocalDNSIsLoopback()
	if err != nil {
		return fmt.Errorf("检查本机 DNS: %w", err)
	}
	if !loopback {
		return nil
	}
	if err := a.Plat.RestoreLocalDNS(); err != nil {
		if errors.Is(err, platform.ErrNotSupported) {
			return nil
		}
		return fmt.Errorf("恢复本机 DNS: %w", err)
	}
	return nil
}

// SetMode updates traffic.mode, saves, and hot-reloads mihomo if it's running.
func (a *App) SetMode(ctx context.Context, mode string) error {
	if mode != config.ModeRule && mode != config.ModeGlobal && mode != config.ModeDirect {
		return fmt.Errorf("不支持的模式: %s", mode)
	}
	a.Cfg.Traffic.Mode = mode
	if err := a.Save(); err != nil {
		return err
	}
	if a.Engine.Running() {
		return a.Engine.Reload(ctx, config.EffectiveRuntimeConfig(a.Cfg))
	}
	return nil
}

// ToggleAdblock flips adblock, saves, hot-reloads.
func (a *App) ToggleAdblock(ctx context.Context) error {
	a.Cfg.Traffic.Adblock = !a.Cfg.Traffic.Adblock
	if err := a.Save(); err != nil {
		return err
	}
	if a.Engine.Running() {
		return a.Engine.Reload(ctx, config.EffectiveRuntimeConfig(a.Cfg))
	}
	return nil
}

// ToggleTUN flips TUN mode, saves, hot-reloads.
func (a *App) ToggleTUN(ctx context.Context) error {
	a.Cfg.Gateway.TUN.Enabled = !a.Cfg.Gateway.TUN.Enabled
	if err := a.Save(); err != nil {
		return err
	}
	if a.Engine.Running() {
		return a.Engine.Reload(ctx, config.EffectiveRuntimeConfig(a.Cfg))
	}
	return nil
}

// SetGatewayMode switches between "tun" and "forward" gateway modes.
// Requires a full restart because the gateway layer (pf rules / TUN) must
// be torn down and re-created.
func (a *App) SetGatewayMode(ctx context.Context, mode string) error {
	if mode != config.GatewayModeTUN && mode != config.GatewayModeForward {
		return fmt.Errorf("不支持的网关模式: %s", mode)
	}
	a.Cfg.Gateway.Mode = mode
	if err := a.Save(); err != nil {
		return err
	}
	if a.Engine != nil && a.Engine.Running() {
		if err := a.Stop(); err != nil {
			return fmt.Errorf("停止旧网关失败: %w", err)
		}
		return a.Start(ctx)
	}
	return nil
}

// SetSource replaces the source config wholesale, saves and reloads.
func (a *App) SetSource(ctx context.Context, src config.SourceConfig) error {
	a.Cfg.Source = src
	if err := a.Save(); err != nil {
		return err
	}
	if a.Engine.Running() {
		return a.Engine.Reload(ctx, config.EffectiveRuntimeConfig(a.Cfg))
	}
	return nil
}

// Status builds a read-only snapshot for UI rendering.
type Status struct {
	Configured  bool
	Running     bool
	Mode        string
	Adblock     bool
	TUN         bool
	GatewayMode string
	Source      string
	Gateway     gateway.Status
	Ports       config.RuntimePorts
	MihomoBin   string
	ConfigFile  string
}

// Status returns the current runtime status (no blocking network calls).
func (a *App) Status() Status {
	effective := config.EffectiveRuntimeConfig(a.Cfg)
	gs, _ := a.Gateway.Status()
	bin := ""
	if p, err := a.Plat.ResolveMihomoPath(""); err == nil {
		bin = p
	}
	gwMode := effective.Gateway.Mode
	if gwMode == "" {
		gwMode = config.GatewayModeTUN
	}
	return Status{
		Configured:  a.Configured(),
		Running:     a.Engine != nil && a.Engine.Running(),
		Mode:        effective.Traffic.Mode,
		Adblock:     effective.Traffic.Adblock,
		TUN:         effective.Gateway.TUN.Enabled,
		GatewayMode: gwMode,
		Source:      effective.Source.Type,
		Gateway:     gs,
		Ports:       effective.Runtime.Ports,
		MihomoBin:   bin,
		ConfigFile:  a.Paths.ConfigFile,
	}
}
