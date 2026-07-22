package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/config"
)

var routingRulesJSON string

var routingCmd = &cobra.Command{
	Use:   "routing",
	Short: "管理域名分流规则",
}

var routingSetCmd = &cobra.Command{
	Use:   "set",
	Short: "替换全部分流规则",
	RunE: func(cmd *cobra.Command, args []string) error {
		var rules []config.RoutingRule
		if err := json.Unmarshal([]byte(routingRulesJSON), &rules); err != nil {
			return fmt.Errorf("解析规则 JSON: %w", err)
		}
		a, err := app.New()
		if err != nil {
			return err
		}
		if err := a.SetRoutingRules(rules); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "已应用 %d 条分流规则\n", len(rules))
		return nil
	},
}

func init() {
	routingSetCmd.Flags().StringVar(&routingRulesJSON, "rules-json", "[]", "规则 JSON 数组")
	routingCmd.AddCommand(routingSetCmd)
}
