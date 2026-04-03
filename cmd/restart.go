package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

var restartSimple bool
var restartTUI bool

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "重启代理网关",
	Run: func(cmd *cobra.Command, args []string) {
		simpleMode, err := resolveConsoleSimpleMode(cmd, true, restartSimple, restartTUI)
		if err != nil {
			ui.Error("%s", err)
			os.Exit(1)
		}

		runStop(cmd, args)
		runStartWithMode(simpleMode, cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)
	restartCmd.Flags().BoolVar(&restartSimple, "simple", false, "使用纯命令模式重启（默认）")
	restartCmd.Flags().BoolVar(&restartTUI, "tui", false, "使用 TUI 工作台重启")
}
