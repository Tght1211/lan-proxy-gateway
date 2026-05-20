//go:build windows

package cmd

import (
	"os/exec"
	"strconv"
	"syscall"
)

func configureWebUIDaemonCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

func terminateWebUIDaemon(pid int) {
	_ = exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
}
