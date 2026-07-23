package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/systemproxy"
)

var (
	systemProxyType string
	systemProxyHost string
	systemProxyPort int
	systemProxyJSON bool
)

var systemProxyCmd = &cobra.Command{
	Use:   "system-proxy",
	Short: "配置 macOS 系统代理和旁路由出口",
}

var systemProxyStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看系统代理",
	RunE: func(cmd *cobra.Command, args []string) error {
		status, err := systemproxy.Get()
		if err != nil && !errors.Is(err, systemproxy.ErrNotSupported) {
			return err
		}
		if errors.Is(err, systemproxy.ErrNotSupported) {
			a, appErr := app.New()
			if appErr != nil {
				return appErr
			}
			if systemProxyJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(a.Status())
			}
			appStatus := a.Status()
			if appStatus.Egress == config.EgressProxy {
				fmt.Fprintf(cmd.OutOrStdout(), "LAN 出口: %s\n", appStatus.Proxy)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "LAN 出口: 直连")
			}
			return nil
		}
		if systemProxyJSON {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(status)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "系统代理: %s\n", status.Summary())
		return nil
	},
}

var systemProxyOnCmd = &cobra.Command{
	Use:   "on",
	Short: "启用系统代理，并让旁路由使用同一个代理出口",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS == "darwin" {
			maybeElevate()
		}
		a, err := app.New()
		if err != nil {
			return err
		}
		previous := a.Cfg.Egress
		next := config.EgressConfig{
			Mode:  config.EgressProxy,
			Proxy: config.ProxyConfig{Type: systemProxyType, Host: systemProxyHost, Port: systemProxyPort},
		}
		if err := a.SetEgress(cmd.Context(), next, true); err != nil {
			return err
		}
		proxyErr := systemproxy.Enable(systemproxy.Mode(systemProxyType), systemProxyHost, systemProxyPort)
		if proxyErr != nil && !errors.Is(proxyErr, systemproxy.ErrNotSupported) {
			_ = a.SetEgress(cmd.Context(), previous, false)
			return proxyErr
		}
		if errors.Is(proxyErr, systemproxy.ErrNotSupported) {
			fmt.Fprintf(cmd.OutOrStdout(), "旁路由出口已启用: %s %s:%d\n", systemProxyType, systemProxyHost, systemProxyPort)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "系统代理和旁路由出口已启用: %s %s:%d\n", systemProxyType, systemProxyHost, systemProxyPort)
		}
		return nil
	},
}

var systemProxyOffCmd = &cobra.Command{
	Use:   "off",
	Short: "关闭系统代理，并让旁路由切回直连",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS == "darwin" {
			maybeElevate()
		}
		if err := systemproxy.Disable(); err != nil && !errors.Is(err, systemproxy.ErrNotSupported) {
			return err
		}
		a, err := app.New()
		if err != nil {
			return err
		}
		if err := a.SetEgress(cmd.Context(), config.EgressConfig{Mode: config.EgressDirect}, false); err != nil {
			return fmt.Errorf("系统代理已关闭，但旁路由切回直连失败: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "系统代理已关闭，旁路由已切回直连")
		return nil
	},
}

var systemProxyTestCmd = &cobra.Command{
	Use:   "test",
	Short: "测试候选代理连通性(不保存配置)",
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		candidate := config.EgressConfig{
			Mode:  config.EgressProxy,
			Proxy: config.ProxyConfig{Type: systemProxyType, Host: systemProxyHost, Port: systemProxyPort},
		}
		if err := a.TestEgressConfig(cmd.Context(), candidate); err != nil {
			return fmt.Errorf("代理连通性测试失败: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "代理连通正常: %s %s:%d\n", systemProxyType, systemProxyHost, systemProxyPort)
		return nil
	},
}

func init() {
	systemProxyStatusCmd.Flags().BoolVar(&systemProxyJSON, "json", false, "JSON 输出")
	systemProxyOnCmd.Flags().StringVar(&systemProxyType, "type", "socks5", "http|socks5")
	systemProxyOnCmd.Flags().StringVar(&systemProxyHost, "host", "127.0.0.1", "代理地址")
	systemProxyOnCmd.Flags().IntVar(&systemProxyPort, "port", 7897, "代理端口")
	systemProxyTestCmd.Flags().StringVar(&systemProxyType, "type", "socks5", "http|socks5")
	systemProxyTestCmd.Flags().StringVar(&systemProxyHost, "host", "127.0.0.1", "代理地址")
	systemProxyTestCmd.Flags().IntVar(&systemProxyPort, "port", 7897, "代理端口")
	systemProxyCmd.AddCommand(systemProxyStatusCmd, systemProxyOnCmd, systemProxyOffCmd, systemProxyTestCmd)
}
