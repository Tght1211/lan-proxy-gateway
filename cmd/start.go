package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/egress"
	"github.com/tght/lan-proxy-gateway/internal/mihomo"
	"github.com/tght/lan-proxy-gateway/internal/platform"
	"github.com/tght/lan-proxy-gateway/internal/proxy"
	tmpl "github.com/tght/lan-proxy-gateway/internal/template"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "启动代理网关",
	Run:   runStart,
}

var startSimple bool
var startTUI bool

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().BoolVar(&startSimple, "simple", false, "使用纯命令模式：默认模式，兼容性更好的命令交互")
	startCmd.Flags().BoolVar(&startTUI, "tui", false, "使用运行中 TUI 工作台")
}

func runStart(cmd *cobra.Command, args []string) {
	simpleMode, err := resolveConsoleSimpleMode(cmd, true, startSimple, startTUI)
	if err != nil {
		ui.Error("%s", err)
		os.Exit(1)
	}

	runStartWithMode(simpleMode, cmd, args)
}

func runStartWithMode(simpleMode bool, cmd *cobra.Command, args []string) {
	checkRoot()

	ui.ShowLogo()
	p := platform.New()

	// Step 1: Prepare
	ui.Step(1, 5, "准备环境...")

	cfg := loadConfigRequired()
	dDir := ensureDataDir()

	binary, err := p.FindBinary()
	if err != nil {
		ui.Error("未找到 mihomo，请先运行 gateway install")
		os.Exit(1)
	}
	ui.Success("mihomo: %s", binary)

	// Stop old process if running
	if running, _, _ := p.IsRunning(); running {
		ui.Warn("检测到 mihomo 正在运行，先停止...")
		p.StopProcess()
	}

	// Step 2: Generate config
	ui.Step(2, 5, "生成配置文件...")

	iface, _ := p.DetectDefaultInterface()
	ip, _ := p.DetectInterfaceIP(iface)

	// 代理来源前置处理
	switch cfg.Proxy.Source {
	case "file":
		if cfg.Proxy.ConfigFile == "" {
			ui.Error("配置文件路径未设置，请运行 gateway config 配置")
			os.Exit(1)
		}
		providerFile := filepath.Join(dDir, "proxy_provider", cfg.Proxy.SubscriptionName+".yaml")
		count, err := proxy.ExtractProxies(cfg.Proxy.ConfigFile, providerFile)
		if err != nil {
			ui.Error("提取代理节点失败: %s", err)
			os.Exit(1)
		}
		ui.Success("已从配置文件中提取 %d 个代理节点", count)
	case "proxy":
		dp := cfg.Proxy.DirectProxy
		if dp == nil || strings.TrimSpace(dp.Server) == "" || dp.Port <= 0 {
			ui.Error("直接代理服务器未完整配置，请运行 gateway config 填写服务器地址和端口")
			os.Exit(1)
		}
		ui.Success("直接代理模式: %s %s:%d", dp.Type, dp.Server, dp.Port)
	case "url":
		if strings.TrimSpace(cfg.Proxy.SubscriptionURL) == "" {
			ui.Error("订阅链接未设置，请运行 gateway config 配置")
			os.Exit(1)
		}
	}

	configPath := filepath.Join(dDir, "config.yaml")
	if err := tmpl.RenderTemplate(cfg, iface, ip, configPath); err != nil {
		ui.Error("配置文件生成失败: %s", err)
		os.Exit(1)
	}
	ui.Success("配置文件已生成: %s", configPath)

	// Step 3: Enable IP forwarding
	ui.Step(3, 5, "开启 IP 转发...")
	if err := p.EnableIPForwarding(); err != nil {
		ui.Error("开启 IP 转发失败: %s", err)
		os.Exit(1)
	}
	ui.Success("IP 转发已开启")

	p.DisableFirewallInterference()

	// Step 4: Start mihomo
	if cfg.Runtime.Tun.Enabled {
		ui.Step(4, 5, "启动 mihomo (TUN 模式)...")
	} else {
		ui.Step(4, 5, "启动 mihomo...")
	}

	logFile := defaultLogFile()
	pid, err := p.StartProcess(binary, dDir, logFile)
	if err != nil {
		ui.Error("mihomo 启动失败: %s", err)
		os.Exit(1)
	}

	time.Sleep(5 * time.Second)

	// Verify process is still alive
	if running, _, _ := p.IsRunning(); !running {
		ui.Error("mihomo 启动失败！")
		fmt.Println()
		fmt.Println("最后 20 行日志:")
		printLastLines(logFile, 20)
		os.Exit(1)
	}
	ui.Success("mihomo 启动成功 (PID: %d)", pid)

	// Step 5: Verify TUN (only when TUN mode is enabled)
	if cfg.Runtime.Tun.Enabled {
		ui.Step(5, 5, "验证 TUN 接口...")
		tunIf, err := p.DetectTUNInterface()
		if err == nil && tunIf != "" {
			ui.Success("TUN 接口已创建: %s", tunIf)
		} else {
			ui.Warn("TUN 接口未检测到（可能还在创建中）")
		}
	} else {
		ui.Step(5, 5, "验证服务...")
		ui.Success("代理服务运行正常（规则模式，无 TUN）")
	}

	if isInteractiveTerminal() {
		runInteractiveConsoleLoop(simpleMode, logFile, ip, iface, dDir, func() {
			runStartWithMode(simpleMode, cmd, args)
		})
		return
	}

	// 非交互模式（systemd / 脚本调用）：只打印基础信息后退出，不做任何网络请求
	// 网络探测（egress/update）放在交互模式里做，避免 systemd 启动超时
	printDaemonStartSummary(cfg, ip, iface)
}

func runInteractiveConsoleLoop(simple bool, logFile, ip, iface, dDir string, restartFn func()) {
	modeSimple := simple

	for {
		var action consoleAction
		if modeSimple {
			action = runSimpleRuntimeConsole(logFile, ip, iface, dDir)
		} else {
			action = runRuntimeConsole(logFile, ip, iface, dDir)
		}

		switch action {
		case consoleActionOpenConfig:
			runConfigMenu(nil, nil)
		case consoleActionOpenChainsSetup:
			runChainsSetup(nil, nil)
		case consoleActionOpenTUI:
			modeSimple = false
		case consoleActionStop:
			runStop(nil, nil)
			return
		case consoleActionRestart:
			runStop(nil, nil)
			restartFn()
			return
		default:
			return
		}
	}
}

// printDaemonStartSummary 用于非交互（systemd/脚本）启动时的输出
// 不做任何网络请求，保证快速退出，避免 systemd TimeoutStartSec 超时
func printDaemonStartSummary(cfg *config.Config, ip, iface string) {
	ui.Separator()
	color.New(color.FgGreen, color.Bold).Println("  Gateway Started")
	ui.Separator()
	fmt.Println()
	fmt.Printf("  共享入口: 网关 / DNS -> %s\n", color.CyanString(ip))
	fmt.Printf("  运行模式: %s\n", compactModeSummary(cfg))
	fmt.Printf("  API 面板: http://%s:%d/ui\n", ip, cfg.Runtime.Ports.API)
	if iface != "" {
		fmt.Printf("  网络接口: %s\n", iface)
	}
	fmt.Println()
}

func printCompactStartSummary(cfg *config.Config, dDir, ip, iface string) {
	apiURL := mihomo.FormatAPIURL("127.0.0.1", cfg.Runtime.Ports.API)
	client := mihomo.NewClient(apiURL, cfg.Runtime.APISecret)
	report := egress.Collect(cfg, dDir, client)
	updateNotice := loadUpdateNotice()

	ui.Separator()
	color.New(color.FgGreen, color.Bold).Println("  Gateway Ready")
	ui.Separator()
	fmt.Println()
	fmt.Printf("  共享入口: 网关 / DNS -> %s\n", color.CyanString(ip))
	fmt.Printf("  运行模式: %s\n", compactModeSummary(cfg))
	fmt.Printf("  出口摘要: %s\n", compactEgressSummary(cfg, report))
	fmt.Printf("  面板入口: http://%s:%d/ui\n", ip, cfg.Runtime.Ports.API)
	fmt.Printf("  配置文件: %s\n", displayConfigPath())
	fmt.Printf("  查看详情: gateway status\n")
	if updateNotice != nil {
		fmt.Printf("  新版可用: %s -> %s\n", color.YellowString(updateNotice.Latest), elevatedCmd("update"))
	}
	if iface != "" {
		fmt.Printf("  网络接口: %s\n", iface)
	}
	fmt.Println()
}

func runSimpleRuntimeConsole(logFile, ip, iface, dDir string) consoleAction {
	reader := bufio.NewReader(os.Stdin)
	workspace := simpleWorkspaceNone
	printCompactStartSummary(loadConfigOrDefault(), dDir, ip, iface)
	fmt.Println("  已进入纯命令模式。输入 help 查看命令，输入 exit 退出。")
	fmt.Println()

	for {
		cfg := loadConfigOrDefault()
		fmt.Print("gateway> ")

		input, _ := reader.ReadString('\n')
		rawChoice := strings.TrimSpace(input)
		choice := strings.ToLower(rawChoice)
		fmt.Println()

		if rawChoice == "" {
			continue
		}
		if handleSimpleHelpCommand(rawChoice) {
			continue
		}
		if action, handled := handleSimpleConfigCommand(reader, &workspace, rawChoice); handled {
			if action != consoleActionNone {
				return action
			}
			continue
		}

		switch choice {
		case "status":
			runStatus(nil, nil)
		case "summary":
			printConfigSummary(loadConfigOrDefault())
		case "config":
			return consoleActionOpenConfig
		case "chains":
			if cfg.Extension.Mode == "chains" {
				runChainsStatus(nil, nil)
			} else {
				printExtensionStatus(cfg)
			}
		case "chains setup":
			return consoleActionOpenChainsSetup
		case "nodes", "node", "groups":
			runSimpleGroupChooser(reader, cfg)
		case "device", "devices":
			printDeviceSetupPanel(ip, cfg.Runtime.Ports.API)
		case "logs", "log":
			ui.Separator()
			color.New(color.Bold).Println("  最近日志")
			ui.Separator()
			printLastLines(logFile, 30)
			fmt.Println()
			fmt.Printf("  实时查看: %s\n", followLogCommand(logFile))
			fmt.Println()
		case "guide":
			printStartGuide(cfg, logFile)
		case "update":
			if notice := loadUpdateNotice(); notice != nil {
				for _, line := range renderUpdateNoticeLines(notice) {
					fmt.Printf("  %s\n", line)
				}
			} else {
				fmt.Println("  当前已经是最新版本，或本次未检测到更新。")
			}
			fmt.Println()
		case "tui", "console":
			fmt.Println("  正在切换到 TUI 工作台...")
			fmt.Println()
			return consoleActionOpenTUI
		case "restart":
			fmt.Print("确认重启网关？[y/N]: ")
			answer, _ := reader.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(answer)) == "y" {
				return consoleActionRestart
			}
			fmt.Println("  已取消。")
			fmt.Println()
		case "stop":
			fmt.Print("确认停止网关？[y/N]: ")
			answer, _ := reader.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(answer)) == "y" {
				return consoleActionStop
			}
			fmt.Println("  已取消。")
			fmt.Println()
		case "q", "quit", "exit":
			fmt.Println("  已退出纯命令模式，网关保持运行。")
			fmt.Printf("  重新进入: %s\n", elevatedCmd("console --simple"))
			fmt.Println()
			return consoleActionExit
		default:
			ui.Warn("未识别的命令，输入 help 查看可用命令")
			fmt.Println()
		}
	}
}

func runSimpleGroupChooser(reader *bufio.Reader, cfg *config.Config) {
	client := newConsoleClient(cfg)
	groups, err := client.ListProxyGroups()
	if err != nil {
		ui.Error("读取节点分组失败: %v", err)
		return
	}
	if len(groups) == 0 {
		ui.Info("当前没有可切换的节点分组")
		return
	}

	ui.Separator()
	color.New(color.Bold).Println("  节点分组")
	ui.Separator()
	for i, group := range groups {
		fmt.Printf("  %d) %s [%s]  当前: %s\n", i+1, group.Name, group.Type, group.Now)
	}
	fmt.Println()
	fmt.Print("选择节点分组 [1-", len(groups), "]，回车取消: ")
	rawGroup, _ := reader.ReadString('\n')
	rawGroup = strings.TrimSpace(rawGroup)
	if rawGroup == "" {
		fmt.Println()
		return
	}

	groupIndex := parseIndex(rawGroup, len(groups))
	if groupIndex < 0 {
		ui.Warn("无效的节点分组编号")
		fmt.Println()
		return
	}

	group := groups[groupIndex]
	runSimpleNodeChooser(reader, client, group)
}

func parseIndex(value string, length int) int {
	var idx int
	if _, err := fmt.Sscanf(value, "%d", &idx); err != nil {
		return -1
	}
	idx--
	if idx < 0 || idx >= length {
		return -1
	}
	return idx
}

func clearInteractiveScreen() {
	fmt.Print("\033[H\033[2J")
}

func printDeviceSetupPanel(ip string, apiPort int) {
	ui.Separator()
	color.New(color.Bold).Println("  设备接入")
	ui.Separator()
	fmt.Println()
	fmt.Println("  ┌───────────────────────────────┐")
	fmt.Printf("  │  网关 (Gateway):  %s\n", color.CyanString(ip))
	fmt.Printf("  │  DNS:             %s\n", color.CyanString(ip))
	fmt.Printf("  │  IP:              %s\n", color.New(color.Faint).Sprint("同网段任意可用 IP"))
	fmt.Printf("  │  子网掩码:        %s\n", color.New(color.Faint).Sprint("255.255.255.0"))
	fmt.Println("  └───────────────────────────────┘")
	fmt.Println()
	fmt.Printf("  API 面板: http://%s:%d/ui\n", ip, apiPort)
	fmt.Println()
}

func printStartGuide(cfg *config.Config, logFile string) {
	ui.Separator()
	color.New(color.Bold).Println("  功能导航")
	ui.Separator()
	fmt.Println()
	if cfg.Runtime.Tun.Enabled {
		fmt.Println("  1. 局域网共享已经就绪：Switch / PS5 / Apple TV / 手机改网关和 DNS 就能接入")
	} else {
		fmt.Printf("  1. 运行 %s，然后 %s，解锁局域网透明代理\n", elevatedCmd("tun on"), elevatedCmd("restart"))
	}
	switch cfg.Extension.Mode {
	case "chains":
		fmt.Println("  2. 当前 chains 已开启，适合 Claude / ChatGPT / Codex / Cursor 的稳定使用场景")
	case "script":
		fmt.Println("  2. 当前 script 已开启，适合自定义复杂分流逻辑")
	default:
		fmt.Println("  2. 运行 gateway chains，体验内置链式代理向导")
	}
	fmt.Println("  3. 运行 gateway config，集中管理代理来源 / 规则 / 扩展")
	fmt.Printf("  4. %s\n", followLogCommand(logFile))
	fmt.Println()
}

func compactModeSummary(cfg *config.Config) string {
	parts := []string{}
	if cfg.Runtime.Tun.Enabled {
		parts = append(parts, color.GreenString("TUN on"))
	} else {
		parts = append(parts, color.YellowString("TUN off"))
	}

	switch cfg.Extension.Mode {
	case "chains":
		mode := "chains"
		if cfg.Extension.ResidentialChain != nil && cfg.Extension.ResidentialChain.Mode != "" {
			mode += "/" + cfg.Extension.ResidentialChain.Mode
		}
		parts = append(parts, color.GreenString(mode))
	case "script":
		parts = append(parts, color.GreenString("script"))
	default:
		parts = append(parts, color.New(color.Faint).Sprint("extension off"))
	}

	return strings.Join(parts, "  ·  ")
}

func compactEgressSummary(cfg *config.Config, report *egress.Report) string {
	if report == nil {
		return "等待探测"
	}
	if cfg.Extension.Mode == "chains" {
		from := "机场"
		if report.AirportNode != nil && strings.TrimSpace(report.AirportNode.Name) != "" {
			from = report.AirportNode.Name
		}
		if report.ResidentialExit != nil {
			return fmt.Sprintf("%s -> %s", from, report.ResidentialExit.AreaSummary())
		}
		if report.ProxyExit != nil {
			return fmt.Sprintf("%s -> %s", from, report.ProxyExit.AreaSummary())
		}
		return from + " -> 探测中"
	}
	if report.ProxyExit != nil {
		return report.ProxyExit.AreaSummary()
	}
	return "探测中"
}

func isInteractiveTerminal() bool {
	stdin, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	stdout, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (stdin.Mode()&os.ModeCharDevice) != 0 && (stdout.Mode()&os.ModeCharDevice) != 0
}

func printLastLines(path string, n int) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("  (无法读取日志)")
		return
	}
	lines := splitLines(string(data))
	start := len(lines) - n
	if start < 0 {
		start = 0
	}
	for _, line := range lines[start:] {
		fmt.Println("  " + line)
	}
}

func splitLines(s string) []string {
	var lines []string
	for len(s) > 0 {
		idx := 0
		for idx < len(s) && s[idx] != '\n' {
			idx++
		}
		lines = append(lines, s[:idx])
		if idx < len(s) {
			idx++ // skip \n
		}
		s = s[idx:]
	}
	return lines
}
