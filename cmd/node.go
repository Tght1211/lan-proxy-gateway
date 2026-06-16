package cmd

// node.go 提供无交互的节点查看 / 切换命令（走 mihomo REST API）。
// 需要网关正在运行（mihomo 起着）才能用。

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/engine"
)

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "查看 / 切换代理节点（需网关运行中）",
}

// runningClient 返回一个可用的 mihomo API client，或在未运行时报清晰错误。
func runningClient() (*engine.Client, error) {
	a, err := app.New()
	if err != nil {
		return nil, err
	}
	if a.Engine == nil || !a.Engine.Running() {
		return nil, fmt.Errorf("网关未运行，先 `gateway start`")
	}
	return a.Engine.API(), nil
}

var nodeListJSON bool

var nodeListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有策略组及其节点和当前选择（--json 机器可读）",
	RunE: func(cmd *cobra.Command, args []string) error {
		cli, err := runningClient()
		if err != nil {
			return err
		}
		groups, err := cli.ListProxyGroups(context.Background())
		if err != nil {
			return err
		}
		if nodeListJSON {
			b, _ := json.MarshalIndent(groups, "", "  ")
			fmt.Println(string(b))
			return nil
		}
		for _, g := range groups {
			fmt.Printf("[%s] (%s)  当前: %s\n", g.Name, g.Type, g.Now)
			for _, n := range g.All {
				mark := "  "
				if n == g.Now {
					mark = "→ "
				}
				fmt.Printf("    %s%s\n", mark, n)
			}
		}
		return nil
	},
}

var nodeSwitchCmd = &cobra.Command{
	Use:   "switch <group> <node>",
	Short: "把某策略组切到指定节点",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cli, err := runningClient()
		if err != nil {
			return err
		}
		if err := cli.SelectNode(context.Background(), args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("✓ 组 %q 已切到节点 %q\n", args[0], args[1])
		return nil
	},
}

func init() {
	nodeListCmd.Flags().BoolVar(&nodeListJSON, "json", false, "机器可读 JSON 输出")
	nodeCmd.AddCommand(nodeListCmd, nodeSwitchCmd)
}
