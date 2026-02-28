//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (p *impl) FindBinary() (string, error) {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "mihomo", "mihomo.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "mihomo", "mihomo.exe"),
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	if path, err := exec.LookPath("mihomo.exe"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("mihomo"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("未找到 mihomo 可执行文件")
}

func (p *impl) IsRunning() (bool, int, error) {
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq mihomo.exe", "/FO", "CSV", "/NH").Output()
	if err != nil {
		return false, 0, nil
	}
	output := strings.TrimSpace(string(out))
	if output == "" || strings.Contains(output, "No tasks") || strings.Contains(output, "没有") {
		return false, 0, nil
	}
	// Parse CSV: "mihomo.exe","1234","Console","1","12,345 K"
	fields := strings.Split(output, ",")
	if len(fields) >= 2 {
		pidStr := strings.Trim(fields[1], "\" ")
		if pid, err := strconv.Atoi(pidStr); err == nil {
			return true, pid, nil
		}
	}
	return true, 0, nil
}

func (p *impl) StartProcess(binary, dataDir, logFile string) (int, error) {
	logF, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, fmt.Errorf("无法创建日志文件: %w", err)
	}

	cmd := exec.Command(binary, "-d", dataDir)
	cmd.Stdout = logF
	cmd.Stderr = logF

	if err := cmd.Start(); err != nil {
		logF.Close()
		return 0, fmt.Errorf("mihomo 启动失败: %w", err)
	}

	pid := cmd.Process.Pid
	cmd.Process.Release()
	logF.Close()

	return pid, nil
}

func (p *impl) StopProcess() error {
	exec.Command("taskkill", "/IM", "mihomo.exe").Run()
	time.Sleep(2 * time.Second)

	if running, _, _ := p.IsRunning(); running {
		exec.Command("taskkill", "/IM", "mihomo.exe", "/F").Run()
	}
	return nil
}
