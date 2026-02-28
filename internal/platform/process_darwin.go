//go:build darwin

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func (p *impl) FindBinary() (string, error) {
	candidates := []string{
		"/opt/homebrew/opt/mihomo/bin/mihomo",
		"/usr/local/opt/mihomo/bin/mihomo",
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	if path, err := exec.LookPath("mihomo"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("未找到 mihomo 可执行文件")
}

func (p *impl) IsRunning() (bool, int, error) {
	out, err := exec.Command("pgrep", "-x", "mihomo").Output()
	if err != nil {
		return false, 0, nil
	}
	pidStr := strings.TrimSpace(string(out))
	// pgrep may return multiple PIDs, take the first
	if idx := strings.Index(pidStr, "\n"); idx > 0 {
		pidStr = pidStr[:idx]
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return false, 0, nil
	}
	return true, pid, nil
}

func (p *impl) StartProcess(binary, dataDir, logFile string) (int, error) {
	logF, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, fmt.Errorf("无法创建日志文件: %w", err)
	}

	cmd := exec.Command(binary, "-d", dataDir)
	cmd.Stdout = logF
	cmd.Stderr = logF
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		logF.Close()
		return 0, fmt.Errorf("mihomo 启动失败: %w", err)
	}

	pid := cmd.Process.Pid
	// Release the process so it doesn't become a zombie when this CLI exits
	cmd.Process.Release()
	logF.Close()

	return pid, nil
}

func (p *impl) StopProcess() error {
	// Graceful stop
	exec.Command("killall", "mihomo").Run()
	time.Sleep(2 * time.Second)

	// Check if still running, force kill
	if running, _, _ := p.IsRunning(); running {
		exec.Command("killall", "-9", "mihomo").Run()
	}
	return nil
}
