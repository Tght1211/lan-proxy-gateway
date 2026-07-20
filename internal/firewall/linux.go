//go:build linux

package firewall

import (
	"fmt"
	"os/exec"
	"strings"
)

func newPlatformManager() Manager {
	return &iptablesManager{
		run: func(args ...string) error {
			out, err := exec.Command("iptables", args...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("iptables %s: %v: %s", strings.Join(args, " "), err, out)
			}
			return nil
		},
		save: func(table string) (string, error) {
			out, err := exec.Command("iptables-save", "-t", table).CombinedOutput()
			if err != nil {
				return "", fmt.Errorf("iptables-save -t %s: %v: %s", table, err, out)
			}
			return string(out), nil
		},
	}
}

// DisablePF is a no-op on Linux (pf only exists on macOS).
func DisablePF() error { return nil }
