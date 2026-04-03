package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/platform"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

var consoleSimple bool

var consoleCmd = &cobra.Command{
	Use:   "console",
	Short: "进入运行中的控制台（TUI 或纯命令模式）",
	Long: `连接到 Gateway 控制台，而不重新启动网关。

示例:
  sudo gateway console
  sudo gateway console --simple`,
	Run: runConsole,
}

func init() {
	rootCmd.AddCommand(consoleCmd)
	consoleCmd.Flags().BoolVar(&consoleSimple, "simple", false, "使用纯命令模式进入控制台")
}

func runConsole(cmd *cobra.Command, args []string) {
	checkRoot()
	if !isInteractiveTerminal() {
		ui.Info("console 需要在交互终端中运行")
		return
	}

	p := platform.New()
	iface, _ := p.DetectDefaultInterface()
	ip, _ := p.DetectInterfaceIP(iface)
	dataDir := ensureDataDir()
	logFile := "/tmp/lan-proxy-gateway.log"

	runInteractiveConsoleLoop(consoleSimple, logFile, ip, iface, dataDir, func() {
		startSimple = consoleSimple
		runStart(startCmd, nil)
	})
}
