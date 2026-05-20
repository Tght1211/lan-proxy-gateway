package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/webui"
)

const webUIServeCommand = "__webui-serve"

var webuiServeCmd = &cobra.Command{
	Use:    webUIServeCommand,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		if !a.Configured() {
			return fmt.Errorf("尚未完成初始化，请先运行 `gateway install` 或直接运行 `gateway` 进入向导")
		}
		if a.Cfg.Runtime.Ports.WebUI <= 0 {
			return nil
		}

		app.InjectWebUIVersion(Version)
		a.StartSupervisor(cmd.Context())
		srv := webui.New(
			webui.PortFromInt(a.Cfg.Runtime.Ports.WebUI),
			a.Cfg.Runtime.WebUIToken,
			app.NewWebUIController(a),
		)
		if err := srv.Start(cmd.Context(), func(format string, args ...any) {
			fmt.Fprintf(os.Stderr, format+"\n", args...)
		}); err != nil {
			return err
		}

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	},
}

func startWebUIDaemon(a *app.App) error {
	if a == nil || a.Cfg.Runtime.Ports.WebUI <= 0 {
		return nil
	}
	base := fmt.Sprintf("http://127.0.0.1:%d/", a.Cfg.Runtime.Ports.WebUI)
	if probeURL(base) {
		return nil
	}

	if err := os.MkdirAll(a.Paths.Root, 0o755); err != nil {
		return fmt.Errorf("create webui runtime dir: %w", err)
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	logPath := filepath.Join(a.Paths.Root, "webui.log")
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open webui log: %w", err)
	}
	defer log.Close()

	c := exec.Command(self, webUIServeCommand)
	c.Stdout = log
	c.Stderr = log
	configureWebUIDaemonCmd(c)
	if err := c.Start(); err != nil {
		return fmt.Errorf("start webui daemon: %w", err)
	}
	if err := os.WriteFile(webUIPIDPath(a.Paths), []byte(strconv.Itoa(c.Process.Pid)), 0o644); err != nil {
		_ = c.Process.Kill()
		return fmt.Errorf("write webui pid: %w", err)
	}
	_ = c.Process.Release()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if probeURL(base) {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("Web 控制台后台进程已启动但未就绪；日志: %s", logPath)
}

func stopWebUIDaemon(paths config.Paths) {
	data, err := os.ReadFile(webUIPIDPath(paths))
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err == nil && pid > 0 {
		terminateWebUIDaemon(pid)
	}
	_ = os.Remove(webUIPIDPath(paths))
}

func webUIPIDPath(paths config.Paths) string {
	return filepath.Join(paths.Root, "webui.pid")
}
