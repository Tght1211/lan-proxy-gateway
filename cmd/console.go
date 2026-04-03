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
	Short: "进入运行中的控制台（默认简单模式，可切到 TUI）",
	Long: `连接到 Gateway 控制台，而不重新启动网关。

示例 (macOS/Linux 需要 sudo，Windows 需要管理员终端):
  gateway console
  gateway console --simple
  gateway console --tui`,
	Run: runConsole,
}

func init() {
	rootCmd.AddCommand(consoleCmd)
	consoleCmd.Flags().BoolVar(&consoleSimple, "simple", false, "使用纯命令模式进入控制台（默认）")
	consoleCmd.Flags().BoolVar(&consoleTUI, "tui", false, "使用 TUI 工作台进入控制台")
}

func runConsole(cmd *cobra.Command, args []string) {
	simpleMode, err := resolveConsoleSimpleMode(cmd, true, consoleSimple, consoleTUI)
	if err != nil {
		ui.Error("%s", err)
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
	dataDir := ensureDataDir()
	logFile := defaultLogFile()

	runInteractiveConsoleLoop(simpleMode, logFile, ip, iface, dataDir, func() {
		runStartWithMode(simpleMode, startCmd, nil)
	})
}
