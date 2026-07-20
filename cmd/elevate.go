package cmd

import (
	"os"

	"github.com/fatih/color"

	"github.com/tght/lan-proxy-gateway/internal/platform"
)

// maybeElevate checks for admin privileges and re-execs under sudo if needed.
func maybeElevate() {
	admin, _ := platform.Current().IsAdmin()
	if admin {
		return
	}
	color.Yellow("此操作需要 sudo 权限，正在切换…")
	if err := reexecWithSudo(); err != nil {
		color.Red("sudo 切换失败: %v", err)
		os.Exit(1)
	}
}
