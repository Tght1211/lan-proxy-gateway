package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/platform"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "管理开机自启动服务",
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "安装开机自启动服务",
	Run:   runServiceInstall,
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "卸载开机自启动服务",
	Run:   runServiceUninstall,
}

func init() {
	serviceCmd.AddCommand(serviceInstallCmd)
	serviceCmd.AddCommand(serviceUninstallCmd)
	rootCmd.AddCommand(serviceCmd)
}

func runServiceInstall(cmd *cobra.Command, args []string) {
	checkRoot()

	p := platform.New()

	// Get the path to this binary
	self, err := os.Executable()
	if err != nil {
		ui.Error("无法获取可执行文件路径: %s", err)
		os.Exit(1)
	}
	self, _ = filepath.Abs(self)

	workDir, _ := os.Getwd()
	cfgPath, _ := filepath.Abs(resolveConfigPath())
	dDir, _ := filepath.Abs(resolveDataDir())
	logDir := filepath.Join(workDir, "logs")

	cfg := platform.ServiceConfig{
		BinaryPath: self,
		DataDir:    dDir,
		ConfigFile: cfgPath,
		LogDir:     logDir,
		WorkDir:    workDir,
	}

	if err := p.InstallService(cfg); err != nil {
		ui.Error("安装服务失败: %s", err)
		os.Exit(1)
	}

	ui.Success("开机自启动服务已安装")
	fmt.Println()
	if runtime.GOOS == "windows" {
		fmt.Println("  已安装开机自启任务；Windows 启动后会自动拉起网关。")
	} else {
		fmt.Println("  服务将在开机时自动启动，崩溃时自动重启。")
		fmt.Printf("  数据目录: %s\n", dDir)
		fmt.Println()
		color.New(color.Faint).Println("  如已安装旧版服务导致自启动失败，请重新安装：")
		color.New(color.Faint).Println("    sudo gateway service uninstall && sudo gateway service install")
	}
	fmt.Println()
}

func runServiceUninstall(cmd *cobra.Command, args []string) {
	checkRoot()

	p := platform.New()

	if err := p.UninstallService(); err != nil {
		ui.Error("卸载服务失败: %s", err)
		os.Exit(1)
	}

	ui.Success("开机自启动服务已卸载")
}
