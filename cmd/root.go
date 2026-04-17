// Package cmd wires the 5-command CLI together.
// The root command, when invoked without any subcommand, launches the TUI console.
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/console"
)

// Version is injected via -ldflags.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:     "gateway",
	Short:   "LAN 代理网关 — 把本机变成局域网代理网关",
	Version: Version,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 进菜单、改配置、看状态都不需要 root；只有用户真点「启动」时才提示 sudo。
		a, err := app.New()
		if err != nil {
			return err
		}
		return console.Run(context.Background(), a)
	},
	SilenceUsage: true,
}

// Execute runs the root command. Called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(
		installCmd,
		startCmd,
		stopCmd,
		statusCmd,
		serviceCmd,
	)
}
