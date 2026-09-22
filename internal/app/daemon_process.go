package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/procutil"
)

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
