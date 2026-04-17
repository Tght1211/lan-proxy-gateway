package cmd

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

var permissionCmd = &cobra.Command{
	Use:   "permission",
	Short: "配置或查看免密权限控制",
}

var permissionPrintCmd = &cobra.Command{
	Use:   "print",
	Short: "打印 sudoers 配置片段",
	Run:   runPermissionPrint,
}

var permissionInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "安装 sudoers 规则，允许后续普通权限自动提权控制",
	Run:   runPermissionInstall,
}

var permissionStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看当前权限控制状态",
	Run:   runPermissionStatus,
}

func init() {
	rootCmd.AddCommand(permissionCmd)
	permissionCmd.AddCommand(permissionPrintCmd)
	permissionCmd.AddCommand(permissionInstallCmd)
	permissionCmd.AddCommand(permissionStatusCmd)
}

func runPermissionPrint(cmd *cobra.Command, args []string) {
	if runtime.GOOS == "windows" {
		fmt.Println("Windows 暂不支持 sudoers 模式。请使用管理员权限运行。")
		return
	}

	currentUser := currentUsername()
	binary := currentBinaryPath()

	fmt.Println(renderSudoers(currentUser, binary))
}

func runPermissionInstall(cmd *cobra.Command, args []string) {
	if runtime.GOOS == "windows" {
		ui.Error("Windows 暂不支持 sudoers 安装")
		return
	}

	checkRoot()

	path := sudoersPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		ui.Error("创建 sudoers.d 目录失败: %s", err)
		os.Exit(1)
	}

	content := renderSudoers(currentUsername(), currentBinaryPath())
	if err := os.WriteFile(path, []byte(content), 0440); err != nil {
		ui.Error("写入 sudoers 文件失败: %s", err)
		os.Exit(1)
	}

	ui.Success("已安装免密控制配置: %s", path)
	fmt.Println()
	fmt.Println("  之后可直接运行 gateway start / stop / restart，CLI 会自动尝试 sudo -n 提权。")
	fmt.Println()
}

func runPermissionStatus(cmd *cobra.Command, args []string) {
	fmt.Println()
	fmt.Printf("  当前用户: %s\n", currentUsername())
	fmt.Printf("  二进制路径: %s\n", currentBinaryPath())
	if runtime.GOOS != "windows" {
		fmt.Printf("  sudoers 文件: %s\n", sudoersPath())
		if _, err := os.Stat(sudoersPath()); err == nil {
			fmt.Println("  状态: 已检测到 sudoers 配置文件")
		} else {
			fmt.Println("  状态: 尚未检测到 sudoers 配置文件")
		}
	}
	fmt.Println()
}

func renderSudoers(username, binary string) string {
	return fmt.Sprintf(`# lan-proxy-gateway passwordless control
# install path example: %s
%s ALL=(root) NOPASSWD: %s, %s *
`, sudoersPath(), username, binary, binary)
}

func currentUsername() string {
	u, err := user.Current()
	if err == nil && u.Username != "" {
		return u.Username
	}
	if name := os.Getenv("USER"); name != "" {
		return name
	}
	return "your-user"
}

func currentBinaryPath() string {
	self, err := os.Executable()
	if err != nil {
		return "gateway"
	}
	if strings.Contains(self, string(filepath.Separator)+"go-build") {
		return "gateway"
	}
	abs, err := filepath.Abs(self)
	if err != nil {
		return self
	}
	return abs
}

func sudoersPath() string {
	return "/etc/sudoers.d/lan-proxy-gateway"
}
