package cmd

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestRenderInputLineShowsTypedValue(t *testing.T) {
	m := runtimeConsoleModel{
		inputValue:  "/hello",
		inputCursor: len([]rune("/hello")),
		focus:       consoleFocusInput,
	}
	line := m.renderInputLine(80)

	if !strings.Contains(line, "/hello") {
		t.Fatalf("expected rendered input to contain typed value, got %q", line)
	}
}

func TestRenderInputLineShowsPlaceholderWhenEmpty(t *testing.T) {
	m := runtimeConsoleModel{}
	line := m.renderInputLine(80)

	if !strings.Contains(line, "/status") {
		t.Fatalf("expected rendered input to contain placeholder, got %q", line)
	}
}

func TestMatchingSuggestionsUsesNodesAlias(t *testing.T) {
	m := runtimeConsoleModel{tab: consoleTabRouting}
	suggestions := m.matchingSuggestions("/no")

	if len(suggestions) == 0 || suggestions[0] != "/nodes" {
		t.Fatalf("expected /nodes suggestion first, got %#v", suggestions)
	}
}

func TestConsoleLayoutFitsWindowHeight(t *testing.T) {
	m := newRuntimeConsoleModel("/tmp/lan-proxy-gateway.log", "192.168.12.100", "en0", "data")
	m.width = 140
	m.height = 38
	m.resize()

	totalHeight := lipgloss.Height(m.renderHeader()) + lipgloss.Height(m.renderMain()) + lipgloss.Height(m.renderInput())
	if totalHeight > m.height {
		t.Fatalf("expected console to fit window height, got total=%d height=%d", totalHeight, m.height)
	}
}

func TestOutputBlockRemovesSeparators(t *testing.T) {
	lines := outputBlock("\n────────────────────\n  配置来源\n────────────────────\n  配置文件: /tmp/gateway.yaml\n")

	got := strings.Join(lines, "\n")
	if strings.Contains(got, "──") {
		t.Fatalf("expected separators to be removed, got %q", got)
	}
	if !strings.Contains(got, "配置来源") || !strings.Contains(got, "配置文件: /tmp/gateway.yaml") {
		t.Fatalf("expected cleaned content to remain, got %q", got)
	}
}

func TestRenderConfigSummaryDetailLinesHasSections(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Proxy.Source = "url"
	cfg.Proxy.SubscriptionName = "demo"
	cfg.Proxy.SubscriptionURL = "https://example.com/sub"
	cfg.Extension.Mode = "chains"
	cfg.Extension.ResidentialChain = &config.ResidentialChain{Mode: "rule", AirportGroup: "Auto"}

	lines := renderConfigSummaryDetailLines(cfg)
	got := strings.Join(lines, "\n")

	if !strings.Contains(got, "配置来源") || !strings.Contains(got, "运行模式") || !strings.Contains(got, "规则开关") {
		t.Fatalf("expected TUI summary sections, got %q", got)
	}
	if strings.Contains(got, "──") {
		t.Fatalf("expected no separator lines in TUI summary, got %q", got)
	}
}

func TestTypingFromNavFocusMovesToCommandInput(t *testing.T) {
	m := newRuntimeConsoleModel("/tmp/lan-proxy-gateway.log", "192.168.12.100", "en0", "data")

	next, _ := m.Update(tea.KeyPressMsg{Text: "/"})
	updated := next.(runtimeConsoleModel)

	if updated.focus != consoleFocusInput {
		t.Fatalf("expected focus to switch to input, got %v", updated.focus)
	}
	if updated.inputValue != "/" {
		t.Fatalf("expected input value to start with slash, got %q", updated.inputValue)
	}
}

func TestTabCompletionUsesNodesCommand(t *testing.T) {
	m := newRuntimeConsoleModel("/tmp/lan-proxy-gateway.log", "192.168.12.100", "en0", "data")
	m.focus = consoleFocusInput
	m.setInputValue("/no")

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	updated := next.(runtimeConsoleModel)

	if updated.inputValue != "/nodes" {
		t.Fatalf("expected tab completion to use /nodes, got %q", updated.inputValue)
	}
}
