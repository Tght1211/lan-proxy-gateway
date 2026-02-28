//go:build windows

package cmd

import (
	"os"
	"os/exec"

	"github.com/tght/lan-proxy-gateway/internal/ui"
)

func checkRoot() {
	err := exec.Command("net", "session").Run()
	if err != nil {
		ui.Error("此操作需要管理员权限，请以管理员身份运行")
		os.Exit(1)
	}
}
