//go:build darwin || linux

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func reexecWithSudo() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	sudo, err := exec.LookPath("sudo")
	if err != nil {
		return fmt.Errorf("未找到 sudo")
	}
	args := append([]string{"sudo", exe}, os.Args[1:]...)
	return syscall.Exec(sudo, args, os.Environ())
}
