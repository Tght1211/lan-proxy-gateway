package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/mihomo"
	"github.com/tght/lan-proxy-gateway/internal/platform"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看网关运行状态",
	Run:   runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) {
	p := platform.New()

	// Load config for port info
	cfg := loadConfigOrDefault()
	apiPort := cfg.Ports.API

	fmt.Println()
	ui.Separator()
	color.New(color.Bold).Println("  运行状态")
	ui.Separator()

	// mihomo process
	running, pid, _ := p.IsRunning()
	if running {
		fmt.Printf("  mihomo:      %s (PID: %d)\n", color.GreenString("运行中"), pid)
	} else {
		fmt.Printf("  mihomo:      %s\n", color.RedString("未运行"))
		fmt.Println()
		fmt.Println("  启动: sudo gateway start")
		return
	}

	// IP forwarding
	forwarding, _ := p.IsIPForwardingEnabled()
	if forwarding {
		fmt.Printf("  IP 转发:     %s\n", color.GreenString("已开启"))
	} else {
		fmt.Printf("  IP 转发:     %s\n", color.RedString("未开启"))
	}

	// TUN interface
	tunIf, err := p.DetectTUNInterface()
	if err == nil && tunIf != "" {
		fmt.Printf("  TUN 接口:    %s\n", color.GreenString(tunIf))
	} else {
		fmt.Printf("  TUN 接口:    %s\n", color.RedString("未检测到"))
	}

	// Network info
	iface, _ := p.DetectDefaultInterface()
	ip, _ := p.DetectInterfaceIP(iface)
	fmt.Printf("  网络接口:    %s\n", iface)
	fmt.Printf("  局域网 IP:   %s\n", ip)

	// Query mihomo API
	apiURL := mihomo.FormatAPIURL("127.0.0.1", apiPort)
	client := mihomo.NewClient(apiURL, cfg.APISecret)

	if !client.IsAvailable() {
		fmt.Println()
		ui.Warn("API 不可用 (%s)", apiURL)
		return
	}

	fmt.Println()
	ui.Separator()
	color.New(color.Bold).Println("  代理信息")
	ui.Separator()

	if v, err := client.GetVersion(); err == nil {
		fmt.Printf("  版本:        %s\n", v.Version)
	}

	if pg, err := client.GetProxyGroup("Proxy"); err == nil {
		fmt.Printf("  当前节点:    %s\n", color.CyanString(pg.Now))
	}

	if conn, err := client.GetConnections(); err == nil {
		fmt.Printf("  活跃连接:    %d\n", len(conn.Connections))
		fmt.Printf("  上传总量:    %s\n", ui.FormatBytes(conn.UploadTotal))
		fmt.Printf("  下载总量:    %s\n", ui.FormatBytes(conn.DownloadTotal))
	}

	// Device setup info
	fmt.Println()
	ui.Separator()
	color.New(color.Bold).Println("  设备配置")
	ui.Separator()
	fmt.Printf("  网关 (Gateway):  %s\n", color.CyanString(ip))
	fmt.Printf("  DNS:             %s\n", color.CyanString(ip))
	fmt.Printf("  API 面板:        http://%s:%d/ui\n", ip, apiPort)
	fmt.Println()
}

// loadConfigOrDefault loads the config file, falling back to defaults.
func loadConfigOrDefault() *config.Config {
	path := resolveConfigPath()
	cfg, err := config.Load(path)
	if err != nil {
		return config.DefaultConfig()
	}
	return cfg
}

// resolveConfigPath determines the config file path.
func resolveConfigPath() string {
	if cfgFile != "" {
		return cfgFile
	}
	// Check current directory
	if _, err := os.Stat("gateway.yaml"); err == nil {
		return "gateway.yaml"
	}
	// Check for legacy .secret
	if _, err := os.Stat(".secret"); err == nil {
		return ".secret"
	}
	return "gateway.yaml"
}

// resolveDataDir determines the data directory path.
func resolveDataDir() string {
	if dataDir != "" {
		return dataDir
	}
	return "data"
}

// checkRoot is defined in root_unix.go and root_windows.go

// loadConfigRequired loads the config or exits with error.
func loadConfigRequired() *config.Config {
	path := resolveConfigPath()

	// Try migrating from .secret if gateway.yaml doesn't exist
	if _, err := os.Stat(path); os.IsNotExist(err) || path == ".secret" {
		secretPath := filepath.Join(filepath.Dir(path), ".secret")
		if path == ".secret" {
			secretPath = path
		}
		if cfg, err := config.MigrateFromSecret(secretPath); err == nil && cfg != nil {
			yamlPath := filepath.Join(filepath.Dir(secretPath), "gateway.yaml")
			ui.Info("检测到旧版 .secret 配置，正在迁移到 gateway.yaml...")
			if err := config.Save(cfg, yamlPath); err == nil {
				ui.Success("配置已迁移到 %s", yamlPath)
				return cfg
			}
		}
	}

	cfg, err := config.Load(path)
	if err != nil {
		ui.Error("无法加载配置文件: %s", err)
		fmt.Println("  请先运行 gateway install")
		os.Exit(1)
	}
	return cfg
}

// ensureDataDir creates the data directory structure.
func ensureDataDir() string {
	dir := resolveDataDir()
	os.MkdirAll(filepath.Join(dir, "proxy_provider"), 0755)
	return dir
}

// portStr is a helper to convert int to string.
func portStr(port int) string {
	return strconv.Itoa(port)
}
