package cmd

import (
	"archive/zip"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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

// downloadMihomo automatically downloads and installs mihomo
func downloadMihomo() error {
	ui.Info("正在自动下载 mihomo...")

	spec, err := resolveMihomoAsset(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	// Try mirrors in order
	mirrors := []string{
		spec.URL,
		"https://ghp.ci/" + spec.URL,
		"https://hub.gitmirror.com/" + spec.URL,
		"https://github.moeyy.xyz/" + spec.URL,
	}

	// Find binary installation path
	p := platform.New()
	binPath := p.GetBinaryPath()

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(binPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "gateway-mihomo-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, spec.AssetName)

	// Try downloading from each URL
	var lastErr error
	for i, mirrorURL := range mirrors {
		ui.Info("尝试从 %s 下载...", getDomain(mirrorURL))
		if err := downloadFile(mirrorURL, archivePath); err == nil {
			if err := extractMihomoArchive(archivePath, binPath, spec); err != nil {
				os.Remove(binPath)
				return fmt.Errorf("解压 mihomo 失败: %v", err)
			}
			if runtime.GOOS != "windows" {
				if err := os.Chmod(binPath, 0755); err != nil {
					return fmt.Errorf("设置权限失败: %v", err)
				}
			}
			ui.Success("mihomo 下载成功")
			return nil
		} else {
			lastErr = err
		}

		// Remove partially downloaded file
		os.Remove(archivePath)
		os.Remove(binPath)

		if i == len(mirrors)-1 && lastErr != nil {
			return fmt.Errorf("all mihomo mirrors failed: %w", lastErr)
		}
		if i == len(mirrors)-1 {
			return fmt.Errorf("所有镜像都下载失败")
		}
	}

	return fmt.Errorf("mihomo 下载失败")
}

type mihomoAssetSpec struct {
	Version   string
	AssetName string
	Format    string
	URL       string
}

func resolveMihomoAsset(goos, goarch string) (mihomoAssetSpec, error) {
	version := "v1.19.8"
	spec := mihomoAssetSpec{Version: version}

	switch {
	case goos == "darwin" && goarch == "arm64":
		spec.AssetName = fmt.Sprintf("mihomo-darwin-arm64-%s.gz", version)
		spec.Format = "gz"
	case goos == "darwin" && goarch == "amd64":
		spec.AssetName = fmt.Sprintf("mihomo-darwin-amd64-compatible-%s.gz", version)
		spec.Format = "gz"
	case goos == "linux" && goarch == "amd64":
		spec.AssetName = fmt.Sprintf("mihomo-linux-amd64-compatible-%s.gz", version)
		spec.Format = "gz"
	case goos == "linux" && goarch == "arm64":
		spec.AssetName = fmt.Sprintf("mihomo-linux-arm64-%s.gz", version)
		spec.Format = "gz"
	case goos == "windows" && goarch == "amd64":
		spec.AssetName = fmt.Sprintf("mihomo-windows-amd64-compatible-%s.zip", version)
		spec.Format = "zip"
	default:
		return mihomoAssetSpec{}, fmt.Errorf("不支持的平台: %s/%s", goos, goarch)
	}

	spec.URL = fmt.Sprintf("https://github.com/MetaCubeX/mihomo/releases/download/%s/%s", version, spec.AssetName)
	return spec, nil
}

func downloadFile(url, dest string) error {
	return downloadGatewayURLToFileWithWindowsFallback(url, dest, 120*time.Second)
}

func extractMihomoArchive(archivePath, binPath string, spec mihomoAssetSpec) error {
	switch spec.Format {
	case "gz":
		return extractGzipBinary(archivePath, binPath)
	case "zip":
		return extractZipBinary(archivePath, binPath)
	default:
		return fmt.Errorf("未知的归档格式: %s", spec.Format)
	}
}

func extractGzipBinary(archivePath, binPath string) error {
	src, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer src.Close()

	gr, err := gzip.NewReader(src)
	if err != nil {
		return err
	}
	defer gr.Close()

	dst, err := os.Create(binPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, gr)
	return err
}

func extractZipBinary(archivePath, binPath string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		if isMihomoWindowsBinary(f.Name) {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			dst, err := os.Create(binPath)
			if err != nil {
				return err
			}
			defer dst.Close()

			_, err = io.Copy(dst, rc)
			return err
		}
	}

	return fmt.Errorf("压缩包中未找到 mihomo.exe")
}

func isMihomoWindowsBinary(name string) bool {
	base := strings.ToLower(filepath.Base(name))
	if base == "mihomo.exe" {
		return true
	}
	return strings.HasPrefix(base, "mihomo-") && strings.HasSuffix(base, ".exe")
}

// Extract domain from URL for display
func getDomain(url string) string {
	if strings.Contains(url, "ghp.ci") {
		return "ghp.ci"
	} else if strings.Contains(url, "gitmirror.com") {
		return "gitmirror"
	} else if strings.Contains(url, "moeyy.xyz") {
		return "moeyy.xyz"
	} else if strings.Contains(url, "github.com") {
		return "GitHub"
	}
	return "未知"
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

	// Step 2: Check and install mihomo
	ui.Step(2, 6, "检查 mihomo...")
	p := platform.New()

	binary, err := p.FindBinary()
	if err != nil {
		ui.Warn("未找到 mihomo，正在自动下载...")
		err = downloadMihomo()
		if err != nil {
			ui.Error("mihomo 下载失败: %v", err)
			ui.Error("请手动安装 mihomo: https://github.com/MetaCubeX/mihomo/releases")
			os.Exit(1)
		}
		binary, err = p.FindBinary()
		if err != nil {
			ui.Error("mihomo 安装失败，请手动检查")
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
		switch {
		case cfg.Proxy.Source == "url" && cfg.Proxy.SubscriptionURL != "":
			ui.Info("已有配置 [订阅链接模式]")
			url := cfg.Proxy.SubscriptionURL
			if len(url) > 50 {
				url = url[:50] + "..."
			}
			fmt.Printf("  当前订阅: %s\n", url)
			needConfig = false
		case cfg.Proxy.Source == "file" && cfg.Proxy.ConfigFile != "":
			ui.Info("已有配置 [配置文件模式]")
			fmt.Printf("  配置文件: %s\n", cfg.Proxy.ConfigFile)
			needConfig = false
		case cfg.Proxy.Source == "proxy" && cfg.Proxy.DirectProxy != nil && cfg.Proxy.DirectProxy.Server != "":
			dp := cfg.Proxy.DirectProxy
			ui.Info("已有配置 [直接代理模式]")
			fmt.Printf("  代理服务器: %s %s:%d\n", dp.Type, dp.Server, dp.Port)
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
		fmt.Println("  1) 订阅链接   — 机场提供的 Clash/mihomo 订阅 URL")
		fmt.Println("  2) 配置文件   — 本地 Clash/mihomo YAML 配置文件")
		fmt.Println("  3) 直接代理   — 手动填写 SOCKS5/HTTP 代理服务器（无需订阅）")
		fmt.Println()
		fmt.Print("请选择 [1/2/3]: ")
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
			path = expandUserPath(path)
			validatedPath, count, err := validateSubscriptionFile(path)
			if err != nil {
				ui.Error("订阅文件校验失败: %s", err)
				os.Exit(1)
			}

			cfg.Proxy.Source = "file"
			cfg.Proxy.ConfigFile = validatedPath
			ui.Success("订阅文件校验通过，识别到 %d 个节点", count)

			fmt.Println()
			fmt.Print("给代理源起个名字 [subscription]: ")
			name, _ := reader.ReadString('\n')
			name = strings.TrimSpace(name)
			if name != "" {
				cfg.Proxy.SubscriptionName = name
			}

		case "3":
			// Direct proxy mode
			fmt.Println()
			color.New(color.Bold).Println("直接代理服务器配置")
			fmt.Printf("  %s\n", color.New(color.Faint).Sprint("适合手头已有 SOCKS5/HTTP 代理，不需要订阅链接"))
			fmt.Println()

			dp := &config.DirectProxyConfig{}

			fmt.Print("代理服务器地址（域名或 IP）: ")
			server, _ := reader.ReadString('\n')
			server = strings.TrimSpace(server)
			if server == "" {
				ui.Error("代理服务器地址不能为空")
				os.Exit(1)
			}
			dp.Server = server

			fmt.Print("端口: ")
			portStr, _ := reader.ReadString('\n')
			portStr = strings.TrimSpace(portStr)
			var port int
			if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil || port <= 0 || port > 65535 {
				ui.Error("端口无效，请输入 1-65535 之间的整数")
				os.Exit(1)
			}
			dp.Port = port

			fmt.Print("协议类型 [socks5/http，默认 socks5]: ")
			proxyType, _ := reader.ReadString('\n')
			proxyType = strings.ToLower(strings.TrimSpace(proxyType))
			if proxyType == "http" {
				dp.Type = "http"
			} else {
				dp.Type = "socks5"
			}

			fmt.Print("用户名（无需认证直接回车）: ")
			username, _ := reader.ReadString('\n')
			username = strings.TrimSpace(username)
			if username != "" {
				dp.Username = username
				fmt.Print("密码: ")
				password, _ := reader.ReadString('\n')
				dp.Password = strings.TrimSpace(password)
			}

			fmt.Print("节点名称 [MyProxy]: ")
			nodeName, _ := reader.ReadString('\n')
			nodeName = strings.TrimSpace(nodeName)
			if nodeName != "" {
				dp.Name = nodeName
			} else {
				dp.Name = "MyProxy"
			}

			cfg.Proxy.Source = "proxy"
			cfg.Proxy.SubscriptionName = "direct"
			cfg.Proxy.DirectProxy = dp

			ui.Success("直接代理配置完成: %s %s:%d", dp.Type, dp.Server, dp.Port)

		default:
			// URL mode
			fmt.Println()
			color.New(color.Bold).Println("请输入你的代理订阅链接:")
			fmt.Printf("  %s\n", color.New(color.Faint).Sprint("（通常是机场提供的 Clash/mihomo 订阅 URL）"))
			fmt.Println()
			fmt.Print("> ")
			url, _ := reader.ReadString('\n')
			url = strings.TrimSpace(url)
			count, err := validateSubscriptionURL(url)
			if err != nil {
				ui.Error("订阅链接校验失败: %s", err)
				os.Exit(1)
			}

			cfg.Proxy.Source = "url"
			cfg.Proxy.SubscriptionURL = url
			ui.Success("订阅链接校验通过，识别到 %d 个节点", count)

			fmt.Println()
			fmt.Print("给订阅起个名字 [subscription]: ")
			name, _ := reader.ReadString('\n')
			name = strings.TrimSpace(name)
			if name != "" {
				cfg.Proxy.SubscriptionName = name
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
		color.New(color.Bold).Println("  启动方式")
		fmt.Println()
		fmt.Printf("  菜单控制台:  %s\n", elevatedCmd("start"))
		fmt.Println()
		color.New(color.Bold).Println("  常用入口")
		fmt.Println()
		fmt.Printf("  重新进入控制台: %s\n", elevatedCmd("console"))
		fmt.Println("  查看状态:       gateway status")
		fmt.Printf("  停止网关:       %s\n", elevatedCmd("stop"))
		fmt.Println()
		if runtime.GOOS == "windows" {
			fmt.Printf("  %s\n", color.New(color.Faint).Sprint("注意: start / stop / restart 需要以管理员身份运行终端"))
			fmt.Println()
		}
		fmt.Printf("  %s\n", color.New(color.Faint).Sprint("启动后，将其他设备的网关和 DNS 设为本机 IP 即可"))
	} else {
		ui.Separator()
		ui.Error("安装未完成，请检查上方错误信息")
		os.Exit(1)
	}
}
