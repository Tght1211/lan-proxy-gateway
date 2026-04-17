//go:build windows

package platform

import (
	"fmt"
	"os/exec"
)

const windowsStartupTaskName = "LAN Proxy Gateway"

func (p *impl) InstallService(cfg ServiceConfig) error {
	taskCmd := buildWindowsStartupTaskCommand(cfg)

	if err := exec.Command("schtasks",
		"/Create",
		"/TN", windowsStartupTaskName,
		"/SC", "ONSTART",
		"/RU", "SYSTEM",
		"/RL", "HIGHEST",
		"/TR", taskCmd,
		"/F",
	).Run(); err != nil {
		return fmt.Errorf("创建开机自启任务失败: %w", err)
	}

	if err := exec.Command("schtasks", "/Run", "/TN", windowsStartupTaskName).Run(); err != nil {
		return fmt.Errorf("启动开机任务失败: %w", err)
	}
	return nil
}

func (p *impl) UninstallService() error {
	if err := exec.Command("schtasks", "/Delete", "/TN", windowsStartupTaskName, "/F").Run(); err != nil {
		return fmt.Errorf("删除开机自启任务失败: %w", err)
	}
	return nil
}
