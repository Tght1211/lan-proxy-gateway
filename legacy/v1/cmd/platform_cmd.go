package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func followLogCommand(logFile string) string {
	return followLogCommandForPlatform(runtime.GOOS, logFile)
}

func followLogCommandForPlatform(goos, logFile string) string {
	if goos == "windows" {
		quoted := strings.ReplaceAll(logFile, `'`, `''`)
		return fmt.Sprintf(`powershell -NoProfile -Command "Get-Content -Path '%s' -Wait"`, quoted)
	}
	return "tail -f " + logFile
}

func expandUserPath(path string) string {
	if path == "" || path[0] != '~' {
		return path
	}
	if len(path) > 1 && path[1] != '/' && path[1] != '\\' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if len(path) == 1 {
		return home
	}
	return filepath.Join(home, strings.TrimLeft(path[1:], `/\`))
}
