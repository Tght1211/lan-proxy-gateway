package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	dataDir string
	version = "dev"
)

var rootCmd = &cobra.Command{
	Use:   "gateway",
	Short: "LAN Proxy Gateway — 把你的电脑变成全屋透明代理网关",
	Long: `LAN Proxy Gateway 通过 mihomo 内核，将你的电脑变成局域网透明代理网关。
支持 macOS / Linux / Windows，支持订阅链接和本地配置文件。

设备（Switch、Apple TV、PS5 等）只需将网关和 DNS 指向本机即可科学上网。`,
}

func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件路径 (默认: ./gateway.yaml)")
	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "", "数据目录路径 (默认: ./data)")
}
