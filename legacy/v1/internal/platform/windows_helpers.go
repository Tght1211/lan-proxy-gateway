package platform

import (
	"fmt"
	"strings"
)

type windowsDefaultRoute struct {
	Gateway     string
	InterfaceIP string
}

func parseWindowsDefaultRoute(output string) (windowsDefaultRoute, bool) {
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		if fields[0] != "0.0.0.0" || fields[1] != "0.0.0.0" {
			continue
		}
		if !looksLikeIPv4(fields[2]) || !looksLikeIPv4(fields[3]) {
			continue
		}
		return windowsDefaultRoute{
			Gateway:     fields[2],
			InterfaceIP: fields[3],
		}, true
	}
	return windowsDefaultRoute{}, false
}

func looksLikeIPv4(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		n := 0
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
			n = n*10 + int(r-'0')
		}
		if n < 0 || n > 255 {
			return false
		}
	}
	return true
}

func buildWindowsStartupTaskCommand(cfg ServiceConfig) string {
	return fmt.Sprintf(`"%s" start --config "%s" --data-dir "%s"`, cfg.BinaryPath, cfg.ConfigFile, cfg.DataDir)
}
