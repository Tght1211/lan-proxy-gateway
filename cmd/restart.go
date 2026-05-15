package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/tght/lan-proxy-gateway/internal/app"
)

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "重启网关",
	RunE: func(cmd *cobra.Command, args []string) error {
		maybeElevate()
		a, err := app.New()
		if err != nil {
			return err
		}
		if !a.Configured() {
			return fmt.Errorf("尚未完成初始化，请先运行 `gateway install` 或直接运行 `gateway` 进入向导")
		}
		color.Yellow("正在停止…")
		if err := a.Stop(); err != nil {
			return err
		}
		color.Yellow("正在启动…")
		if err := a.Start(cmd.Context()); err != nil {
			return err
		}
		color.Green("✔ 网关已重启")
		color.New(color.Faint).Println(a.Engine.LogPath())
		return nil
	},
}
