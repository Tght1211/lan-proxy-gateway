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
	Use:   "switch",
	Short: "切换代理来源或扩展模式",
	Long: `切换代理来源或扩展模式。

用法:
  gateway switch                         # 查看当前模式
  gateway switch url          # 切换到订阅链接模式
  gateway switch file         # 切换到配置文件模式
  gateway switch file /path   # 切换并更新配置文件路径
  gateway switch extension    # 查看当前扩展模式
  gateway switch extension chains
  gateway switch extension script ./script-demo.js
  gateway switch extension off`,
	Args: cobra.MaximumNArgs(2),
	Run:  runSwitch,
}

var switchExtensionCmd = &cobra.Command{
	Use:     "extension [chains|script|off] [script-path]",
	Aliases: []string{"ext"},
	Short:   "切换扩展模式（chains / script / off）",
	Args:    cobra.MaximumNArgs(2),
	Run:     runSwitchExtension,
}

func init() {
	rootCmd.AddCommand(switchCmd)
	switchCmd.AddCommand(switchExtensionCmd)
}

func runSwitch(cmd *cobra.Command, args []string) {
	cfgPath := resolveConfigPath()
	cfg := loadConfigOrDefault()

	// No args: show current mode
	if len(args) == 0 {
		fmt.Println()
		fmt.Printf("  %s %s\n", color.New(color.Bold).Sprint("代理来源:"), color.CyanString(cfg.Proxy.Source))
		if cfg.Proxy.Source == "url" {
			url := cfg.Proxy.SubscriptionURL
			if len(url) > 50 {
				url = url[:50] + "..."
			}
			fmt.Printf("  %s %s\n", color.New(color.Bold).Sprint("订阅链接:"), url)
		} else {
			fmt.Printf("  %s %s\n", color.New(color.Bold).Sprint("配置文件:"), cfg.Proxy.ConfigFile)
		}
		fmt.Printf("  %s %s\n", color.New(color.Bold).Sprint("扩展模式:"), extensionModeSummary(cfg))
		fmt.Println()
		fmt.Println(color.New(color.Faint).Sprint("  常用切换:"))
		fmt.Println("    gateway switch url")
		fmt.Println("    gateway switch file /path/to/config.yaml")
		fmt.Println("    gateway switch extension chains")
		fmt.Println("    gateway switch extension script ./script-demo.js")
		fmt.Println("    gateway switch extension off")
		fmt.Println()
		return
	}

	target := args[0]
	if target != "url" && target != "file" {
		ui.Error("参数应为 url 或 file")
		os.Exit(1)
	}

	// Switch to url mode
	if target == "url" && cfg.Proxy.SubscriptionURL == "" {
		ui.Error("未配置订阅链接，请先在 gateway.yaml 中设置 proxy.subscription_url")
		os.Exit(1)
	}

	// Switch to file mode
	if target == "file" {
		if len(args) >= 2 {
			validatedPath, count, err := validateSubscriptionFile(args[1])
			if err != nil {
				ui.Error("订阅文件校验失败: %s", err)
				os.Exit(1)
			}
			ui.Info("检测到 %d 个代理节点", count)
			cfg.Proxy.ConfigFile = validatedPath
		}
		if cfg.Proxy.ConfigFile == "" {
			ui.Error("未配置文件路径")
			fmt.Println("  用法: gateway switch file /path/to/config.yaml")
			os.Exit(1)
		}
	}

	if target == "url" {
		count, err := validateSubscriptionURL(cfg.Proxy.SubscriptionURL)
		if err != nil {
			ui.Error("订阅链接校验失败: %s", err)
			os.Exit(1)
		}
		ui.Info("订阅链接校验通过，识别到 %d 个节点", count)
	}

	oldSource := cfg.Proxy.Source
	cfg.Proxy.Source = target

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

		if cfg.Proxy.Source == "file" {
			providerFile := filepath.Join(dDir, "proxy_provider", cfg.Proxy.SubscriptionName+".yaml")
			validatedPath, count, err := validateSubscriptionFile(cfg.Proxy.ConfigFile)
			if err != nil {
				ui.Error("订阅文件校验失败: %s", err)
				os.Exit(1)
			}
			cfg.Proxy.ConfigFile = validatedPath
			if _, err := os.Stat(filepath.Dir(providerFile)); err != nil {
				_ = os.MkdirAll(filepath.Dir(providerFile), 0755)
			}
			if count, err = proxyExtractToProviderFile(cfg.Proxy.ConfigFile, providerFile); err != nil {
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
		ui.Info("如需生效，请重启网关: %s", elevatedCmd("start"))
	}
}

func proxyExtractToProviderFile(inputPath, outputPath string) (int, error) {
	return proxy.ExtractProxies(inputPath, outputPath)
}

func runSwitchExtension(cmd *cobra.Command, args []string) {
	cfgPath := resolveConfigPath()
	cfg := loadConfigOrDefault()

	if len(args) == 0 {
		printExtensionStatus(cfg)
		return
	}

	target := args[0]
	switch target {
	case "chains":
		if cfg.Extension.ResidentialChain == nil {
			ui.Error("未配置 extension.residential_chain")
			fmt.Println("  先运行 gateway chains 进入向导，或手动编辑 gateway.yaml")
			os.Exit(1)
		}
		prev := extensionModeName(cfg.Extension.Mode)
		cfg.Extension.Mode = "chains"
		saveExtensionSwitch(cfg, cfgPath)
		ui.Success("已切换扩展模式: %s → chains", prev)
		if cfg.Extension.ScriptPath != "" {
			ui.Info("script_path 已保留，当前不生效: %s", cfg.Extension.ScriptPath)
		}
	case "script":
		if len(args) >= 2 {
			cfg.Extension.ScriptPath = expandPath(args[1])
		}
		if cfg.Extension.ScriptPath == "" {
			ui.Error("未配置 extension.script_path")
			fmt.Println("  用法: gateway switch extension script /path/to/script.js")
			os.Exit(1)
		}
		if _, err := os.Stat(cfg.Extension.ScriptPath); err != nil {
			ui.Error("脚本文件不存在: %s", cfg.Extension.ScriptPath)
			os.Exit(1)
		}
		prev := extensionModeName(cfg.Extension.Mode)
		cfg.Extension.Mode = "script"
		saveExtensionSwitch(cfg, cfgPath)
		ui.Success("已切换扩展模式: %s → script", prev)
		if cfg.Extension.ResidentialChain != nil {
			ui.Info("residential_chain 配置已保留，当前不生效")
		}
	case "off":
		if cfg.Extension.Mode == "" {
			ui.Info("扩展模式本就未启用")
			return
		}
		prev := cfg.Extension.Mode
		cfg.Extension.Mode = ""
		saveExtensionSwitch(cfg, cfgPath)
		ui.Success("已关闭扩展模式（原模式: %s）", prev)
		if cfg.Extension.ScriptPath != "" {
			ui.Info("script_path 已保留，后续可直接运行 gateway switch extension script 启用")
		}
	default:
		ui.Error("参数应为 chains、script 或 off")
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("  重启网关后生效:")
	fmt.Printf("    %s\n", elevatedCmd("restart"))
}

func saveExtensionSwitch(cfg *config.Config, cfgPath string) {
	if cfgPath == ".secret" {
		cfgPath = "gateway.yaml"
	}
	if err := config.Save(cfg, cfgPath); err != nil {
		ui.Error("保存配置失败: %s", err)
		os.Exit(1)
	}
}

func printExtensionStatus(cfg *config.Config) {
	fmt.Println()
	fmt.Printf("  %s %s\n", color.New(color.Bold).Sprint("当前扩展模式:"), extensionModeSummary(cfg))
	switch cfg.Extension.Mode {
	case "chains":
		if cfg.Extension.ResidentialChain != nil {
			fmt.Printf("  %s %s:%d (%s)\n", color.New(color.Bold).Sprint("住宅代理:"), cfg.Extension.ResidentialChain.ProxyServer, cfg.Extension.ResidentialChain.ProxyPort, cfg.Extension.ResidentialChain.ProxyType)
			fmt.Printf("  %s %s\n", color.New(color.Bold).Sprint("路由模式:"), cfg.Extension.ResidentialChain.Mode)
		}
	case "script":
		fmt.Printf("  %s %s\n", color.New(color.Bold).Sprint("脚本路径:"), cfg.Extension.ScriptPath)
	default:
		if cfg.Extension.ScriptPath != "" {
			fmt.Printf("  %s %s\n", color.New(color.Bold).Sprint("已保留脚本:"), cfg.Extension.ScriptPath)
		}
	}
	fmt.Println()
	fmt.Println(color.New(color.Faint).Sprint("  常用切换:"))
	fmt.Println("    gateway chains")
	fmt.Println("    gateway switch extension chains")
	fmt.Println("    gateway switch extension script ./script-demo.js")
	fmt.Println("    gateway switch extension off")
	fmt.Println()
}

func extensionModeSummary(cfg *config.Config) string {
	switch cfg.Extension.Mode {
	case "chains":
		return color.GreenString("chains（内置链式代理）")
	case "script":
		if cfg.Extension.ScriptPath != "" {
			return color.GreenString("script（%s）", cfg.Extension.ScriptPath)
		}
		return color.YellowString("script（缺少 script_path）")
	default:
		return color.New(color.Faint).Sprint("未启用")
	}
}

func extensionModeName(mode string) string {
	if mode == "" {
		return "off"
	}
	return mode
}

func expandPath(path string) string {
	return expandUserPath(path)
}
