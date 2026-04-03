package cmd

import (
	"github.com/spf13/cobra"
)

var restartSimple bool

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "重启代理网关",
	Run: func(cmd *cobra.Command, args []string) {
		startSimple = restartSimple
		runStop(cmd, args)
		runStart(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)
	restartCmd.Flags().BoolVar(&restartSimple, "simple", false, "使用纯命令模式重启：不进入 TUI，使用兼容性更好的命令交互")
}
