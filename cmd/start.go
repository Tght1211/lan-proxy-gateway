package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
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

func init() {
	rootCmd.AddCommand(startCmd)
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
	if cfg.ProxySource == "file" {
		if cfg.ProxyConfigFile == "" {
			ui.Error("配置文件路径未设置，请检查 gateway.yaml")
			os.Exit(1)
		}
		providerFile := filepath.Join(dDir, "proxy_provider", cfg.SubscriptionName+".yaml")
		count, err := proxy.ExtractProxies(cfg.ProxyConfigFile, providerFile)
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
	ui.Step(4, 5, "启动 mihomo (TUN 模式)...")

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

	// Step 5: Verify TUN
	ui.Step(5, 5, "验证 TUN 接口...")
	tunIf, err := p.DetectTUNInterface()
	if err == nil && tunIf != "" {
		ui.Success("TUN 接口已创建: %s", tunIf)
	} else {
		ui.Warn("TUN 接口未检测到（可能还在创建中）")
	}

	// Print connection info
	fmt.Println()
	ui.Separator()
	color.New(color.FgGreen, color.Bold).Println("  LAN Proxy Gateway 已启动！")
	ui.Separator()
	fmt.Println()
	fmt.Printf("  %-14s %s\n", color.New(color.Bold).Sprint("本机 IP:"), ip)
	fmt.Printf("  %-14s %s\n", color.New(color.Bold).Sprint("网络接口:"), iface)
	fmt.Printf("  %-14s http://%s:%d/ui\n", color.New(color.Bold).Sprint("API 面板:"), ip, cfg.Ports.API)
	fmt.Println()
	fmt.Println(color.New(color.Bold).Sprint("  其他设备网络设置:"))
	fmt.Println("  ┌───────────────────────────────┐")
	fmt.Printf("  │  网关 (Gateway):  %s\n", color.CyanString(ip))
	fmt.Printf("  │  DNS:             %s\n", color.CyanString(ip))
	fmt.Printf("  │  IP:              %s\n", color.New(color.Faint).Sprint("同网段任意可用 IP"))
	fmt.Printf("  │  子网掩码:        %s\n", color.New(color.Faint).Sprint("255.255.255.0"))
	fmt.Println("  └───────────────────────────────┘")
	fmt.Println()
	fmt.Printf("  %s tail -f %s\n", color.New(color.Faint).Sprint("日志:"), logFile)
	fmt.Printf("  %s sudo gateway stop\n", color.New(color.Faint).Sprint("停止:"))
	fmt.Println()
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
