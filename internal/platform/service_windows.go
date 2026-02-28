//go:build windows

package platform

import (
	"fmt"
	"os/exec"
)

func (p *impl) InstallService(cfg ServiceConfig) error {
	binPath := fmt.Sprintf(`"%s" start --config "%s" --data-dir "%s"`, cfg.BinaryPath, cfg.ConfigFile, cfg.DataDir)

	if err := exec.Command("sc", "create", "lan-proxy-gateway",
		"binPath=", binPath,
		"start=", "auto",
		"DisplayName=", "LAN Proxy Gateway",
	).Run(); err != nil {
		return fmt.Errorf("创建服务失败: %w", err)
	}

	if err := exec.Command("sc", "start", "lan-proxy-gateway").Run(); err != nil {
		return fmt.Errorf("启动服务失败: %w", err)
	}
	return nil
}

func (p *impl) UninstallService() error {
	exec.Command("sc", "stop", "lan-proxy-gateway").Run()
	if err := exec.Command("sc", "delete", "lan-proxy-gateway").Run(); err != nil {
		return fmt.Errorf("删除服务失败: %w", err)
	}
	return nil
}
