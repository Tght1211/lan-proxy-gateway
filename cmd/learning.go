package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/app"
)

func init() {
	c := &cobra.Command{Use: "learning <accept|ignore|restore|undo|configure> <host|settings-json>", Short: "管理自动代理学习、暂停域名或调整学习设置", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		if err := app.NewAPIClient(a.Cfg.Runtime.APIPort, a.Paths.ConfigFile).Learning(cmd.Context(), args[0], args[1]); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "学习设置或记录已更新")
		return nil
	}}
	rootCmd.AddCommand(c)
}
