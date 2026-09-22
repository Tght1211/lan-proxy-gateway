package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/app"
)

func init() {
	c := &cobra.Command{Use: "learning <accept|ignore|restore|undo> <host>", Short: "确认、忽略或撤销规则建议", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		if err := app.NewAPIClient(a.Cfg.Runtime.APIPort).Learning(cmd.Context(), args[0], args[1]); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "规则建议已更新")
		return nil
	}}
	rootCmd.AddCommand(c)
}
