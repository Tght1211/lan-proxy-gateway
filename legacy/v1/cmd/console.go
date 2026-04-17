package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/platform"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

var consoleSimple bool
var consoleTUI bool

var consoleCmd = &cobra.Command{
	Use:   "console",
	Short: "进入运行中的菜单式控制台",
	Long: `连接到 Gateway 控制台，而不重新启动网关。

示例 (macOS/Linux 需要 sudo，Windows 需要管理员终端):
  gateway console`,
	Run: runConsole,
}

func init() {
	rootCmd.AddCommand(consoleCmd)
	consoleCmd.Flags().BoolVar(&consoleSimple, "simple", false, "兼容旧参数；当前默认就是菜单式 CLI 控制台")
	consoleCmd.Flags().BoolVar(&consoleTUI, "tui", false, "已移除：旧版 TUI 工作台")
	_ = consoleCmd.Flags().MarkHidden("simple")
	_ = consoleCmd.Flags().MarkHidden("tui")
}

func runConsole(cmd *cobra.Command, args []string) {
	if rejectRemovedTUIFlag(cmd) {
		return
	}

	checkRoot()
	if !isInteractiveTerminal() {
		ui.Info("console 需要在交互终端中运行")
		return
	}

	p := platform.New()
	iface, _ := p.DetectDefaultInterface()
	ip, _ := p.DetectInterfaceIP(iface)
	logFile := defaultLogFile()

	runInteractiveConsoleLoop(logFile, ip, func() {
		runStartWithMode(startCmd, nil)
	})
}
