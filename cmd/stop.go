package cmd

import (
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/tght/lan-proxy-gateway/internal/app"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "停止网关",
	RunE: func(cmd *cobra.Command, args []string) error {
		maybeElevate()
		a, err := app.New()
		if err != nil {
			return err
		}
		if err := a.Stop(); err != nil {
			return err
		}
		color.Green("✔ 已停止")
		return nil
	},
}
