package cmd

import (
	"os"
	"path/filepath"
	"runtime"
)

// elevatedCmd returns the platform-appropriate prefixed gateway command.
// On Windows: "gateway <sub>"  (must be run in an Administrator terminal)
// On Unix:    "sudo gateway <sub>"
func elevatedCmd(sub string) string {
	if runtime.GOOS == "windows" {
		return "gateway " + sub
	}
	return "sudo gateway " + sub
}

// defaultLogFile returns the platform-appropriate log file path.
// Avoids hardcoding "/tmp/" which does not exist on Windows.
func defaultLogFile() string {
	return filepath.Join(os.TempDir(), "lan-proxy-gateway.log")
}
