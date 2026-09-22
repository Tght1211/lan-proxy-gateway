package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/app"
	"io"
)

var httpProxyCmd = &cobra.Command{Use: "http-proxy", Short: "配置供局域网设备使用的 HTTP/HTTPS 代理"}

func init() {
	httpProxyCmd.AddCommand(&cobra.Command{Use: "set", Short: "从标准输入读取 JSON 配置（密码不进入命令行参数）", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		var update app.HTTPProxyUpdate
		decoder := json.NewDecoder(io.LimitReader(cmd.InOrStdin(), 16<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&update); err != nil {
			return fmt.Errorf("HTTP 代理配置 JSON 无效: %w", err)
		}
		a, err := app.New()
		if err != nil {
			return err
		}
		if err = a.SetHTTPProxy(update); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "HTTP 代理配置已保存；运行中的核心将自动应用")
		return nil
	}})
}
