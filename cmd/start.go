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

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().BoolVar(&startSimple, "simple", false, "使用简洁启动模式：打印摘要后直接返回，不进入 TUI")
}

func runStart(cmd *cobra.Command, args []string) {
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

	// If file mode, extract proxies first
	if cfg.Proxy.Source == "file" {
		if cfg.Proxy.ConfigFile == "" {
			ui.Error("配置文件路径未设置，请检查 gateway.yaml")
			os.Exit(1)
		}
		providerFile := filepath.Join(dDir, "proxy_provider", cfg.Proxy.SubscriptionName+".yaml")
		count, err := proxy.ExtractProxies(cfg.Proxy.ConfigFile, providerFile)
		if err != nil {
			ui.Error("提取代理节点失败: %s", err)
			os.Exit(1)
		}
		ui.Success("已从配置文件中提取 %d 个代理节点", count)
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

	logFile := "/tmp/lan-proxy-gateway.log"
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

	if isInteractiveTerminal() && !startSimple {
		for {
			action := runRuntimeConsole(logFile, ip, iface, dDir)
			switch action {
			case consoleActionOpenConfig:
				runConfigMenu(nil, nil)
			case consoleActionOpenChainsSetup:
				runChainsSetup(nil, nil)
			case consoleActionStop:
				runStop(nil, nil)
				return
			case consoleActionRestart:
				runStop(nil, nil)
				runStart(cmd, args)
				return
			default:
				return
			}
		}
	} else {
		printCompactStartSummary(cfg, dDir, ip, iface)
		if startSimple && isInteractiveTerminal() {
			fmt.Println("  已使用简洁启动模式；如需运行中工作台，请去掉 --simple")
			fmt.Println()
		}
	}
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
		fmt.Printf("  新版可用: %s -> sudo gateway update\n", color.YellowString(updateNotice.Latest))
	}
	if iface != "" {
		fmt.Printf("  网络接口: %s\n", iface)
	}
	fmt.Println()
}

func runStartConsole(logFile, ip, iface, dDir string) {
	reader := bufio.NewReader(os.Stdin)

	for {
		clearInteractiveScreen()
		cfg := loadConfigOrDefault()

		printCompactStartSummary(cfg, dDir, ip, iface)
		ui.Separator()
		color.New(color.Bold).Println("  快捷操作")
		fmt.Println()
		fmt.Println("  1) 完整状态")
		fmt.Println("  2) 配置中心")
		fmt.Println("  3) 链式代理 / 扩展")
		fmt.Println("  4) 设备接入说明")
		fmt.Println("  5) 最近日志")
		fmt.Println("  6) 功能导航")
		fmt.Println("  0) 退出控制台")
		fmt.Println()
		color.New(color.Faint).Println("  网关会继续保持运行；退出这里只是返回终端，不会停止服务")
		fmt.Println()
		fmt.Print("选择操作 [0-6]，也支持 s/g/e/d/l/h/q: ")

		input, _ := reader.ReadString('\n')
		choice := strings.ToLower(strings.TrimSpace(input))
		fmt.Println()

		switch choice {
		case "1", "s", "status":
			runStatus(nil, nil)
			waitEnter(reader)
		case "2", "g", "config", "menu":
			runConfigMenu(nil, nil)
		case "3", "e", "ext", "extension", "chains":
			if cfg.Extension.Mode == "chains" {
				runChainsStatus(nil, nil)
			} else {
				printExtensionStatus(cfg)
			}
			waitEnter(reader)
		case "4", "d", "device", "devices":
			printDeviceSetupPanel(ip, cfg.Runtime.Ports.API)
			waitEnter(reader)
		case "5", "l", "log", "logs":
			ui.Separator()
			color.New(color.Bold).Println("  最近日志")
			ui.Separator()
			printLastLines(logFile, 30)
			fmt.Println()
			fmt.Printf("  实时查看: tail -f %s\n", logFile)
			fmt.Println()
			waitEnter(reader)
		case "6", "h", "help", "guide":
			printStartGuide(cfg, logFile)
			waitEnter(reader)
		case "0", "q", "quit", "exit":
			fmt.Println("  已退出运行中控制台，网关保持运行。")
			fmt.Println()
			return
		default:
			ui.Warn("请输入 0-6，或 s/g/e/d/l/h/q")
			fmt.Println()
		}
	}
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
		fmt.Println("  1. 运行 sudo gateway tun on，然后 sudo gateway restart，解锁局域网透明代理")
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
	fmt.Printf("  4. tail -f %s\n", logFile)
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
