package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/tght/lan-proxy-gateway/internal/platform"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "安装 / 卸载 / 查看系统服务（开机自启）",
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "安装为系统服务（launchd / systemd / schtasks）",
	RunE: func(cmd *cobra.Command, args []string) error {
		maybeElevate()
		plat := platform.Current()
		binPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("获取当前可执行文件: %w", err)
		}
		if err := plat.InstallService(binPath); err != nil {
			return err
		}
		color.Green("✔ 服务已安装")
		return nil
	},
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "卸载系统服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		maybeElevate()
		if err := platform.Current().UninstallService(); err != nil {
			return err
		}
		color.Green("✔ 服务已卸载")
		return nil
	},
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看系统服务状态",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := platform.Current().ServiceStatus()
		if err != nil {
			return err
		}
		fmt.Println(s)
		return nil
	},
}

func init() {
	serviceCmd.AddCommand(serviceInstallCmd, serviceUninstallCmd, serviceStatusCmd)
}
