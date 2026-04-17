package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "查看可供 AI 客户端安装的 skill 信息",
	Run:   runSkillShow,
}

var skillPathCmd = &cobra.Command{
	Use:   "path",
	Short: "输出 skill 目录路径",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(resolveSkillPath())
	},
}

func init() {
	rootCmd.AddCommand(skillCmd)
	skillCmd.AddCommand(skillPathCmd)
}

func runSkillShow(cmd *cobra.Command, args []string) {
	fmt.Println()
	fmt.Println("  AI Skill")
	fmt.Println()
	fmt.Printf("  skill 路径: %s\n", resolveSkillPath())
	fmt.Println("  推荐场景:")
	fmt.Println("    - 局域网共享开通与排障")
	fmt.Println("    - chains 链式代理开通与验证")
	fmt.Println("    - 策略组与节点切换")
	fmt.Println("    - 本机绕过代理开关")
	fmt.Println("    - 健康检查与日志定位")
	fmt.Println()
	fmt.Println("  安装后，AI 客户端可按场景调用 gateway CLI，而不是让用户手动记命令。")
	fmt.Println()
}

func resolveSkillPath() string {
	path, err := filepath.Abs("skills/lan-proxy-gateway-ops")
	if err != nil {
		return "skills/lan-proxy-gateway-ops"
	}
	return path
}
