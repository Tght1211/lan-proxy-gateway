package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/tght/lan-proxy-gateway/internal/app"
)

var startForeground bool

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "启动网关",
	Long: `启动网关（默认后台运行）。

默认: 起 mihomo 后立即返回 shell，mihomo 作为孤儿进程在后台跑；
      之后运行 gateway 进主菜单，或 gateway stop 停止
      （Linux/macOS 需 sudo 前缀）。
--foreground: 阻塞当前终端直到 Ctrl+C 再 stop，给 launchd / systemd 用。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		maybeElevate()
		a, err := app.New()
		if err != nil {
			return err
		}
		if !a.Configured() {
			return fmt.Errorf("尚未完成初始化，请先运行 `gateway install` 或直接运行 `gateway` 进入向导")
		}
		if err := a.Start(cmd.Context()); err != nil {
			return err
		}
		a.StartSupervisor(cmd.Context())
		color.Green("✔ 网关已启动")
		color.New(color.Faint).Println(a.Engine.LogPath())

		if !startForeground {
			printMihomoConsoleHint(a)
			color.New(color.Faint).Printf("\nmihomo 已在后台运行；CLI 菜单 %s，停止 %s。\n",
				elevatedCmd(""), elevatedCmd("stop"))
			return nil
		}

		printMihomoConsoleHint(a)

		// --foreground: launchd / systemd 要前台进程，等 Ctrl+C 再优雅停止。
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		color.Yellow("正在停止…")
		return a.Stop()
	},
}

func init() {
	startCmd.Flags().BoolVar(&startForeground, "foreground", false, "前台阻塞直到 Ctrl+C（给 launchd / systemd 用）")
}

// printMihomoConsoleHint 在启动尾巴上提示 mihomo 自带的 external-ui 控制台地址
// （http://<host>:<api_port>/ui/）。这是 mihomo 引擎自己的面板，能切节点 / 看
// 流量；UI 静态文件需用户自行放到 workdir/ui（目录为空时 /ui 返回 404，不影响
// 代理本体）。本项目自带的 Web 控制台已移除，配置改动统一走 CLI 菜单。
func printMihomoConsoleHint(a *app.App) {
	mihomoPort := a.Cfg.Runtime.Ports.API
	if mihomoPort <= 0 {
		return
	}
	lanIP := ""
	if a.Gateway != nil {
		if err := a.Gateway.Detect(); err == nil {
			lanIP = a.Gateway.Info().IP
		}
	}

	bold := color.New(color.Bold)
	faint := color.New(color.Faint)

	fmt.Println()
	bold.Println("  ┌─ Mihomo 完整控制台 (external-ui) ───────────────────")
	bold.Printf("  │ ")
	if lanIP != "" {
		fmt.Printf("http://%s:%d/ui/\n", lanIP, mihomoPort)
	} else {
		fmt.Printf("http://localhost:%d/ui/\n", mihomoPort)
	}
	bold.Printf("  │ ")
	faint.Printf("mihomo 引擎自带面板（切节点 / 看流量）\n")
	bold.Printf("  │ ")
	faint.Printf("CLI 菜单（改配置 / 切模式 / 管接入）%s\n", elevatedCmd(""))
	bold.Println("  └────────────────────────────────────────────────────")
}
