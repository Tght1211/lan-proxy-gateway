//go:build windows

package engine

import (
	"os"
	"os/exec"
	"syscall"
)

func configureProcAttrs(cmd *exec.Cmd) {
	// Hide child window on Windows.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

func terminateProcess(p *os.Process) {
	// Windows lacks SIGTERM; the safest cross-API signal is Kill.
	_ = p.Kill()
}

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Windows FindProcess always succeeds; use an actual probe.
	err = p.Signal(syscall.Signal(0))
	return err == nil
}
