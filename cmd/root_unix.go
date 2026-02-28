//go:build !windows

package cmd

import (
	"os"

	"github.com/tght/lan-proxy-gateway/internal/ui"
)

func checkRoot() {
	if os.Geteuid() != 0 {
		ui.Error("此操作需要 root 权限，请使用 sudo 运行")
		os.Exit(1)
	}
}
