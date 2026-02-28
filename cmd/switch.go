package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/platform"
	"github.com/tght/lan-proxy-gateway/internal/proxy"
	tmpl "github.com/tght/lan-proxy-gateway/internal/template"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

var switchCmd = &cobra.Command{
	Use:   "switch [url|file] [path]",
	Short: "切换代理来源（订阅链接 / 本地配置文件）",
	Long: `切换代理来源模式。

用法:
  gateway switch              # 查看当前模式
  gateway switch url          # 切换到订阅链接模式
  gateway switch file         # 切换到配置文件模式
  gateway switch file /path   # 切换并更新配置文件路径`,
	Args: cobra.MaximumNArgs(2),
	Run:  runSwitch,
}

func init() {
	rootCmd.AddCommand(switchCmd)
}

func runSwitch(cmd *cobra.Command, args []string) {
	cfgPath := resolveConfigPath()
	cfg := loadConfigOrDefault()

	// No args: show current mode
	if len(args) == 0 {
		fmt.Println()
		fmt.Printf("  %s %s\n", color.New(color.Bold).Sprint("当前模式:"), color.CyanString(cfg.ProxySource))
		if cfg.ProxySource == "url" {
			url := cfg.SubscriptionURL
			if len(url) > 50 {
				url = url[:50] + "..."
			}
			fmt.Printf("  %s %s\n", color.New(color.Bold).Sprint("订阅链接:"), url)
		} else {
			fmt.Printf("  %s %s\n", color.New(color.Bold).Sprint("配置文件:"), cfg.ProxyConfigFile)
		}
		fmt.Println()
		fmt.Printf("  %s\n", color.New(color.Faint).Sprint("用法: gateway switch [url|file] [配置文件路径]"))
		fmt.Println()
		return
	}

	target := args[0]
	if target != "url" && target != "file" {
		ui.Error("参数应为 url 或 file")
		os.Exit(1)
	}

	// Switch to url mode
	if target == "url" && cfg.SubscriptionURL == "" {
		ui.Error("未配置订阅链接，请先在 gateway.yaml 中设置 subscription_url")
		os.Exit(1)
	}

	// Switch to file mode
	if target == "file" {
		if len(args) >= 2 {
			path := args[1]
			if strings.HasPrefix(path, "~") {
				home, _ := os.UserHomeDir()
				path = filepath.Join(home, path[1:])
			}
			if _, err := os.Stat(path); os.IsNotExist(err) {
				ui.Error("文件不存在: %s", path)
				os.Exit(1)
			}
			// Validate proxies section
			count, err := proxy.ExtractProxies(path, os.DevNull)
			if err != nil {
				ui.Error("%s", err)
				os.Exit(1)
			}
			ui.Info("检测到 %d 个代理节点", count)
			cfg.ProxyConfigFile = path
		}
		if cfg.ProxyConfigFile == "" {
			ui.Error("未配置文件路径")
			fmt.Println("  用法: gateway switch file /path/to/config.yaml")
			os.Exit(1)
		}
	}

	oldSource := cfg.ProxySource
	cfg.ProxySource = target

	// Save config
	if cfgPath == ".secret" {
		cfgPath = "gateway.yaml"
	}
	if err := config.Save(cfg, cfgPath); err != nil {
		ui.Error("保存配置失败: %s", err)
		os.Exit(1)
	}

	if oldSource == target {
		ui.Info("当前已是 %s 模式，配置已更新", target)
	} else {
		ui.Success("已切换: %s → %s", oldSource, target)
	}

	// Ask to regenerate config
	fmt.Println()
	fmt.Print("是否立即重新生成配置？[Y/n] ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)

	if answer == "" || strings.ToLower(answer) == "y" {
		p := platform.New()
		dDir := ensureDataDir()
		iface, _ := p.DetectDefaultInterface()
		ip, _ := p.DetectInterfaceIP(iface)

		if cfg.ProxySource == "file" {
			providerFile := filepath.Join(dDir, "proxy_provider", cfg.SubscriptionName+".yaml")
			count, err := proxy.ExtractProxies(cfg.ProxyConfigFile, providerFile)
			if err != nil {
				ui.Error("提取代理节点失败: %s", err)
				os.Exit(1)
			}
			ui.Success("已提取 %d 个代理节点", count)
		}

		configPath := filepath.Join(dDir, "config.yaml")
		if err := tmpl.RenderTemplate(cfg, iface, ip, configPath); err != nil {
			ui.Error("配置生成失败: %s", err)
			os.Exit(1)
		}
		ui.Success("配置文件已生成")
		fmt.Println()
		ui.Info("如需生效，请重启网关: sudo gateway start")
	}
}
