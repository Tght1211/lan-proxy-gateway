//go:build !windows

package cmd

import (
	"os"
	"os/exec"

	"github.com/tght/lan-proxy-gateway/internal/ui"
)

func checkRoot() {
	if os.Geteuid() != 0 {
		if os.Getenv("GATEWAY_ELEVATED") == "" {
			args := append([]string{"-n", os.Args[0]}, os.Args[1:]...)
			cmd := exec.Command("sudo", args...)
			cmd.Env = append(os.Environ(), "GATEWAY_ELEVATED=1")
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err == nil {
				os.Exit(0)
			}
		}
		ui.Error("此操作需要 root 权限。可使用 sudo，或运行 gateway permission print / install 配置免密控制")
		os.Exit(1)
	}
}
