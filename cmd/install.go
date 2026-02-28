package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/mihomo"
	"github.com/tght/lan-proxy-gateway/internal/platform"
	tmpl "github.com/tght/lan-proxy-gateway/internal/template"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "安装向导 — 一键配置代理网关",
	Run:   runInstall,
}

func init() {
	rootCmd.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) {
	ui.ShowLogo()
	color.New(color.Bold).Println("欢迎使用 LAN Proxy Gateway 安装向导")
	ui.Separator()

	reader := bufio.NewReader(os.Stdin)

	// Step 1: System check
	ui.Step(1, 6, "检查系统环境...")
	fmt.Printf("  系统: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	// Step 2: Check mihomo
	ui.Step(2, 6, "检查 mihomo...")
	p := platform.New()

	binary, err := p.FindBinary()
	if err != nil {
		ui.Warn("未找到 mihomo")
		if runtime.GOOS == "darwin" {
			fmt.Println("  建议通过 Homebrew 安装: brew install mihomo")
		} else {
			fmt.Println("  请从 https://github.com/MetaCubeX/mihomo/releases 下载对应平台版本")
		}
		fmt.Println()
		fmt.Print("mihomo 安装完成后按 Enter 继续（或 Ctrl+C 退出）...")
		reader.ReadString('\n')
		binary, err = p.FindBinary()
		if err != nil {
			ui.Error("仍未找到 mihomo，请确认安装后重试")
			os.Exit(1)
		}
	}
	ui.Success("mihomo 已就绪: %s", binary)

	// Step 3: Download GeoIP/GeoSite
	ui.Step(3, 6, "下载 GeoIP/GeoSite 数据文件...")
	dDir := ensureDataDir()

	sources := mihomo.GeoDataSources(dDir)
	for _, src := range sources {
		name := filepath.Base(src.Dest)
		downloaded, err := mihomo.DownloadFile(src.URL, src.Dest)
		if err != nil {
			ui.Warn("%s 下载失败，尝试镜像源...", name)
			downloaded, err = mihomo.DownloadFile(src.Mirror, src.Dest)
			if err != nil {
				ui.Warn("%s 下载失败，mihomo 启动时会自动下载", name)
				continue
			}
		}
		if downloaded {
			ui.Success("%s 下载完成", name)
		} else {
			ui.Info("%s 已存在，跳过", name)
		}
	}

	// Step 4: Configure proxy source
	ui.Step(4, 6, "配置代理来源...")

	cfgPath := resolveConfigPath()
	cfg := loadConfigOrDefault()
	needConfig := true

	// Check existing config
	if _, err := os.Stat(cfgPath); err == nil && cfgPath != ".secret" {
		if cfg.ProxySource == "url" && cfg.SubscriptionURL != "" {
			ui.Info("已有配置 [订阅链接模式]")
			url := cfg.SubscriptionURL
			if len(url) > 40 {
				url = url[:40] + "..."
			}
			fmt.Printf("  当前订阅: %s\n", url)
			needConfig = false
		} else if cfg.ProxySource == "file" && cfg.ProxyConfigFile != "" {
			ui.Info("已有配置 [配置文件模式]")
			fmt.Printf("  配置文件: %s\n", cfg.ProxyConfigFile)
			needConfig = false
		}
		if !needConfig {
			fmt.Println()
			fmt.Print("是否重新配置？[y/N] ")
			answer, _ := reader.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(answer)) == "y" {
				needConfig = true
			}
		}
	}

	if needConfig {
		fmt.Println()
		color.New(color.Bold).Println("请选择代理来源:")
		fmt.Println("  1) 订阅链接（机场提供的 Clash/mihomo 订阅 URL）")
		fmt.Println("  2) 配置文件（本地 Clash/mihomo YAML 配置文件）")
		fmt.Println()
		fmt.Print("请选择 [1/2]: ")
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "2":
			// File mode
			fmt.Println()
			color.New(color.Bold).Println("请输入配置文件的路径:")
			fmt.Printf("  %s\n", color.New(color.Faint).Sprint("（支持包含 proxies 段落的 Clash/mihomo YAML 配置）"))
			fmt.Println()
			fmt.Print("> ")
			path, _ := reader.ReadString('\n')
			path = strings.TrimSpace(path)
			if strings.HasPrefix(path, "~") {
				home, _ := os.UserHomeDir()
				path = filepath.Join(home, path[1:])
			}
			if _, err := os.Stat(path); os.IsNotExist(err) {
				ui.Error("文件不存在: %s", path)
				os.Exit(1)
			}

			cfg.ProxySource = "file"
			cfg.ProxyConfigFile = path

			fmt.Println()
			fmt.Print("给代理源起个名字 [subscription]: ")
			name, _ := reader.ReadString('\n')
			name = strings.TrimSpace(name)
			if name != "" {
				cfg.SubscriptionName = name
			}

		default:
			// URL mode
			fmt.Println()
			color.New(color.Bold).Println("请输入你的代理订阅链接:")
			fmt.Printf("  %s\n", color.New(color.Faint).Sprint("（通常是机场提供的 Clash/mihomo 订阅 URL）"))
			fmt.Println()
			fmt.Print("> ")
			url, _ := reader.ReadString('\n')
			url = strings.TrimSpace(url)
			if url == "" {
				ui.Error("订阅链接不能为空")
				os.Exit(1)
			}

			cfg.ProxySource = "url"
			cfg.SubscriptionURL = url

			fmt.Println()
			fmt.Print("给订阅起个名字 [subscription]: ")
			name, _ := reader.ReadString('\n')
			name = strings.TrimSpace(name)
			if name != "" {
				cfg.SubscriptionName = name
			}
		}

		// Save config
		yamlPath := "gateway.yaml"
		if err := config.Save(cfg, yamlPath); err != nil {
			ui.Error("保存配置失败: %s", err)
			os.Exit(1)
		}
		ui.Success("代理配置已保存到 %s", yamlPath)
	}

	// Step 5: Detect network & generate config
	ui.Step(5, 6, "检测网络并生成配置...")

	iface, _ := p.DetectDefaultInterface()
	ip, _ := p.DetectInterfaceIP(iface)
	gateway, _ := p.DetectGateway()

	ui.Separator()
	fmt.Printf("  %-14s %s\n", "CPU 架构:", platform.DetectArch())
	fmt.Printf("  %-14s %s\n", "网络接口:", iface)
	fmt.Printf("  %-14s %s\n", "局域网 IP:", ip)
	fmt.Printf("  %-14s %s\n", "网关地址:", gateway)
	fmt.Printf("  %-14s %s\n", "mihomo:", binary)
	ui.Separator()

	configPath := filepath.Join(dDir, "config.yaml")
	if err := tmpl.RenderTemplate(cfg, iface, ip, configPath); err != nil {
		ui.Error("配置文件生成失败: %s", err)
		os.Exit(1)
	}
	ui.Success("配置文件已生成: %s", configPath)

	// Step 6: Verify
	ui.Step(6, 6, "安装验证...")

	allOK := true
	checkExists := func(path, label string) {
		if _, err := os.Stat(path); err == nil {
			ui.Success(label)
		} else {
			ui.Error("%s — 文件缺失: %s", label, path)
			allOK = false
		}
	}
	checkExists(binary, "mihomo 可执行文件")
	checkExists(configPath, "运行时配置文件")
	checkExists("gateway.yaml", "网关配置文件")

	fmt.Println()
	if allOK {
		ui.Separator()
		color.New(color.FgGreen, color.Bold).Println("  安装完成！")
		ui.Separator()
		fmt.Println()
		fmt.Println("  启动网关:  sudo gateway start")
		fmt.Println("  停止网关:  sudo gateway stop")
		fmt.Println("  查看状态:  gateway status")
		fmt.Println()
		fmt.Printf("  %s\n", color.New(color.Faint).Sprint("启动后，将其他设备的网关和 DNS 设为本机 IP 即可"))
	} else {
		ui.Separator()
		ui.Error("安装未完成，请检查上方错误信息")
		os.Exit(1)
	}
}
