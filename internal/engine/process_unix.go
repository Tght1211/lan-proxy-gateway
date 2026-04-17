//go:build darwin || linux

package engine

import (
	"os"
	"os/exec"
	"syscall"
)

func configureProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminateProcess(p *os.Process) {
	_ = p.Signal(syscall.SIGTERM)
}

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}
