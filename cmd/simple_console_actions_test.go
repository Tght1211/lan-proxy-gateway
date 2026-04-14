package cmd

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestExpandSimpleWorkspaceShortcut(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Extension.Mode = "chains"
	chain := ensureConsoleChain(cfg)
	chain.Mode = "rule"
	chain.ProxyType = "socks5"

	cases := []struct {
		workspace simpleWorkspace
		raw       string
		want      string
	}{
		{simpleWorkspaceChain, "A", "chain airport"},
		{simpleWorkspaceChain, "T", "chain type http"},
		{simpleWorkspaceExtension, "R", "chain mode global"},
		{simpleWorkspaceRules, "4", "rule nintendo toggle"},
		{simpleWorkspaceRuntime, "1", "tun toggle"},
		{simpleWorkspaceProxy, "2", "proxy source file"},
	}

	for _, tc := range cases {
		if got := expandSimpleWorkspaceShortcut(tc.workspace, tc.raw, cfg); got != tc.want {
			t.Fatalf("expandSimpleWorkspaceShortcut(%q, %q) = %q, want %q", tc.workspace, tc.raw, got, tc.want)
		}
	}
}

func TestHandleSimpleConfigCommandChainShortcutPromptsAndUpdates(t *testing.T) {
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	oldCfgFile := cfgFile
	cfgFile = ""
	defer func() {
		cfgFile = oldCfgFile
	}()

	cfg := config.DefaultConfig()
	cfg.Extension.Mode = "chains"
	chain := ensureConsoleChain(cfg)
	chain.ProxyServer = "1.2.3.4"
	chain.ProxyPort = 443
	chain.ProxyType = "socks5"
	chain.AirportGroup = "Auto"
	if err := config.Save(cfg, filepath.Join(dir, "gateway.yaml")); err != nil {
		t.Fatalf("save config: %v", err)
	}

	workspace := simpleWorkspaceNone
	if action, handled := handleSimpleConfigCommand(bufio.NewReader(strings.NewReader("")), &workspace, "chain"); !handled || action != consoleActionNone {
		t.Fatalf("expected chain workspace to open, handled=%v action=%v", handled, action)
	}
	if workspace != simpleWorkspaceChain {
		t.Fatalf("expected workspace to be chain, got %q", workspace)
	}

	reader := bufio.NewReader(strings.NewReader("Manual Group\n"))
	if action, handled := handleSimpleConfigCommand(reader, &workspace, "A"); !handled || action != consoleActionNone {
		t.Fatalf("expected chain shortcut to be handled, handled=%v action=%v", handled, action)
	}

	updated := loadConfigOrDefault()
	if updated.Extension.ResidentialChain == nil {
		t.Fatalf("expected residential chain to remain configured")
	}
	if updated.Extension.ResidentialChain.AirportGroup != "Manual Group" {
		t.Fatalf("expected airport group to update, got %q", updated.Extension.ResidentialChain.AirportGroup)
	}
}

func TestRenderSimpleHelpLinesDefaultIsGrouped(t *testing.T) {
	lines := renderSimpleHelpLines(false)
	got := strings.Join(outputBlock(strings.Join(lines, "\n")), "\n")

	if !strings.Contains(got, "日常最常用") || !strings.Contains(got, "工作台提示") || !strings.Contains(got, "补充常用") {
		t.Fatalf("expected grouped help sections, got %q", got)
	}
	if !strings.Contains(got, "nodes") || !strings.Contains(got, "proxy") || !strings.Contains(got, "subscription") {
		t.Fatalf("expected default help to focus on common operations, got %q", got)
	}
	if !strings.Contains(got, "help all") {
		t.Fatalf("expected default help to mention help all, got %q", got)
	}
	if strings.Contains(got, "subscription add url|file") {
		t.Fatalf("expected default help to stay concise, got %q", got)
	}
}

func TestRenderSimpleHelpLinesAllIncludesDirectCommands(t *testing.T) {
	lines := renderSimpleHelpLines(true)
	got := strings.Join(outputBlock(strings.Join(lines, "\n")), "\n")

	if !strings.Contains(got, "完整命令") || !strings.Contains(got, "代理来源") {
		t.Fatalf("expected detailed help to include direct commands, got %q", got)
	}
	if !strings.Contains(got, "subscription add url|file") || !strings.Contains(got, "rule <lan|china|apple|nintendo|global|ads>") || !strings.Contains(got, "chain server|port|type|airport|user|password") {
		t.Fatalf("expected detailed help commands, got %q", got)
	}
}
