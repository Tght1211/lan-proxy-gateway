package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/fatih/color"

	"github.com/tght/lan-proxy-gateway/internal/platform"
)

// maybeElevate checks for admin privileges. If missing, it re-execs under sudo
// (Unix) or prints a clear message (Windows). Returns only when already admin.
func maybeElevate() {
	admin, _ := platform.Current().IsAdmin()
	if admin {
		return
	}
	if runtime.GOOS == "windows" {
		color.Red("此操作需要管理员权限。")
		color.Yellow("请关闭当前窗口，右键 PowerShell → 以管理员身份运行，再执行：")
		fmt.Println("  gateway")
		os.Exit(1)
	}
	color.Yellow("此操作需要 sudo 权限，正在切换…")
	if err := reexecWithSudo(); err != nil {
		color.Red("sudo 切换失败: %v", err)
		os.Exit(1)
	}
}
