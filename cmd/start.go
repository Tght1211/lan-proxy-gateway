package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/gateway"
)

var startForeground bool

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "启动网关",
	Long: `启动网关。

默认: spawn 一个脱离终端的守护进程后立即返回 shell；
      之后运行 gateway 进主菜单，或 gateway stop 停止
      （Linux/macOS 需 sudo 前缀）。
--foreground: 守护主体在前台阻塞（launchd / systemd 用，等价于 gateway run）。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		maybeElevate()
		a, err := app.New()
		if err != nil {
			return err
		}
		if !a.Configured() {
			return fmt.Errorf("尚未完成初始化，请先运行 `gateway install` 或直接运行 `gateway` 进入向导")
		}
		if startForeground {
			return runForeground(cmd, a)
		}
		alreadyRunning := a.Running()
		if err := a.Start(cmd.Context()); err != nil {
			return err
		}
		if alreadyRunning {
			color.Green("✔ 网关已在运行，当前 CLI 已连接到现有服务")
			printDeviceGuide(a)
			return nil
		}
		color.Green("✔ 网关已启动")
		printDeviceGuide(a)
		return nil
	},
}

// runCmd is the hidden daemon body. `start --foreground` aliases to it so the
// existing systemd unit / launchd plist keep working unchanged.
var runCmd = &cobra.Command{
	Use:    "run",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		maybeElevate()
		a, err := app.New()
		if err != nil {
			return err
		}
		return runForeground(cmd, a)
	},
}

func runForeground(cmd *cobra.Command, a *app.App) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return a.Run(ctx)
}

func init() {
	startCmd.Flags().BoolVar(&startForeground, "foreground", false, "前台运行守护主体（给 launchd / systemd 用）")
}

// printDeviceGuide prints the "point your devices at me" instructions.
func printDeviceGuide(a *app.App) {
	gs, _ := a.Gateway.Status()
	fmt.Println()
	fmt.Println(gateway.DeviceGuide(gs))
}
