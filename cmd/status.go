package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/gateway"
)

var statusJSON bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看当前运行状态（--json 输出机器可读）",
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		s := a.Status()
		if statusJSON {
			b, _ := json.MarshalIndent(s, "", "  ")
			fmt.Println(string(b))
			return nil
		}
		title := color.New(color.FgCyan, color.Bold)
		title.Println("== lan-proxy-gateway · 状态 ==")
		fmt.Printf("  配置:   %v (%s)\n", s.Configured, s.ConfigFile)
		fmt.Printf("  运行:   %v\n", s.Running)
		if s.Egress == "proxy" {
			fmt.Printf("  出口:   代理 (%s)\n", s.Proxy)
		} else {
			fmt.Printf("  出口:   直连\n")
		}
		fmt.Println()
		fmt.Println(gateway.DeviceGuide(s.Gateway))
		return nil
	},
}
