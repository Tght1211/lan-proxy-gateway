package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/ui"
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
	Run: runRootHome,
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

func runRootHome(cmd *cobra.Command, args []string) {
	cfg := loadConfigOrDefault()
	updateNotice := loadUpdateNotice()

	ui.ShowLogo()
	color.New(color.Bold).Println("LAN Proxy Gateway")
	fmt.Println()
	color.New(color.Faint).Println("  核心亮点 1: 局域网共享")
	color.New(color.Faint).Println("  Switch / PS5 / Apple TV / 手机等设备，只改网关和 DNS 就能用")
	color.New(color.Faint).Println("  核心亮点 2: 链式代理")
	color.New(color.Faint).Println("  Claude / ChatGPT / Codex / Cursor 等流量可切到住宅出口，降低风控")
	fmt.Println()
	if updateNotice != nil {
		color.New(color.FgYellow, color.Bold).Printf("  新版本可用: %s", updateNotice.Latest)
		fmt.Println()
		if runtime.GOOS == "windows" {
			color.New(color.Faint).Println("  在管理员终端中运行 gateway update 可一键升级；直连失败时会自动尝试镜像")
		} else {
			color.New(color.Faint).Println("  运行 sudo gateway update 可一键升级；直连失败时会自动尝试镜像")
		}
		fmt.Println()
	}
	ui.Separator()
	fmt.Println()
	fmt.Printf("  %-14s %s\n", "配置文件:", displayConfigPath())
	fmt.Printf("  %-14s %s\n", "代理来源:", cfg.Proxy.Source)
	fmt.Printf("  %-14s %s\n", "TUN:", onOff(cfg.Runtime.Tun.Enabled))
	fmt.Printf("  %-14s %s\n", "扩展模式:", extensionModeName(cfg.Extension.Mode))
	fmt.Printf("  %-14s %s\n", "配置中心:", color.CyanString("gateway config"))
	fmt.Println()
	fmt.Println(color.New(color.Bold).Sprint("  推荐流程:"))
	fmt.Println("    1. gateway install")
	fmt.Println("    2. gateway config")
	if runtime.GOOS == "windows" {
		fmt.Println("    3. gateway start   (需要管理员终端)")
	} else {
		fmt.Println("    3. sudo gateway start")
	}
	fmt.Println("    4. gateway status")
	fmt.Println()
	fmt.Println(color.New(color.Bold).Sprint("  常用入口:"))
	fmt.Println("    gateway config         交互式配置中心")
	fmt.Println("    gateway chains         链式代理向导")
	fmt.Println("    gateway tun on         开启局域网透明代理")
	fmt.Println("    gateway switch         查看代理来源 / 扩展模式")
	fmt.Println()
	fmt.Println(color.New(color.Faint).Sprint("  完整命令列表见下方："))
	fmt.Println()
	_ = cmd.Help()
}
