package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

var tunCmd = &cobra.Command{
	Use:   "tun <on|off>",
	Short: "开启或关闭 TUN 透明代理模式",
	Long: `控制 TUN 模式开关，修改后需重启网关生效。

TUN 模式开启后，mihomo 会创建虚拟网卡接管所有流量，实现真正的透明代理。
如果已有其他 TUN 程序运行（如 Clash Verge），请先关闭它们再开启此模式。

示例:
  gateway tun on    # 开启 TUN 模式
  gateway tun off   # 关闭 TUN 模式（默认）`,
	Args: cobra.ExactArgs(1),
	Run:  runTun,
}

func init() {
	rootCmd.AddCommand(tunCmd)
}

func runTun(cmd *cobra.Command, args []string) {
	action := args[0]
	if action != "on" && action != "off" {
		ui.Error("参数错误，请使用 on 或 off")
		os.Exit(1)
	}

	cfg := loadConfigRequired()
	enabled := action == "on"

	if cfg.Runtime.Tun.Enabled == enabled {
		if enabled {
			ui.Warn("TUN 模式已经是开启状态")
		} else {
			ui.Warn("TUN 模式已经是关闭状态")
		}
		return
	}

	cfg.Runtime.Tun.Enabled = enabled

	cfgPath := resolveConfigPath()
	if cfgPath == ".secret" {
		cfgPath = "gateway.yaml"
	}
	if err := config.Save(cfg, cfgPath); err != nil {
		ui.Error("保存配置失败: %s", err)
		os.Exit(1)
	}

	if enabled {
		ui.Success("TUN 模式已开启")
		fmt.Println()
		fmt.Println("  注意：请确保没有其他 TUN 程序运行（如 Clash Verge），否则会产生冲突。")
	} else {
		ui.Success("TUN 模式已关闭")
	}
	fmt.Println()
	fmt.Println("  执行以下命令使配置生效：")
	fmt.Printf("    %s\n", elevatedCmd("restart"))
	fmt.Println()
}
