//go:build !windows

package procutil

import "syscall"

// PIDAlive reports whether a process with the given pid exists.
func PIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}
