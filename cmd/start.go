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
      之后用 sudo gateway 进主菜单，或 sudo gateway stop 停止。
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
		color.Green("✔ 网关已启动")
		color.New(color.Faint).Println(a.Engine.LogPath())

		if !startForeground {
			color.New(color.Faint).Println("mihomo 已在后台运行；进主菜单 sudo gateway，停止 sudo gateway stop。")
			return nil
		}

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
