//go:build darwin

package platform

import (
	"os/exec"
	"runtime"
	"strings"
)

type impl struct{}

// New returns the Platform implementation for macOS.
func New() Platform { return &impl{} }

// DetectArch returns the CPU architecture.
func DetectArch() string {
	switch runtime.GOARCH {
	case "arm64":
		return "arm64"
	case "amd64":
		return "amd64"
	default:
		out, _ := exec.Command("uname", "-m").Output()
		return strings.TrimSpace(string(out))
	}
}
