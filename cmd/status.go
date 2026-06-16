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
		fmt.Printf("  模式:   %s   广告拦截: %v   TUN: %v\n", s.Mode, s.Adblock, s.TUN)
		fmt.Printf("  源:     %s\n", s.Source)
		fmt.Printf("  端口:   mixed=%d  api=%d  redir=%d\n", s.Ports.Mixed, s.Ports.API, s.Ports.Redir)
		fmt.Printf("  mihomo: %s\n", firstNonEmpty(s.MihomoBin, "(未找到)"))
		fmt.Println()
		fmt.Println(gateway.DeviceGuide(s.Gateway, s.Ports.Mixed))
		return nil
	},
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
