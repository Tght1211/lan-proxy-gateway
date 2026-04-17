package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/mihomo"
	"github.com/tght/lan-proxy-gateway/internal/platform"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "检查网关健康状态，异常时自动修复",
	Long: `检查 mihomo 进程、TUN 接口、API 可用性。
如果检测到异常，自动执行重启恢复。
可配合 cron 或 launchd 定期执行。`,
	Run: runHealth,
}

func init() {
	rootCmd.AddCommand(healthCmd)
}

func runHealth(cmd *cobra.Command, args []string) {
	checkRoot()

	p := platform.New()
	cfg := loadConfigOrDefault()
	healthy := true

	running, _, _ := p.IsRunning()
	if !running {
		ui.Error("[health] mihomo 进程未运行")
		healthy = false
	}

	if running {
		tunIf, err := p.DetectTUNInterface()
		if err != nil || tunIf == "" {
			ui.Error("[health] TUN 接口未检测到")
			healthy = false
		}
	}

	if running {
		apiURL := mihomo.FormatAPIURL("127.0.0.1", cfg.Runtime.Ports.API)
		client := mihomo.NewClient(apiURL, cfg.Runtime.APISecret)
		if !client.IsAvailable() {
			ui.Error("[health] mihomo API 不可用")
			healthy = false
		} else {
			if err := client.UpdateProxyProvider(cfg.Proxy.SubscriptionName); err != nil {
				ui.Warn("[health] 订阅刷新失败: %s", err)
			}
		}
	}

	if !healthy {
		ui.Warn("[health] 检测到异常，执行重启...")
		runStop(cmd, args)
		runStart(cmd, args)
		return
	}

	// Proactive: clear stale connections to free resources
	apiURL := mihomo.FormatAPIURL("127.0.0.1", cfg.Runtime.Ports.API)
	client := mihomo.NewClient(apiURL, cfg.Runtime.APISecret)
	if conn, err := client.GetConnections(); err == nil && len(conn.Connections) > 500 {
		ui.Info("[health] 活跃连接数 %d 较高，清理旧连接...", len(conn.Connections))
		client.CloseAllConnections()
	}

	fmt.Println()
	ui.Success("[health] 网关运行正常")
	os.Exit(0)
}
