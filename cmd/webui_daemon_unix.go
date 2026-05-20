//go:build darwin || linux

package cmd

import (
	"os/exec"
	"syscall"
	"time"
)

func configureWebUIDaemonCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminateWebUIDaemon(pid int) {
	_ = syscall.Kill(pid, syscall.SIGTERM)
	time.Sleep(500 * time.Millisecond)
	if syscall.Kill(pid, 0) == nil {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}
