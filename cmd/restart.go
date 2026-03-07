package cmd

import (
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "重启代理网关",
	Run: func(cmd *cobra.Command, args []string) {
		runStop(cmd, args)
		runStart(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)
}
