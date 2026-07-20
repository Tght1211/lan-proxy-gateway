//go:build !windows

package app

import "syscall"

// detachSysProcAttr puts the daemon in its own session so it survives the
// parent CLI exiting (and never receives the terminal's SIGHUP).
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
