// Package app is the single facade that the console and cobra commands both use.
// Every user-visible action (start, stop, set mode, switch source, ...) lives
// here — there is no parallel implementation in the CLI vs the TUI.
package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/engine"
	"github.com/tght/lan-proxy-gateway/internal/gateway"
	"github.com/tght/lan-proxy-gateway/internal/platform"
)

// App wires together config, engine, gateway and platform.
type App struct {
	Cfg     *config.Config
	Paths   config.Paths
	Engine  *engine.Engine
	Gateway *gateway.Gateway
	Plat    platform.Platform
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
	return &App{
		Cfg:     cfg,
		Paths:   paths,
		Engine:  engine.New(bin, paths.MihomoDir, paths.CacheDir),
		Gateway: gateway.New(),
		Plat:    platform.Current(),
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

// Start brings up the LAN gateway and the mihomo engine.
func (a *App) Start(ctx context.Context) error {
	if a.Cfg.Gateway.Enabled {
		if err := a.Gateway.Enable(); err != nil {
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
	return a.Engine.Start(startCtx, a.Cfg)
}

// Stop tears everything down, best-effort.
func (a *App) Stop() error {
	_ = a.Engine.Stop()
	return a.Gateway.Disable()
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
		return a.Engine.Reload(ctx, a.Cfg)
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
		return a.Engine.Reload(ctx, a.Cfg)
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
		return a.Engine.Reload(ctx, a.Cfg)
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
		return a.Engine.Reload(ctx, a.Cfg)
	}
	return nil
}

// Status builds a read-only snapshot for UI rendering.
type Status struct {
	Configured bool
	Running    bool
	Mode       string
	Adblock    bool
	TUN        bool
	Source     string
	Gateway    gateway.Status
	Ports      config.RuntimePorts
	MihomoBin  string
	ConfigFile string
}

// Status returns the current runtime status (no blocking network calls).
func (a *App) Status() Status {
	gs, _ := a.Gateway.Status()
	bin := ""
	if p, err := a.Plat.ResolveMihomoPath(""); err == nil {
		bin = p
	}
	return Status{
		Configured: a.Configured(),
		Running:    a.Engine != nil && a.Engine.Running(),
		Mode:       a.Cfg.Traffic.Mode,
		Adblock:    a.Cfg.Traffic.Adblock,
		TUN:        a.Cfg.Gateway.TUN.Enabled,
		Source:     a.Cfg.Source.Type,
		Gateway:    gs,
		Ports:      a.Cfg.Runtime.Ports,
		MihomoBin:  bin,
		ConfigFile: a.Paths.ConfigFile,
	}
}
