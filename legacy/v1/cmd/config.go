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
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "交互式配置中心（代理来源 / 局域网共享 / 规则 / 扩展）",
	Long: `统一的交互式配置中心。

适合不想直接编辑 gateway.yaml 的用户：
  gateway config        # 打开交互菜单
  gateway config show   # 查看当前配置摘要`,
	Run: runConfigMenu,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "查看当前配置摘要",
	Run: func(cmd *cobra.Command, args []string) {
		printConfigSummary(loadConfigOrDefault())
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
}

func runConfigMenu(cmd *cobra.Command, args []string) {
	cfgPath := resolveConfigPath()
	cfg := loadConfigOrDefault()
	reader := bufio.NewReader(os.Stdin)

	for {
		ui.ShowLogo()
		color.New(color.Bold).Println("配置中心")
		fmt.Println()
		color.New(color.Faint).Println("  这里可以配置代理来源、局域网共享、规则开关和扩展模式")
		ui.Separator()
		printCompactConfigSummary(cfg)
		fmt.Println("  1) 代理来源")
		fmt.Println("  2) 局域网共享 / TUN / 端口")
		fmt.Println("  3) 规则开关与自定义规则")
		fmt.Println("  4) 扩展模式（chains / script / off）")
		fmt.Println("  5) 查看完整配置摘要")
		fmt.Println("  0) 退出")
		fmt.Println()
		fmt.Print("请选择 [0-5]: ")

		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)
		fmt.Println()

		switch choice {
		case "1":
			configureProxyMenu(reader, cfg)
			saveConfigOrExit(cfg, cfgPath)
		case "2":
			configureRuntimeMenu(reader, cfg)
			saveConfigOrExit(cfg, cfgPath)
		case "3":
			configureRulesMenu(reader, cfg)
			saveConfigOrExit(cfg, cfgPath)
		case "4":
			configureExtensionMenu(reader, cfg)
			cfg = loadConfigOrDefault()
			cfgPath = resolveConfigPath()
		case "5":
			printConfigSummary(cfg)
			waitEnter(reader)
		case "0":
			return
		default:
			ui.Warn("请输入 0-5 之间的选项")
			waitEnter(reader)
		}
	}
}

func configureProxyMenu(reader *bufio.Reader, cfg *config.Config) {
	ui.Separator()
	color.New(color.Bold).Println("代理来源")
	fmt.Println()
	color.New(color.Faint).Println("  决定网关从哪里拿节点：")
	color.New(color.Faint).Println("    url   — 机场提供的订阅链接（推荐）")
	color.New(color.Faint).Println("    file  — 本地 Clash/mihomo YAML 配置文件")
	color.New(color.Faint).Println("    proxy — 直接填写 SOCKS5/HTTP 代理服务器（无需订阅）")
	fmt.Println()

	cfg.Proxy.Source = promptChoice(reader, "代理来源", cfg.Proxy.Source, "url", []string{"url", "file", "proxy"})
	switch cfg.Proxy.Source {
	case "url":
		cfg.Proxy.SubscriptionURL = prompt(reader, "订阅链接", cfg.Proxy.SubscriptionURL, true)
		cfg.Proxy.SubscriptionName = prompt(reader, "订阅名称", cfg.Proxy.SubscriptionName, true)
	case "file":
		cfg.Proxy.ConfigFile = prompt(reader, "本地配置文件路径", cfg.Proxy.ConfigFile, true)
		cfg.Proxy.SubscriptionName = prompt(reader, "订阅名称", cfg.Proxy.SubscriptionName, true)
	case "proxy":
		configureDirectProxyMenu(reader, cfg)
		return
	}

	fmt.Println()
	ui.Success("代理来源配置已更新")
	waitEnter(reader)
}

func configureDirectProxyMenu(reader *bufio.Reader, cfg *config.Config) {
	fmt.Println()
	color.New(color.Bold).Println("● 直接代理服务器配置")
	fmt.Println()

	if cfg.Proxy.DirectProxy == nil {
		cfg.Proxy.DirectProxy = &config.DirectProxyConfig{
			Name: "MyProxy",
			Type: "socks5",
		}
	}
	dp := cfg.Proxy.DirectProxy

	dp.Server = prompt(reader, "代理服务器地址", dp.Server, true)
	portStr := prompt(reader, "端口", fmt.Sprintf("%d", dp.Port), true)
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	if port > 0 {
		dp.Port = port
	}
	dp.Type = promptChoice(reader, "协议类型", dp.Type, "socks5", []string{"socks5", "http"})
	dp.Username = prompt(reader, "用户名（无认证留空）", dp.Username, false)
	if dp.Username != "" {
		dp.Password = prompt(reader, "密码", dp.Password, false)
	}
	dp.Name = prompt(reader, "节点名称", dp.Name, true)
	if dp.Name == "" {
		dp.Name = "MyProxy"
	}
	cfg.Proxy.SubscriptionName = "direct"

	fmt.Println()
	ui.Success("直接代理配置已更新")
	waitEnter(reader)
}

func configureRuntimeMenu(reader *bufio.Reader, cfg *config.Config) {
	ui.Separator()
	color.New(color.Bold).Println("局域网共享 / TUN / 端口")
	fmt.Println()
	color.New(color.Faint).Println("  局域网共享的核心在 TUN：Switch / PS5 / Apple TV 等设备要走透明代理，通常需要开启")
	color.New(color.Faint).Println("  如需让当前这台电脑本机流量保持直连、只给局域网其他设备提供科学上网，可开启“本机绕过代理”")
	fmt.Println()

	tunChoice := promptChoice(reader, "TUN 模式", onOff(cfg.Runtime.Tun.Enabled), "off", []string{"on", "off"})
	cfg.Runtime.Tun.Enabled = tunChoice == "on"
	if cfg.Runtime.Tun.Enabled {
		bypassChoice := promptChoice(reader, "本机绕过代理", onOff(cfg.Runtime.Tun.BypassLocal), "off", []string{"off", "on"})
		cfg.Runtime.Tun.BypassLocal = bypassChoice == "on"
	} else {
		cfg.Runtime.Tun.BypassLocal = false
	}

	fmt.Println()
	color.New(color.Bold).Println("● 运行端口")
	fmt.Println()
	cfg.Runtime.Ports.Mixed = promptInt(reader, "Mixed 端口", cfg.Runtime.Ports.Mixed, 7890)
	cfg.Runtime.Ports.Redir = promptInt(reader, "Redir 端口", cfg.Runtime.Ports.Redir, 7892)
	cfg.Runtime.Ports.API = promptInt(reader, "API 端口", cfg.Runtime.Ports.API, 9090)
	cfg.Runtime.Ports.DNS = promptInt(reader, "DNS 端口", cfg.Runtime.Ports.DNS, 53)
	cfg.Runtime.APISecret = prompt(reader, "API Secret（可留空）", cfg.Runtime.APISecret, false)

	fmt.Println()
	ui.Success("运行参数配置已更新")
	waitEnter(reader)
}

func configureRulesMenu(reader *bufio.Reader, cfg *config.Config) {
	for {
		ui.Separator()
		color.New(color.Bold).Println("规则开关与自定义规则")
		fmt.Println()
		fmt.Printf("  1) 局域网直连           %s\n", enabledText(cfg.Rules.LanDirectEnabled()))
		fmt.Printf("  2) 国内服务直连         %s\n", enabledText(cfg.Rules.ChinaDirectEnabled()))
		fmt.Printf("  3) Apple 分流规则       %s\n", enabledText(cfg.Rules.AppleRulesEnabled()))
		fmt.Printf("  4) Nintendo 走代理      %s\n", enabledText(cfg.Rules.NintendoProxyEnabled()))
		fmt.Printf("  5) 国外常见网站走代理   %s\n", enabledText(cfg.Rules.GlobalProxyEnabled()))
		fmt.Printf("  6) 明显广告拦截         %s\n", enabledText(cfg.Rules.AdsRejectEnabled()))
		fmt.Printf("  7) 编辑额外直连规则     %d 条\n", len(cfg.Rules.ExtraDirectRules))
		fmt.Printf("  8) 编辑额外代理规则     %d 条\n", len(cfg.Rules.ExtraProxyRules))
		fmt.Printf("  9) 编辑额外拦截规则     %d 条\n", len(cfg.Rules.ExtraRejectRules))
		fmt.Println("  0) 返回")
		fmt.Println()
		fmt.Print("请选择 [0-9]: ")

		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)
		fmt.Println()

		switch choice {
		case "1":
			cfg.Rules.LanDirect = boolPtr(!cfg.Rules.LanDirectEnabled())
		case "2":
			cfg.Rules.ChinaDirect = boolPtr(!cfg.Rules.ChinaDirectEnabled())
		case "3":
			cfg.Rules.AppleRules = boolPtr(!cfg.Rules.AppleRulesEnabled())
		case "4":
			cfg.Rules.NintendoProxy = boolPtr(!cfg.Rules.NintendoProxyEnabled())
		case "5":
			cfg.Rules.GlobalProxy = boolPtr(!cfg.Rules.GlobalProxyEnabled())
		case "6":
			cfg.Rules.AdsReject = boolPtr(!cfg.Rules.AdsRejectEnabled())
		case "7":
			cfg.Rules.ExtraDirectRules = promptRuleList(reader, "额外直连规则", cfg.Rules.ExtraDirectRules)
		case "8":
			cfg.Rules.ExtraProxyRules = promptRuleList(reader, "额外代理规则", cfg.Rules.ExtraProxyRules)
		case "9":
			cfg.Rules.ExtraRejectRules = promptRuleList(reader, "额外拦截规则", cfg.Rules.ExtraRejectRules)
		case "0":
			return
		default:
			ui.Warn("请输入 0-9 之间的选项")
			waitEnter(reader)
		}
	}
}

func configureExtensionMenu(reader *bufio.Reader, cfg *config.Config) {
	ui.Separator()
	color.New(color.Bold).Println("扩展模式")
	fmt.Println()
	color.New(color.Faint).Println("  chains 适合 Claude / ChatGPT / Codex / Cursor 的住宅出口场景")
	color.New(color.Faint).Println("  script 适合你已有 Clash Verge Rev 脚本，或要更复杂的自定义逻辑")
	fmt.Println()

	choice := promptChoice(reader, "扩展模式", extensionModeName(cfg.Extension.Mode), "off", []string{"off", "chains", "script"})
	switch choice {
	case "off":
		cfg.Extension.Mode = ""
		saveConfigOrExit(cfg, resolveConfigPath())
		ui.Success("扩展模式已关闭")
		waitEnter(reader)
	case "script":
		cfg.Extension.ScriptPath = prompt(reader, "脚本路径", cfg.Extension.ScriptPath, true)
		cfg.Extension.Mode = "script"
		saveConfigOrExit(cfg, resolveConfigPath())
		ui.Success("已切换到 script 模式")
		waitEnter(reader)
	case "chains":
		saveConfigOrExit(cfg, resolveConfigPath())
		fmt.Println()
		color.New(color.Faint).Println("  进入链式代理向导...")
		fmt.Println()
		runChainsSetup(nil, nil)
	}
}

func saveConfigOrExit(cfg *config.Config, cfgPath string) {
	if cfgPath == ".secret" {
		cfgPath = "gateway.yaml"
	}
	if err := config.Save(cfg, cfgPath); err != nil {
		ui.Error("保存配置失败: %s", err)
		os.Exit(1)
	}
}

func printCompactConfigSummary(cfg *config.Config) {
	color.New(color.Bold).Println("  当前配置")
	fmt.Println()
	fmt.Printf("  配置文件: %s\n", displayConfigPath())
	fmt.Printf("  代理来源: %s\n", proxySourceLabel(cfg))
	fmt.Printf("  TUN: %s\n", onOff(cfg.Runtime.Tun.Enabled))
	if cfg.Runtime.Tun.Enabled {
		fmt.Printf("  本机绕过: %s\n", onOff(cfg.Runtime.Tun.BypassLocal))
	}
	fmt.Printf("  扩展模式: %s\n", extensionModeName(cfg.Extension.Mode))
	fmt.Printf("  国内直连: %s\n", enabledText(cfg.Rules.ChinaDirectEnabled()))
	fmt.Printf("  广告拦截: %s\n", enabledText(cfg.Rules.AdsRejectEnabled()))
	fmt.Println()
}

func proxySourceLabel(cfg *config.Config) string {
	switch cfg.Proxy.Source {
	case "url":
		return fmt.Sprintf("订阅链接 (%s)", cfg.Proxy.SubscriptionName)
	case "file":
		return fmt.Sprintf("本地文件 (%s)", cfg.Proxy.SubscriptionName)
	case "proxy":
		if dp := cfg.Proxy.DirectProxy; dp != nil && dp.Server != "" {
			return fmt.Sprintf("直接代理 (%s %s:%d)", dp.Type, dp.Server, dp.Port)
		}
		return "直接代理 (未配置)"
	}
	return cfg.Proxy.Source
}

func printConfigSummary(cfg *config.Config) {
	ui.Separator()
	color.New(color.Bold).Println("  当前配置摘要")
	ui.Separator()
	fmt.Println()

	color.New(color.Bold).Println("  配置来源")
	fmt.Println()
	fmt.Printf("  配置文件: %s\n", displayConfigPath())
	fmt.Printf("  代理来源: %s\n", proxySourceLabel(cfg))
	switch cfg.Proxy.Source {
	case "url":
		fmt.Printf("  订阅名称: %s\n", cfg.Proxy.SubscriptionName)
		fmt.Printf("  订阅链接: %s\n", shortText(cfg.Proxy.SubscriptionURL, 72))
	case "file":
		fmt.Printf("  订阅名称: %s\n", cfg.Proxy.SubscriptionName)
		fmt.Printf("  本地配置: %s\n", cfg.Proxy.ConfigFile)
	case "proxy":
		if dp := cfg.Proxy.DirectProxy; dp != nil {
			fmt.Printf("  节点名称: %s\n", dp.Name)
			fmt.Printf("  服务器:   %s\n", dp.Server)
			fmt.Printf("  端口:     %d\n", dp.Port)
			fmt.Printf("  协议:     %s\n", dp.Type)
			if dp.Username != "" {
				fmt.Printf("  用户名:   %s\n", dp.Username)
				fmt.Printf("  密码:     %s\n", maskedSecret(dp.Password))
			}
		}
	}
	fmt.Println()

	color.New(color.Bold).Println("  运行模式")
	fmt.Println()
	fmt.Printf("  TUN: %s\n", onOff(cfg.Runtime.Tun.Enabled))
	fmt.Printf("  本机绕过代理: %s\n", onOff(cfg.Runtime.Tun.BypassLocal))
	fmt.Printf("  端口: mixed %d | redir %d | api %d | dns %d\n", cfg.Runtime.Ports.Mixed, cfg.Runtime.Ports.Redir, cfg.Runtime.Ports.API, cfg.Runtime.Ports.DNS)
	fmt.Println()

	color.New(color.Bold).Println("  扩展模式")
	fmt.Println()
	fmt.Printf("  模式: %s\n", extensionModeName(cfg.Extension.Mode))
	if cfg.Extension.Mode == "script" {
		fmt.Printf("  脚本路径: %s\n", cfg.Extension.ScriptPath)
	}
	if cfg.Extension.Mode == "chains" && cfg.Extension.ResidentialChain != nil {
		fmt.Printf("  链式模式: %s\n", cfg.Extension.ResidentialChain.Mode)
		fmt.Printf("  机场组: %s\n", cfg.Extension.ResidentialChain.AirportGroup)
	}
	fmt.Println()

	color.New(color.Bold).Println("  规则开关")
	fmt.Println()
	fmt.Printf("  局域网直连: %s\n", enabledText(cfg.Rules.LanDirectEnabled()))
	fmt.Printf("  国内直连: %s\n", enabledText(cfg.Rules.ChinaDirectEnabled()))
	fmt.Printf("  Apple 规则: %s\n", enabledText(cfg.Rules.AppleRulesEnabled()))
	fmt.Printf("  Nintendo 代理: %s\n", enabledText(cfg.Rules.NintendoProxyEnabled()))
	fmt.Printf("  国外代理: %s\n", enabledText(cfg.Rules.GlobalProxyEnabled()))
	fmt.Printf("  广告拦截: %s\n", enabledText(cfg.Rules.AdsRejectEnabled()))
	fmt.Printf("  自定义规则: 直连 %d | 代理 %d | 拦截 %d\n", len(cfg.Rules.ExtraDirectRules), len(cfg.Rules.ExtraProxyRules), len(cfg.Rules.ExtraRejectRules))
	fmt.Println()
	ui.Separator()
	fmt.Println()
}

func promptRuleList(reader *bufio.Reader, label string, current []string) []string {
	fmt.Println(label + "：")
	if len(current) == 0 {
		color.New(color.Faint).Println("  当前为空")
	} else {
		for _, item := range current {
			fmt.Println("  - " + item)
		}
	}
	fmt.Println()
	color.New(color.Faint).Println("  逐行输入，空行结束；直接回车保留当前；输入 - 清空")
	fmt.Println()

	var next []string
	for {
		fmt.Print("> ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		if line == "" {
			if len(next) == 0 {
				return current
			}
			return next
		}
		if line == "-" {
			return nil
		}
		next = append(next, line)
	}
}

func enabledText(enabled bool) string {
	if enabled {
		return color.GreenString("on")
	}
	return color.YellowString("off")
}

func onOff(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

func shortText(text string, max int) string {
	if len(text) <= max {
		return text
	}
	if max < 4 {
		return text[:max]
	}
	return text[:max-3] + "..."
}

func waitEnter(reader *bufio.Reader) {
	fmt.Print("回车继续...")
	_, _ = reader.ReadString('\n')
	fmt.Println()
}

func displayConfigPath() string {
	path := resolveConfigPath()
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func boolPtr(v bool) *bool {
	return &v
}
