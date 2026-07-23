// Package cmd wires the CLI together.
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
		// The menu can render without root; privileged actions explain how to
		// relaunch it with sudo when needed.
		a, err := app.New()
		if err != nil {
			return err
		}
		return console.Run(context.Background(), a)
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command. Called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "机器可读 JSON 输出")
	rootCmd.AddCommand(
		initConfigCmd,
		installCmd,
		startCmd,
		restartCmd,
		stopCmd,
		statusCmd,
		serviceCmd,
		systemProxyCmd,
		routingCmd,
		updateCmd,
		runCmd,
	)
}
