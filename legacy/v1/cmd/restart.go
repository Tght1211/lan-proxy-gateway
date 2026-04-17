package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var restartSimple bool
var restartTUI bool

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "重启代理网关",
	Run: func(cmd *cobra.Command, args []string) {
		if rejectRemovedTUIFlag(cmd) {
			os.Exit(1)
		}

		runStop(cmd, args)
		runStartWithMode(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)
	restartCmd.Flags().BoolVar(&restartSimple, "simple", false, "兼容旧参数；当前默认就是菜单式 CLI 控制台")
	restartCmd.Flags().BoolVar(&restartTUI, "tui", false, "已移除：旧版 TUI 工作台")
	_ = restartCmd.Flags().MarkHidden("simple")
	_ = restartCmd.Flags().MarkHidden("tui")
}
