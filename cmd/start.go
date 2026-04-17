package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/tght/lan-proxy-gateway/internal/app"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "启动网关（非交互，供系统服务使用）",
	RunE: func(cmd *cobra.Command, args []string) error {
		maybeElevate()
		a, err := app.New()
		if err != nil {
			return err
		}
		if !a.Configured() {
			return fmt.Errorf("尚未完成初始化，请先运行 `gateway install` 或直接运行 `gateway` 进入向导")
		}
		ctx := cmd.Context()
		if err := a.Start(ctx); err != nil {
			return err
		}
		color.Green("✔ 网关已启动")
		color.New(color.Faint).Println(a.Engine.LogPath())

		// Wait for Ctrl+C.
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		color.Yellow("正在停止…")
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = stopCtx
		return a.Stop()
	},
}
