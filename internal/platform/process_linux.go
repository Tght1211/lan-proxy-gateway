//go:build linux

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
		"/usr/local/bin/mihomo",
		"/usr/bin/mihomo",
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

func (p *impl) GetBinaryPath() string {
	return "/usr/local/bin/mihomo"
}

func (p *impl) IsRunning() (bool, int, error) {
	out, err := exec.Command("pgrep", "-x", "mihomo").Output()
	if err != nil {
		return false, 0, nil
	}
	pidStr := strings.TrimSpace(string(out))
	if idx := strings.Index(pidStr, "\n"); idx > 0 {
		pidStr = pidStr[:idx]
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return false, 0, nil
	}
	return true, pid, nil
}

// pidFilePath 返回 mihomo PID 文件路径，供 systemd PIDFile= 指令使用
func pidFilePath(dataDir string) string {
	return dataDir + "/mihomo.pid"
}

func (p *impl) StartProcess(binary, dataDir, logFile string) (int, error) {
	rotateLog(logFile)

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
	cmd.Process.Release()
	logF.Close()

	// 写入 PID 文件，供 systemd Type=forking + PIDFile= 追踪进程用
	pidFile := pidFilePath(dataDir)
	_ = os.WriteFile(pidFile, []byte(strconv.Itoa(pid)+"\n"), 0644)

	return pid, nil
}

func rotateLog(logFile string) {
	const maxBackups = 3
	info, err := os.Stat(logFile)
	if err != nil || info.Size() == 0 {
		return
	}

	for i := maxBackups - 1; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", logFile, i)
		dst := fmt.Sprintf("%s.%d", logFile, i+1)
		os.Rename(src, dst)
	}
	os.Rename(logFile, logFile+".1")
}

func (p *impl) StopProcess() error {
	out, _ := exec.Command("pgrep", "-x", "mihomo").Output()
	pidStr := strings.TrimSpace(string(out))
	if pidStr == "" {
		return nil
	}
	for _, s := range strings.Split(pidStr, "\n") {
		if pid, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			syscall.Kill(pid, syscall.SIGTERM)
		}
	}
	time.Sleep(2 * time.Second)

	// Force kill if still running
	if running, _, _ := p.IsRunning(); running {
		out, _ = exec.Command("pgrep", "-x", "mihomo").Output()
		for _, s := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if pid, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
				syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	}
	return nil
}
