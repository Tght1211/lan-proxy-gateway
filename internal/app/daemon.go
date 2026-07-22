package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/dns"
	"github.com/tght/lan-proxy-gateway/internal/platform"
	"github.com/tght/lan-proxy-gateway/internal/procutil"
	"github.com/tght/lan-proxy-gateway/internal/relay"
)

// ---------- lifecycle (called from any process) ----------

// Running reports whether the daemon is up: pid alive, or (cross-uid case
// where the pidfile is unreadable) the loopback API answers a TCP probe.
func (a *App) Running() bool {
	if pid := readPIDFile(a.Paths.PIDFile); pid > 0 && procutil.PIDAlive(pid) {
		return true
	}
	return probeTCP(fmt.Sprintf("127.0.0.1:%d", a.Cfg.Runtime.APIPort), 300*time.Millisecond)
}

// Start spawns the daemon detached and waits for readiness, then returns.
// Use Run for the foreground/service-manager path.
func (a *App) Start(ctx context.Context) error {
	if !a.Configured() {
		return errors.New("尚未初始化，请先运行 `gateway install` 或进入控制台完成向导")
	}
	if a.Running() {
		return nil
	}
	if err := a.spawnDetached(); err != nil {
		return err
	}
	return a.waitReady(ctx, 8*time.Second)
}

// Stop signals the daemon (SIGTERM) and waits; as a belt-and-braces it also
// rolls back gateway state directly (idempotent when the daemon already did).
// If the host's own DNS was pointed at loopback, restore it — the DNS server
// it pointed at is going away.
func (a *App) Stop() error {
	var firstErr error
	if a.Plat != nil {
		if loopback, err := a.Plat.LocalDNSIsLoopback(); err == nil && loopback {
			if err := a.Plat.RestoreLocalDNS(); err != nil && !errors.Is(err, platform.ErrNotSupported) {
				firstErr = err
			}
		}
	}
	pid := readPIDFile(a.Paths.PIDFile)
	if pid > 0 && procutil.PIDAlive(pid) {
		proc, err := os.FindProcess(pid)
		if err == nil {
			_ = proc.Signal(syscall.SIGTERM)
		}
		for i := 0; i < 50; i++ {
			if !procutil.PIDAlive(pid) {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if procutil.PIDAlive(pid) {
			_ = procutil.KillPID(pid)
		}
	}
	_ = os.Remove(a.Paths.PIDFile)
	if a.Gateway != nil {
		if err := a.Gateway.Disable(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// ---------- daemon body ----------

// Run is the daemon body (the `gateway run` / `start --foreground` command).
// It blocks until ctx is cancelled (SIGTERM/SIGINT), then tears everything down.
func (a *App) Run(ctx context.Context) error {
	if !a.Configured() {
		return errors.New("尚未初始化")
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return fmt.Errorf("当前平台 %s 不支持旁路网关模式", runtime.GOOS)
	}

	logger, logFile, err := a.openLogger()
	if err != nil {
		return err
	}
	defer logFile.Close()

	// 端口预检：冲突时报出占用者，避免守护进程起来即死
	var checks []procutil.PortCheck
	checks = append(checks,
		procutil.PortCheck{Label: "relay", Port: a.Cfg.Runtime.RedirPort, Bind: "0.0.0.0"},
		procutil.PortCheck{Label: "api", Port: a.Cfg.Runtime.APIPort, Bind: "127.0.0.1"},
	)
	if a.Cfg.DNS.Enabled {
		checks = append(checks, procutil.PortCheck{Label: "dns", Port: a.Cfg.DNS.Port, Bind: "0.0.0.0"})
	}
	if err := procutil.CheckPorts(checks); err != nil {
		return err
	}

	origDST, err := relay.NewPlatformOrigDST()
	if err != nil {
		return fmt.Errorf("初始化透明转发失败: %w", err)
	}

	// 防火墙 + ip_forward
	if a.Cfg.Gateway.Enabled {
		if err := a.Gateway.Enable(firewallConfig(a.Cfg)); err != nil {
			return err
		}
	}

	rt, err := a.startServices(ctx, logger, origDST)
	if err != nil {
		_ = a.Gateway.Disable()
		return err
	}
	defer rt.shutdown()

	if err := writePIDFile(a.Paths.PIDFile); err != nil {
		logger.Warn("写 pidfile 失败", "err", err)
	}
	defer os.Remove(a.Paths.PIDFile)

	logger.Info("gateway 守护进程已就绪",
		"egress", a.Cfg.Egress.Mode,
		"lan_ip", a.Gateway.Info().IP,
		"redir_port", a.Cfg.Runtime.RedirPort,
		"dns_port", a.Cfg.DNS.Port)

	a.StartSupervisor(ctx)

	<-ctx.Done()
	logger.Info("收到退出信号，开始清理")
	return a.Gateway.Disable()
}

// startServices launches relay, DNS and the loopback API for the current config.
func (a *App) startServices(ctx context.Context, logger *slog.Logger, origDST relay.OrigDSTResolver) (*daemonRuntime, error) {
	rt := &daemonRuntime{logger: logger}
	rt.tracker = relay.NewTracker()
	rt.tracker.StartSampling(ctx, 5*time.Second)

	dialer, err := buildDialer(a.Cfg.Egress)
	if err != nil {
		return nil, err
	}
	rt.relay = relay.New(relay.Options{
		ListenAddr: net.JoinHostPort("", strconv.Itoa(a.Cfg.Runtime.RedirPort)),
		OrigDST:    origDST,
		Tracker:    rt.tracker,
		Dialer:     dialer,
		ViaProxy:   a.Cfg.Egress.Mode == config.EgressProxy,
		Logger:     logger,
	})
	if a.Cfg.DNS.Enabled {
		rt.dns = dns.New(dnsOptions(a.Cfg, a.Paths.FakeIPCacheFile, logger))
		rt.bindFakeIP()
		go func() {
			if err := rt.dns.ListenAndServe(ctx); err != nil && ctx.Err() == nil {
				logger.Error("DNS 服务异常退出", "err", err)
			}
		}()
	}
	go func() {
		if err := rt.relay.ListenAndServe(ctx); err != nil && ctx.Err() == nil {
			logger.Error("relay 异常退出", "err", err)
		}
	}()
	rt.api = newAPIServer(a, rt)
	go func() {
		if err := rt.api.ListenAndServe(ctx); err != nil && ctx.Err() == nil {
			logger.Error("状态 API 异常退出", "err", err)
		}
	}()
	go rt.watchConfig(ctx, a)
	return rt, nil
}

// daemonRuntime bundles the long-running services for shutdown/reload.
type daemonRuntime struct {
	logger  *slog.Logger
	tracker *relay.Tracker
	relay   *relay.Server
	dns     *dns.Server
	api     *apiServer
}

func (rt *daemonRuntime) shutdown() {
	if rt.relay != nil {
		_ = rt.relay.Close()
	}
	if rt.dns != nil {
		rt.dns.Shutdown()
	}
	if rt.api != nil {
		_ = rt.api.Close()
	}
}

// applyConfig re-reads the config file and live-applies deltas.
// Everything is idempotent: relay dialer swap, dns toggles, firewall re-sync.
func (rt *daemonRuntime) applyConfig(a *App, cfg *config.Config) {
	a.setCfg(cfg)
	if dialer, err := buildDialer(cfg.Egress); err == nil {
		rt.relay.SetDialer(dialer, cfg.Egress.Mode == config.EgressProxy)
	}
	if rt.dns != nil {
		rt.dns.SetUpstreams(cfg.DNS.Upstreams)
		rt.dns.SetFakeIPEnabled(cfg.Egress.Mode == config.EgressProxy && cfg.DNS.FakeIP)
	}
	rt.bindFakeIP()
	if cfg.Gateway.Enabled {
		if err := a.Gateway.Enable(firewallConfig(cfg)); err != nil {
			rt.logger.Warn("防火墙规则热应用失败", "err", err)
		}
	}
	rt.logger.Info("配置已热应用", "egress", cfg.Egress.Mode)
}

// bindFakeIP wires the relay's fake-ip lookup to the DNS server's pool.
func (rt *daemonRuntime) bindFakeIP() {
	if rt.dns == nil || rt.relay == nil {
		return
	}
	prefix := rt.dns.FakeIPRange()
	rt.relay.SetFakeIP(&prefix, rt.dns.LookupFakeIP)
	rt.relay.SetRealIPLookup(rt.dns.LookupRealIP)
}

// watchConfig polls the config file mtime and applies changes (belt; the
// /api/reload poke is the suspenders).
func (rt *daemonRuntime) watchConfig(ctx context.Context, a *App) {
	var lastMod time.Time
	if fi, err := os.Stat(a.Paths.ConfigFile); err == nil {
		lastMod = fi.ModTime()
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fi, err := os.Stat(a.Paths.ConfigFile)
			if err != nil || !fi.ModTime().After(lastMod) {
				continue
			}
			lastMod = fi.ModTime()
			cfg, err := config.LoadFrom(a.Paths.ConfigFile)
			if err != nil {
				rt.logger.Warn("配置文件变更但解析失败", "err", err)
				continue
			}
			rt.applyConfig(a, cfg)
		}
	}
}

// ---------- process plumbing ----------

func readPIDFile(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return pid
}

func writePIDFile(path string) error {
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644)
}

func probeTCP(addr string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// spawnDetached re-executes this binary as `gateway run` in a new session,
// with stdout/stderr appended to the daemon log file.
func (a *App) spawnDetached() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(a.Paths.Root, 0o755); err != nil {
		return err
	}
	logOut, err := os.OpenFile(a.Paths.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logOut.Close()

	cmd := exec.Command(exe, "run")
	cmd.Stdout = logOut
	cmd.Stderr = logOut
	cmd.Stdin = nil
	cmd.Dir = "/"
	cmd.SysProcAttr = detachSysProcAttr()
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动守护进程失败: %w", err)
	}
	// detach: the daemon is reparented to init; we never Wait for it
	go cmd.Wait()
	return nil
}

// waitReady polls the loopback API until the daemon serves it or timeout.
func (a *App) waitReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", a.Cfg.Runtime.APIPort)
	for time.Now().Before(deadline) {
		if probeTCP(addr, 300*time.Millisecond) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	tail := procutil.TailLog(a.Paths.LogFile, 20)
	if tail != "" {
		return fmt.Errorf("守护进程未在 %s 内就绪，日志末尾:\n%s", timeout, tail)
	}
	return fmt.Errorf("守护进程未在 %s 内就绪", timeout)
}

// openLogger routes slog to the daemon log file (daemon side only).
func (a *App) openLogger() (*slog.Logger, *os.File, error) {
	if err := os.MkdirAll(a.Paths.Root, 0o755); err != nil {
		return nil, nil, err
	}
	f, err := os.OpenFile(a.Paths.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, err
	}
	level := slog.LevelInfo
	switch strings.ToLower(a.Cfg.Runtime.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	logger := slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	return logger, f, nil
}
