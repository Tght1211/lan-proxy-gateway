package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/platform"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "停止代理网关",
	Run:   runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) {
	checkRoot()

	ui.ShowLogo()
	p := platform.New()

	// Step 1: Stop mihomo
	ui.Step(1, 3, "停止 mihomo...")
	running, _, _ := p.IsRunning()
	if running {
		if err := p.StopProcess(); err != nil {
			ui.Error("停止 mihomo 失败: %s", err)
		} else {
			ui.Success("mihomo 已停止")
		}
	} else {
		ui.Info("mihomo 未在运行")
	}

	// Step 2: Clear firewall
	ui.Step(2, 3, "清除防火墙规则...")
	p.ClearFirewallRules()
	ui.Success("防火墙规则已清除")

	// Step 3: Disable IP forwarding
	ui.Step(3, 3, "关闭 IP 转发...")
	p.DisableIPForwarding()
	ui.Success("IP 转发已关闭")

	fmt.Println()
	ui.Separator()
	color.New(color.FgGreen, color.Bold).Println("  LAN Proxy Gateway 已停止")
	ui.Separator()
	fmt.Println()
	fmt.Printf("  %s\n", color.New(color.Faint).Sprint("设备网络设置可恢复为自动获取 (DHCP)"))
	fmt.Println()
}
