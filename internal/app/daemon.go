package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
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
	if err := a.prepareDNSPort(logger); err != nil {
		return err
	}

	// 端口预检：冲突时报出占用者，避免守护进程起来即死
	var checks []procutil.PortCheck
	checks = append(checks,
		procutil.PortCheck{Label: "relay", Port: a.Cfg.Runtime.RedirPort, Bind: "0.0.0.0"},
		procutil.PortCheck{Label: "api", Port: a.Cfg.Runtime.APIPort, Bind: "127.0.0.1"},
	)
	if a.Cfg.DNS.Enabled {
		checks = append(checks, procutil.PortCheck{Label: "dns", Port: a.Cfg.DNS.Port, Bind: "0.0.0.0"})
	}
	if a.Cfg.Egress.Mode == config.EgressProxy && a.Cfg.DNS.FakeIP {
		checks = append(checks, procutil.PortCheck{Label: "udp-relay", Port: a.Cfg.Runtime.UDPRedirPort, Bind: "0.0.0.0"})
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
	defer func() {
		rt.shutdown()
		if err := rt.tracker.SaveHistory(); err != nil {
			logger.Error("保存流量历史失败", "err", err)
		}
	}()

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

	// Monitor sub-service crashes; allow limited restarts before giving up.
	const maxRestarts = 3
	for {
		select {
		case <-ctx.Done():
			logger.Info("收到退出信号，开始清理")
			return a.Gateway.Disable()
		case ev := <-rt.eventCh:
			rt.crashCounts[ev.name]++
			count := rt.crashCounts[ev.name]
			logger.Error("子服务异常退出", "service", ev.name, "err", ev.err,
				"restart_count", count, "max", maxRestarts)
			if count > maxRestarts {
				logger.Error("子服务多次重启失败，撤销防火墙规则并退出",
					"service", ev.name, "restarts", count)
				_ = a.Gateway.Disable()
				return fmt.Errorf("子服务 %s 崩溃超过 %d 次: %w", ev.name, maxRestarts, ev.err)
			}
			time.Sleep(time.Duration(count) * time.Second)
			rt.restartService(ctx, a, ev.name)
		}
	}
}
