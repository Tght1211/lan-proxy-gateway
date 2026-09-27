package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/hotspot"
)

var hotspotCmd = &cobra.Command{Use: "hotspot", Short: "通过 Mac 互联网共享接入游戏机，不修改本机 DNS 或系统代理"}

func init() {
	hotspotCmd.AddCommand(&cobra.Command{Use: "status", Args: cobra.NoArgs, Short: "检测 Wi-Fi 共享（JSON）", RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		s := hotspot.Detect()
		s.Enabled = a.Cfg.Gateway.AccessMode == "hotspot" && a.Cfg.Gateway.Enabled
		return json.NewEncoder(cmd.OutOrStdout()).Encode(s)
	}})
	for _, action := range []string{"enable", "disable", "use-lan"} {
		action := action
		hotspotCmd.AddCommand(&cobra.Command{Use: action, Args: cobra.NoArgs, Short: map[string]string{"enable": "开启热点接管", "disable": "停止热点接管，保留系统 Wi-Fi 共享", "use-lan": "切回手动网关接入"}[action], RunE: func(cmd *cobra.Command, args []string) error {
			a, err := app.New()
			if err != nil {
				return err
			}
			if action == "enable" {
				s := hotspot.Detect()
				if !s.Available {
					return fmt.Errorf("%s", s.Message)
				}
			}
			a.Cfg.Gateway.AccessMode = "hotspot"
			a.Cfg.Gateway.Enabled = action != "disable"
			if action == "use-lan" {
				a.Cfg.Gateway.AccessMode = ""
			}
			if action == "enable" {
				a.Cfg.DNS.Enabled = true
				if a.Cfg.DNS.Port == 53 {
					a.Cfg.DNS.Port = config.DefaultDNSPort
				}
			}
			config.Normalize(a.Cfg)
			if err := config.Validate(a.Cfg); err != nil {
				return err
			}
			if err := a.Save(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "接入设置已保存，重启核心后生效；系统 Wi-Fi 共享保持原样。")
			return nil
		}})
	}
	rootCmd.AddCommand(hotspotCmd)
}
