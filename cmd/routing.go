package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/config"
)

var (
	routingRulesJSON string
	routingListJSON  bool
)

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

var routingListCmd = &cobra.Command{
	Use:   "list",
	Short: "按优先级列出分流规则（含自动学习标记）",
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		rules := a.Cfg.Routing.Rules
		if routingListJSON {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(rules)
		}
		if len(rules) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "暂无分流规则，全部流量使用默认出口")
			return nil
		}
		for i, rule := range rules {
			marker := ""
			if rule.Group != "" {
				marker += "  [" + rule.Group + "]"
			}
			if rule.Learned {
				marker += "  [自动学习]"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%2d. %-14s %-40s %s%s\n", i+1, rule.Type, rule.Value, rule.Action, marker)
		}
		return nil
	},
}

func init() {
	routingSetCmd.Flags().StringVar(&routingRulesJSON, "rules-json", "[]", "规则 JSON 数组")
	routingListCmd.Flags().BoolVar(&routingListJSON, "json", false, "JSON 输出")
	routingCmd.AddCommand(routingSetCmd, routingListCmd)
}
