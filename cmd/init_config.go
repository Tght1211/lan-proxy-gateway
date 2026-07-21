package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tght/lan-proxy-gateway/internal/app"
)

var initConfigCmd = &cobra.Command{
	Use:   "init",
	Short: "创建默认配置（适合图形界面和自动化调用）",
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		if a.Configured() {
			fmt.Fprintf(cmd.OutOrStdout(), "配置已存在: %s\n", a.Paths.ConfigFile)
			return nil
		}
		if err := a.Save(); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "已创建默认配置: %s\n", a.Paths.ConfigFile)
		return nil
	},
}
