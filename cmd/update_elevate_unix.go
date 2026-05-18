//go:build darwin || linux

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// reexecForUpdateInstall 在用户权限阶段下载完成后，切换到 sudo 重新 exec 自己，
// 通过隐藏 flag 把已下载的临时文件路径与目标 tag 透传给 root 子进程。这样即使
// sudo 默认的 env_reset 把 HTTPS_PROXY 等代理变量剥掉，安装阶段也不再需要联网。
func reexecForUpdateInstall(tag, tmpPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	sudo, err := exec.LookPath("sudo")
	if err != nil {
		return fmt.Errorf("未找到 sudo")
	}
	args := []string{
		"sudo", exe, "update",
		"--prefetched-tag=" + tag,
		"--prefetched-asset=" + tmpPath,
	}
	return syscall.Exec(sudo, args, os.Environ())
}
