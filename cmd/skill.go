package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/agentskill"
	"os"
	"path/filepath"
)

func init() {
	var output string
	var force bool
	skill := &cobra.Command{Use: "skill", Short: "导出供外部 AI Agent 安装的 Skill"}
	export := &cobra.Command{Use: "export", Short: "导出不含本机配置的 Skill ZIP", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if output == "" {
			return fmt.Errorf("请用 --output 指定 ZIP 路径")
		}
		flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
		if force {
			flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		}
		f, err := os.OpenFile(output, flags, 0600)
		if err != nil {
			return err
		}
		err = agentskill.WriteZip(f)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		path, _ := filepath.Abs(output)
		fmt.Fprintln(cmd.OutOrStdout(), path)
		return nil
	}}
	export.Flags().StringVar(&output, "output", "", "导出 ZIP 路径")
	export.Flags().BoolVar(&force, "force", false, "替换已存在的导出文件")
	skill.AddCommand(export)
	rootCmd.AddCommand(skill)
}
