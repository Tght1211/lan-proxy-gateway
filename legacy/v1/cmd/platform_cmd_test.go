package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFollowLogCommandForPlatform(t *testing.T) {
	windowsCmd := followLogCommandForPlatform("windows", `C:\Temp\lan-proxy-gateway.log`)
	if !strings.Contains(windowsCmd, "Get-Content") || !strings.Contains(windowsCmd, `C:\Temp\lan-proxy-gateway.log`) {
		t.Fatalf("expected Windows follow command, got %q", windowsCmd)
	}

	unixCmd := followLogCommandForPlatform("darwin", "/tmp/lan-proxy-gateway.log")
	if unixCmd != "tail -f /tmp/lan-proxy-gateway.log" {
		t.Fatalf("expected tail command, got %q", unixCmd)
	}
}

func TestExpandUserPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("user home dir not available")
	}

	cases := map[string]string{
		"~/gateway.yaml": filepath.Join(home, "gateway.yaml"),
		`~\gateway.yaml`: filepath.Join(home, "gateway.yaml"),
		"~":              home,
	}

	for input, want := range cases {
		if got := expandUserPath(input); got != want {
			t.Fatalf("expandUserPath(%q) = %q, want %q", input, got, want)
		}
	}
}
