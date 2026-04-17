package cmd

import (
	"strings"
	"testing"
)

func TestRenderSudoersIncludesUserAndBinary(t *testing.T) {
	rendered := renderSudoers("alice", "/usr/local/bin/gateway")

	if !strings.Contains(rendered, "alice ALL=(root) NOPASSWD: /usr/local/bin/gateway, /usr/local/bin/gateway *") {
		t.Fatalf("rendered sudoers missing expected rule:\n%s", rendered)
	}
}
