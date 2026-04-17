package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	configpkg "github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/mihomo"
)

// Engine wraps the mihomo process + its REST API. One instance per running gateway.
type Engine struct {
	bin      string
	workdir  string
	cacheDir string // optional, for geodata caching across reinstalls
	proc     *process
	api      *Client
}

// New returns an Engine configured to run `bin` with its working directory.
// cacheDir is optional (pass "" to disable geodata caching).
func New(bin, workdir, cacheDir string) *Engine {
	return &Engine{
		bin:      bin,
		workdir:  workdir,
		cacheDir: cacheDir,
		api:      NewClient(""), // baseURL filled in Start()
	}
}

// Workdir returns the working directory where the rendered config lives.
func (e *Engine) Workdir() string { return e.workdir }

// ConfigPath returns the path to the rendered mihomo config.yaml.
func (e *Engine) ConfigPath() string { return filepath.Join(e.workdir, "config.yaml") }

// API exposes the HTTP client (already configured to hit the running mihomo).
func (e *Engine) API() *Client { return e.api }

// Start renders the config, launches mihomo, and waits briefly for the API to come up.
func (e *Engine) Start(ctx context.Context, cfg *configpkg.Config) error {
	if e.bin == "" {
		return fmt.Errorf("未找到 mihomo 二进制，请先运行 `gateway install`")
	}
	if err := os.MkdirAll(e.workdir, 0o755); err != nil {
		return fmt.Errorf("create workdir: %w", err)
	}

	// Preflight: check port conflicts so we fail fast with a clear error.
	checks := []PortCheck{
		{Label: "mihomo mixed (HTTP+SOCKS5)", Port: cfg.Runtime.Ports.Mixed, Bind: "0.0.0.0"},
		{Label: "mihomo API", Port: cfg.Runtime.Ports.API, Bind: "127.0.0.1"},
	}
	if cfg.Gateway.DNS.Enabled {
		checks = append(checks, PortCheck{Label: "DNS", Port: cfg.Gateway.DNS.Port, Bind: "0.0.0.0"})
	}
	if err := CheckPorts(checks); err != nil {
		return err
	}

	// 确保 GeoIP / GeoSite 文件齐全，避免 mihomo 启动时卡在下载。
	// 正常路径（install 已跑过）会命中缓存/workdir，秒过；
	// 冷启动（workdir 被清掉）会静默下载。
	upstream := localUpstreamURL(cfg)
	_ = mihomo.EnsureGeodata(e.workdir, e.cacheDir, upstream, nil)

	data, err := Render(ctx, cfg, e.workdir)
	if err != nil {
		return err
	}
	if err := os.WriteFile(e.ConfigPath(), data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	logPath := filepath.Join(e.workdir, "mihomo.log")
	_ = os.Truncate(logPath, 0) // start fresh so tail-on-fail shows only this run
	e.proc = newProcess(e.bin, e.workdir, logPath)
	if err := e.proc.Start(); err != nil {
		return fmt.Errorf("启动 mihomo 失败: %w", err)
	}
	e.api.baseURL = fmt.Sprintf("http://127.0.0.1:%d", cfg.Runtime.Ports.API)
	e.api.secret = cfg.Runtime.APISecret
	if err := e.api.WaitReady(ctx, 10*time.Second); err != nil {
		tail := TailLog(logPath, 20)
		_ = e.proc.Stop()
		if tail != "" {
			return fmt.Errorf("mihomo 启动超时（API 未就绪）。\n\nmihomo 日志最后 20 行：\n%s\n\n完整日志: %s", tail, logPath)
		}
		return fmt.Errorf("mihomo 启动超时且无日志输出，检查二进制权限: %s", e.bin)
	}
	return nil
}

// Stop kills the mihomo process. Safe to call if never started.
func (e *Engine) Stop() error {
	if e.proc == nil {
		return nil
	}
	return e.proc.Stop()
}

// Reload re-renders the config and asks mihomo to reload via API.
// If the API is unreachable, it falls back to a process restart.
func (e *Engine) Reload(ctx context.Context, cfg *configpkg.Config) error {
	data, err := Render(ctx, cfg, e.workdir)
	if err != nil {
		return err
	}
	if err := os.WriteFile(e.ConfigPath(), data, 0o600); err != nil {
		return err
	}
	if err := e.api.ReloadConfig(ctx, e.ConfigPath()); err == nil {
		return nil
	}
	// API failed; restart.
	_ = e.Stop()
	return e.Start(ctx, cfg)
}

// Running reports whether the mihomo child process is alive.
func (e *Engine) Running() bool {
	return e.proc != nil && e.proc.Alive()
}

// LogPath returns the path to the mihomo log file.
func (e *Engine) LogPath() string { return filepath.Join(e.workdir, "mihomo.log") }

// localUpstreamURL returns a proxy URL if the user's source is an external
// proxy — so geodata downloads can route through it when direct is blocked.
func localUpstreamURL(cfg *configpkg.Config) string {
	if cfg.Source.Type != configpkg.SourceTypeExternal {
		return ""
	}
	e := cfg.Source.External
	if e.Server == "" || e.Port == 0 {
		return ""
	}
	scheme := "http"
	if e.Kind == "socks5" {
		scheme = "socks5"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, e.Server, e.Port)
}
