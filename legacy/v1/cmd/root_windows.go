//go:build windows

package cmd

import (
	"os"

	"github.com/tght/lan-proxy-gateway/internal/ui"
	"golang.org/x/sys/windows"
)

func checkRoot() {
	isAdmin, err := isWindowsAdmin()
	if err != nil || !isAdmin {
		ui.Error("此操作需要管理员权限，请以管理员身份运行")
		os.Exit(1)
	}
}

func isWindowsAdmin() (bool, error) {
	adminSID, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return false, err
	}
	return windows.Token(0).IsMember(adminSID)
}
