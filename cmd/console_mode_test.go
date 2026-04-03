package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func newConsoleModeTestCommand(t *testing.T, args ...string) *cobra.Command {
	t.Helper()

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("simple", false, "")
	cmd.Flags().Bool("tui", false, "")
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	return cmd
}

func TestResolveConsoleSimpleModeDefaultIsSimple(t *testing.T) {
	cmd := newConsoleModeTestCommand(t)

	simple, err := resolveConsoleSimpleMode(cmd, true, false, false)
	if err != nil {
		t.Fatalf("resolve mode: %v", err)
	}
	if !simple {
		t.Fatalf("expected default mode to be simple")
	}
}

func TestResolveConsoleSimpleModeSupportsTUIFlag(t *testing.T) {
	cmd := newConsoleModeTestCommand(t, "--tui")

	simple, err := resolveConsoleSimpleMode(cmd, true, false, true)
	if err != nil {
		t.Fatalf("resolve mode: %v", err)
	}
	if simple {
		t.Fatalf("expected --tui to disable simple mode")
	}
}

func TestResolveConsoleSimpleModeSupportsSimpleFlag(t *testing.T) {
	cmd := newConsoleModeTestCommand(t, "--simple")

	simple, err := resolveConsoleSimpleMode(cmd, true, true, false)
	if err != nil {
		t.Fatalf("resolve mode: %v", err)
	}
	if !simple {
		t.Fatalf("expected --simple to keep simple mode")
	}
}

func TestResolveConsoleSimpleModeRejectsConflictingFlags(t *testing.T) {
	cmd := newConsoleModeTestCommand(t, "--simple", "--tui")

	if _, err := resolveConsoleSimpleMode(cmd, true, true, true); err == nil {
		t.Fatalf("expected conflicting flags to fail")
	}
}
