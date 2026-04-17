package cmd

import "testing"

func TestPowerShellSingleQuote(t *testing.T) {
	got := powerShellSingleQuote(`C:\tmp\foo'bar.zip`)
	want := `'C:\tmp\foo''bar.zip'`
	if got != want {
		t.Fatalf("powerShellSingleQuote() = %q, want %q", got, want)
	}
}
