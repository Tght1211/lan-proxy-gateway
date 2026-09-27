package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/config"
)

func init() {
	var typ, host string
	var port int
	command := &cobra.Command{Use: "egress", Short: "仅配置设备流量出口，保留 Mac 系统代理和 DNS"}
	proxy := &cobra.Command{Use: "proxy", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		if err := a.SetEgress(cmd.Context(), config.EgressConfig{Mode: config.EgressProxy, Proxy: config.ProxyConfig{Type: typ, Host: host, Port: port}}, true); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "设备代理出口已更新，Mac 系统设置未修改")
		return nil
	}}
	proxy.Flags().StringVar(&typ, "type", "socks5", "http|socks5")
	proxy.Flags().StringVar(&host, "host", "127.0.0.1", "代理地址")
	proxy.Flags().IntVar(&port, "port", 7897, "代理端口")
	command.AddCommand(proxy, &cobra.Command{Use: "direct", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		return a.SetEgress(cmd.Context(), config.EgressConfig{Mode: config.EgressDirect}, false)
	}})
	rootCmd.AddCommand(command)
}
