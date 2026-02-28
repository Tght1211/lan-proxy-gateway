//go:build linux

package platform

import (
	"fmt"
	"os"
	"os/exec"
)

const systemdUnit = `[Unit]
Description=LAN Proxy Gateway
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart={{.BinaryPath}} start --config {{.ConfigFile}} --data-dir {{.DataDir}}
WorkingDirectory={{.WorkDir}}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`

const unitPath = "/etc/systemd/system/lan-proxy-gateway.service"

func (p *impl) InstallService(cfg ServiceConfig) error {
	os.MkdirAll(cfg.LogDir, 0755)

	// Simple string replacement instead of text/template to keep it minimal
	content := systemdUnit
	content = replaceAll(content, "{{.BinaryPath}}", cfg.BinaryPath)
	content = replaceAll(content, "{{.ConfigFile}}", cfg.ConfigFile)
	content = replaceAll(content, "{{.DataDir}}", cfg.DataDir)
	content = replaceAll(content, "{{.WorkDir}}", cfg.WorkDir)

	if err := os.WriteFile(unitPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("无法创建 systemd 服务文件: %w", err)
	}

	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("systemctl daemon-reload 失败: %w", err)
	}
	if err := exec.Command("systemctl", "enable", "--now", "lan-proxy-gateway").Run(); err != nil {
		return fmt.Errorf("启用服务失败: %w", err)
	}
	return nil
}

func (p *impl) UninstallService() error {
	exec.Command("systemctl", "disable", "--now", "lan-proxy-gateway").Run()

	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("无法删除 systemd 服务文件: %w", err)
	}
	exec.Command("systemctl", "daemon-reload").Run()
	return nil
}

func replaceAll(s, old, new string) string {
	for {
		idx := indexOf(s, old)
		if idx < 0 {
			break
		}
		s = s[:idx] + new + s[idx+len(old):]
	}
	return s
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
