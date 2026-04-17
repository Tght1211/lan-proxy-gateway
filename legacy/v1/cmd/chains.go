package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

var chainsCmd = &cobra.Command{
	Use:   "chains",
	Short: "配置链式代理（住宅 IP + 机场节点，获得纯净出口）",
	Long: `链式代理模式：设备 → 机场节点 → 住宅代理 → 目标

流量通过机场节点连接住宅/ISP 代理，以纯净住宅 IP 出口。
适用场景：Claude / ChatGPT / Cursor 注册及使用防风控。

子命令:
  chains          交互式配置并切换到 chains 模式
  chains disable  关闭链式代理（extension.mode 置空）
  chains status   查看当前链式代理配置

快速切换:
  gateway switch extension chains
  gateway switch extension script ./script-demo.js`,
	Run: runChainsSetup,
}

var chainsDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "关闭链式代理",
	Run:   runChainsDisable,
}

var chainsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看当前链式代理配置",
	Run:   runChainsStatus,
}

func init() {
	rootCmd.AddCommand(chainsCmd)
	chainsCmd.AddCommand(chainsDisableCmd)
	chainsCmd.AddCommand(chainsStatusCmd)
}

func runChainsSetup(cmd *cobra.Command, args []string) {
	ui.ShowLogo()
	color.New(color.Bold).Println("链式代理配置向导")
	fmt.Println()
	color.New(color.Faint).Println("  流量链路: 设备 → 机场节点 → 住宅代理 → 目标网站")
	color.New(color.Faint).Println("  效果: 以纯净住宅 IP 出口，避免 AI 服务账号风控")
	ui.Separator()

	cfgPath := resolveConfigPath()
	cfg := loadConfigOrDefault()

	// Show current state
	fmt.Println()
	switch cfg.Extension.Mode {
	case "chains":
		color.New(color.FgYellow).Println("  当前模式: chains（重新配置将覆盖现有参数）")
	case "script":
		color.New(color.FgYellow).Printf("  当前模式: script (%s)\n", cfg.Extension.ScriptPath)
		color.New(color.FgYellow).Println("  继续将切换到 chains 模式，script_path 保留但不再生效")
	default:
		color.New(color.Faint).Println("  当前模式: 未启用")
	}
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	chain := &config.ResidentialChain{
		ProxyType:    "socks5",
		AirportGroup: "Auto",
		Mode:         "rule",
	}
	if cfg.Extension.ResidentialChain != nil {
		chain = cfg.Extension.ResidentialChain
	}

	color.New(color.Bold).Println("● 住宅代理信息")
	fmt.Println()

	chain.ProxyServer = prompt(reader, "服务器地址 (IP 或域名)", chain.ProxyServer, true)
	chain.ProxyPort = promptInt(reader, "端口", chain.ProxyPort, 443)

	chain.ProxyType = promptChoice(reader, "协议类型", chain.ProxyType, "socks5", []string{"socks5", "http"})

	fmt.Println()
	color.New(color.Faint).Println("  认证信息（无需认证直接回车跳过）")
	chain.ProxyUsername = prompt(reader, "用户名", chain.ProxyUsername, false)
	if chain.ProxyUsername != "" {
		chain.ProxyPassword = promptPassword(reader, "密码", chain.ProxyPassword)
	} else {
		chain.ProxyPassword = ""
	}

	fmt.Println()
	color.New(color.Bold).Println("● 机场代理组")
	fmt.Println()
	color.New(color.Faint).Println("  填入机场订阅中的延迟测速组名称（通过该组出口连接住宅代理）")
	color.New(color.Faint).Println("  不确定？打开管理面板 http://localhost:9090/ui 查看代理组列表")
	color.New(color.Faint).Println("  常见名称: Auto、自动选择、⚡️最低延迟")
	fmt.Println()
	chain.AirportGroup = prompt(reader, "代理组名称", chain.AirportGroup, false)
	if chain.AirportGroup == "" {
		chain.AirportGroup = "Auto"
	}

	fmt.Println()
	color.New(color.Bold).Println("● 路由模式")
	fmt.Println()
	color.New(color.Faint).Println("  rule   — 仅 AI 服务（Claude / ChatGPT / Cursor）走住宅代理")
	color.New(color.Faint).Println("  global — 所有流量走住宅代理（适合注册账号等全程需要干净 IP 的场景）")
	fmt.Println()
	chain.Mode = promptChoice(reader, "路由模式", chain.Mode, "rule", []string{"rule", "global"})

	cfg.Extension.ResidentialChain = chain
	cfg.Extension.Mode = "chains"

	if err := config.Save(cfg, cfgPath); err != nil {
		ui.Error("保存配置失败: %v", err)
		os.Exit(1)
	}

	fmt.Println()
	ui.Separator()
	ui.Success("已切换到 chains 模式")
	ui.Separator()
	fmt.Println()
	fmt.Printf("  %-16s %s\n", "路由模式:", chain.Mode)
	fmt.Printf("  %-16s %s:%d (%s)\n", "住宅代理:", chain.ProxyServer, chain.ProxyPort, chain.ProxyType)
	if chain.ProxyUsername != "" {
		fmt.Printf("  %-16s %s\n", "认证用户:", chain.ProxyUsername)
	}
	fmt.Printf("  %-16s %s\n", "机场出口组:", chain.AirportGroup)
	fmt.Println()
	fmt.Printf("  %s\n", color.New(color.Faint).Sprint("重启网关后生效:"))
	fmt.Println()
	fmt.Printf("    %s\n", elevatedCmd("restart"))
	fmt.Println()
}

func runChainsDisable(cmd *cobra.Command, args []string) {
	cfgPath := resolveConfigPath()
	cfg := loadConfigOrDefault()

	if cfg.Extension.Mode == "" {
		ui.Info("扩展模式本就未启用")
		return
	}

	prev := cfg.Extension.Mode
	cfg.Extension.Mode = ""
	if err := config.Save(cfg, cfgPath); err != nil {
		ui.Error("保存配置失败: %v", err)
		os.Exit(1)
	}
	ui.Success("已关闭 %s 模式，重启后生效: %s", prev, elevatedCmd("restart"))
}

func runChainsStatus(cmd *cobra.Command, args []string) {
	cfg := loadConfigOrDefault()

	ui.Separator()
	fmt.Println()
	switch cfg.Extension.Mode {
	case "chains":
		color.New(color.FgGreen, color.Bold).Println("  扩展模式: chains（内置链式代理）")
		fmt.Println()
		if cfg.Extension.ResidentialChain != nil {
			c := cfg.Extension.ResidentialChain
			fmt.Printf("  %-14s %s\n", "路由模式:", c.Mode)
			fmt.Printf("  %-14s %s:%d (%s)\n", "住宅代理:", c.ProxyServer, c.ProxyPort, c.ProxyType)
			if c.ProxyUsername != "" {
				fmt.Printf("  %-14s %s\n", "认证用户:", c.ProxyUsername)
			}
			fmt.Printf("  %-14s %s\n", "机场出口组:", c.AirportGroup)
			if len(c.ExtraDirectRules) > 0 {
				fmt.Printf("  %-14s %d 条\n", "额外直连:", len(c.ExtraDirectRules))
			}
			if len(c.ExtraProxyRules) > 0 {
				fmt.Printf("  %-14s %d 条\n", "额外代理:", len(c.ExtraProxyRules))
			}
		}
	case "script":
		color.New(color.FgGreen, color.Bold).Println("  扩展模式: script（扩展脚本）")
		fmt.Println()
		fmt.Printf("  %-14s %s\n", "脚本路径:", cfg.Extension.ScriptPath)
	default:
		color.New(color.Faint).Println("  扩展模式: 未启用")
		fmt.Println()
		fmt.Println("  运行 gateway chains 配置链式代理")
		if cfg.Extension.ScriptPath != "" {
			fmt.Println("  或运行 gateway switch extension script 启用脚本模式")
		}
	}
	fmt.Println()
	ui.Separator()
}

// promptChoice prompts with allowed values, returns defaultVal on empty input.
func promptChoice(reader *bufio.Reader, label, currentVal, fallback string, choices []string) string {
	shown := currentVal
	if shown == "" {
		shown = fallback
	}
	for {
		fmt.Printf("%s [%s] (%s): ", label, strings.Join(choices, "/"), shown)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			return shown
		}
		for _, c := range choices {
			if input == c {
				return input
			}
		}
		ui.Warn("请输入 %s 之一", strings.Join(choices, " / "))
	}
}

// promptPassword prompts for a password, showing masked hint if already set.
func promptPassword(reader *bufio.Reader, label, currentVal string) string {
	hint := "留空跳过"
	if currentVal != "" {
		hint = "已设置，回车保留"
	}
	fmt.Printf("%s (%s): ", label, hint)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return currentVal
	}
	return input
}

// prompt shows a prompt with optional default value, returns trimmed input.
func prompt(reader *bufio.Reader, label, defaultVal string, required bool) string {
	for {
		fmt.Printf("%s (%s): ", label, defaultHint(defaultVal, ""))
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			if defaultVal != "" {
				return defaultVal
			}
			if required {
				ui.Warn("此项不能为空，请重新输入")
				continue
			}
		}
		return input
	}
}

// promptInt prompts for an integer with a fallback default.
func promptInt(reader *bufio.Reader, label string, currentVal, fallback int) int {
	shown := currentVal
	if shown == 0 {
		shown = fallback
	}
	fmt.Printf("%s (%d): ", label, shown)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		if currentVal != 0 {
			return currentVal
		}
		return fallback
	}
	n, err := strconv.Atoi(input)
	if err != nil || n <= 0 || n > 65535 {
		ui.Warn("无效端口，使用默认值 %d", shown)
		return shown
	}
	return n
}

func defaultHint(val, fallback string) string {
	if val != "" {
		return val
	}
	if fallback != "" {
		return fallback
	}
	return "留空跳过"
}
