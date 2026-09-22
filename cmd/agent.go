package cmd

import (
	"encoding/json"
	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/app"
)

func init() {
	agent := &cobra.Command{Use: "agent", Short: "供外部 AI Agent 使用的诊断接口"}
	agent.AddCommand(&cobra.Command{Use: "snapshot", Short: "输出不含代理密码的配置与运行快照", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		status := a.Status()
		out := map[string]any{"version": Version, "status": status}
		stats, err := app.NewAPIClient(status.Ports.API, a.Paths.ConfigFile).Stats(cmd.Context())
		if err != nil {
			out["runtime_error"] = err.Error()
		} else {
			if len(stats.Relay.Active) > 100 {
				stats.Relay.Active = stats.Relay.Active[:100]
			}
			if len(stats.Relay.Recent) > 100 {
				stats.Relay.Recent = stats.Relay.Recent[:100]
			}
			stats.Relay.Traffic = nil
			stats.Health.History = nil
			out["runtime"] = stats
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
	}})
	rootCmd.AddCommand(agent)
}
