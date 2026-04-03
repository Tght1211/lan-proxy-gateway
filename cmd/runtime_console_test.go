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

	totalHeight := lipgloss.Height(m.renderHeader()) + lipgloss.Height(m.renderMain()) + lipgloss.Height(m.renderFooter())
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

func TestTypingSlashInNavFocusIsIgnored(t *testing.T) {
	m := newRuntimeConsoleModel("/tmp/lan-proxy-gateway.log", "192.168.12.100", "en0", "data")
	m.focus = consoleFocusNav

	next, _ := m.Update(tea.KeyPressMsg{Text: "/"})
	updated := next.(runtimeConsoleModel)

	if updated.focus != consoleFocusNav {
		t.Fatalf("expected nav focus to remain after typing slash, got %v", updated.focus)
	}
	if updated.inputValue != "" {
		t.Fatalf("expected input value to stay empty in nav mode, got %q", updated.inputValue)
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

func TestEscFromNavFocusMovesToHeader(t *testing.T) {
	m := newRuntimeConsoleModel("/tmp/lan-proxy-gateway.log", "192.168.12.100", "en0", "data")
	m.focus = consoleFocusNav

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	updated := next.(runtimeConsoleModel)

	if updated.focus != consoleFocusHeader {
		t.Fatalf("expected esc from nav to move focus to header, got %v", updated.focus)
	}
}

func TestDownFromHeaderMovesToNav(t *testing.T) {
	m := newRuntimeConsoleModel("/tmp/lan-proxy-gateway.log", "192.168.12.100", "en0", "data")
	m.focus = consoleFocusHeader

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	updated := next.(runtimeConsoleModel)

	if updated.focus != consoleFocusNav {
		t.Fatalf("expected down from header to move focus to nav, got %v", updated.focus)
	}
}

func TestRenderGuideDetailLinesUsesNativeSections(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Tun.Enabled = true
	cfg.Extension.Mode = "chains"
	cfg.Extension.ResidentialChain = &config.ResidentialChain{Mode: "rule"}

	lines := renderGuideDetailLines(cfg, "/tmp/lan-proxy-gateway.log")
	got := strings.Join(lines, "\n")

	if !strings.Contains(got, "当前主线") || !strings.Contains(got, "常用入口") {
		t.Fatalf("expected guide sections, got %q", got)
	}
	if !strings.Contains(got, "/status") || !strings.Contains(got, "/config open") {
		t.Fatalf("expected guide commands, got %q", got)
	}
	if strings.Contains(got, "──") {
		t.Fatalf("expected native guide layout without separator lines, got %q", got)
	}
}

func TestRefreshKeyStartsPulse(t *testing.T) {
	m := newRuntimeConsoleModel("/tmp/lan-proxy-gateway.log", "192.168.12.100", "en0", "data")

	next, _ := m.Update(tea.KeyPressMsg{Text: "r"})
	updated := next.(runtimeConsoleModel)

	if updated.refreshPulse <= 0 {
		t.Fatalf("expected refresh key to start pulse, got %d", updated.refreshPulse)
	}
}

func TestRefreshKeyKeepsGuidePage(t *testing.T) {
	m := newRuntimeConsoleModel("/tmp/lan-proxy-gateway.log", "192.168.12.100", "en0", "data")
	m.setDetail("功能导航", renderGuideDetailLines(config.DefaultConfig(), "/tmp/lan-proxy-gateway.log"))

	next, _ := m.Update(tea.KeyPressMsg{Text: "r"})
	updated := next.(runtimeConsoleModel)

	if updated.detailTitle != "功能导航" {
		t.Fatalf("expected refresh to preserve current guide page, got %q", updated.detailTitle)
	}
}

func TestEnterMovesFocusToDetailForInfoPage(t *testing.T) {
	m := newRuntimeConsoleModel("/tmp/lan-proxy-gateway.log", "192.168.12.100", "en0", "data")
	m.tab = consoleTabOverview
	m.cursor = 0
	m.focus = consoleFocusNav
	m.refreshSelectionPreview()

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := next.(runtimeConsoleModel)

	if updated.focus != consoleFocusDetail {
		t.Fatalf("expected enter from nav to move focus to detail, got %v", updated.focus)
	}
}

func TestRenderMenuLinesShowsActionTag(t *testing.T) {
	m := runtimeConsoleModel{tab: consoleTabRouting}
	got := strings.Join(m.renderMenuLines(), "\n")

	if !strings.Contains(got, "操作") {
		t.Fatalf("expected routing menu to include action tag, got %q", got)
	}
}

func TestPickerDelayResultUpdatesStatus(t *testing.T) {
	m := newRuntimeConsoleModel("/tmp/lan-proxy-gateway.log", "192.168.12.100", "en0", "data")
	m.picker = pickerModeNodes
	m.nodeDelays = map[string]string{}

	next, _ := m.Update(pickerDelayResultMsg{node: "香港 02", delay: 184})
	updated := next.(runtimeConsoleModel)

	if updated.nodeDelays["香港 02"] != "184ms" {
		t.Fatalf("expected picker delay to be recorded, got %q", updated.nodeDelays["香港 02"])
	}
	if !strings.Contains(updated.pickerStatus, "香港 02") {
		t.Fatalf("expected picker status to mention node, got %q", updated.pickerStatus)
	}
}

func TestLeftFromNavDoesNotChangeTabAndStartsNavAlert(t *testing.T) {
	m := newRuntimeConsoleModel("/tmp/lan-proxy-gateway.log", "192.168.12.100", "en0", "data")
	m.focus = consoleFocusNav
	m.tab = consoleTabRouting

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	updated := next.(runtimeConsoleModel)

	if updated.tab != consoleTabRouting {
		t.Fatalf("expected left from nav to keep current tab, got %v", updated.tab)
	}
	if updated.navAlertPulse <= 0 {
		t.Fatalf("expected left from nav to trigger nav alert pulse, got %d", updated.navAlertPulse)
	}
}

func TestRightFromHeaderStillChangesTab(t *testing.T) {
	m := newRuntimeConsoleModel("/tmp/lan-proxy-gateway.log", "192.168.12.100", "en0", "data")
	m.focus = consoleFocusHeader
	m.tab = consoleTabOverview

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	updated := next.(runtimeConsoleModel)

	if updated.tab != consoleTabRouting {
		t.Fatalf("expected right from header to switch tab, got %v", updated.tab)
	}
}

func TestRenderNavigationCardShowsEscHintDuringNavAlert(t *testing.T) {
	m := newRuntimeConsoleModel("/tmp/lan-proxy-gateway.log", "192.168.12.100", "en0", "data")
	m.focus = consoleFocusNav
	m.navAlertPulse = navAlertFrames
	m.mainHeight = 24

	got := plainText(m.renderNavigationCard(72))
	if !strings.Contains(got, "先按 Esc 回顶部") {
		t.Fatalf("expected nav alert card to show esc hint, got %q", got)
	}
	if !strings.Contains(got, "左右键只在顶部可用") {
		t.Fatalf("expected nav alert card to explain boundary, got %q", got)
	}
}

func TestRefreshSelectionPreviewDoesNotDuplicateDetailTitle(t *testing.T) {
	m := newRuntimeConsoleModel("/tmp/lan-proxy-gateway.log", "192.168.12.100", "en0", "data")
	m.tab = consoleTabOverview
	m.cursor = 0
	m.refreshSelectionPreview()

	got := plainText(m.viewport.GetContent())
	if strings.Contains(got, "运行状态 · 信息页") {
		t.Fatalf("expected preview body to avoid repeating detail title, got %q", got)
	}
	if !strings.Contains(got, "当前摘要") {
		t.Fatalf("expected preview body to include summary section, got %q", got)
	}
}

func TestRefreshSelectionPreviewShowsNextStepSection(t *testing.T) {
	m := newRuntimeConsoleModel("/tmp/lan-proxy-gateway.log", "192.168.12.100", "en0", "data")
	m.tab = consoleTabOverview
	m.cursor = 0
	m.refreshSelectionPreview()

	got := plainText(m.viewport.GetContent())
	if !strings.Contains(got, "下一步") {
		t.Fatalf("expected preview body to include next-step section, got %q", got)
	}
	if !strings.Contains(got, "回车:") || !strings.Contains(got, "命令:") {
		t.Fatalf("expected preview body to show enter and command hints, got %q", got)
	}
	if strings.Contains(got, "回车后:") {
		t.Fatalf("expected preview body to use compact next-step layout, got %q", got)
	}
}

func TestMouseWheelScrollsDetailPaneWhenFocused(t *testing.T) {
	m := newRuntimeConsoleModel("/tmp/lan-proxy-gateway.log", "192.168.12.100", "en0", "data")
	m.width = 120
	m.height = 30
	m.resize()
	m.focus = consoleFocusDetail

	lines := make([]string, 0, 80)
	for i := 0; i < 80; i++ {
		lines = append(lines, "line")
	}
	m.setDetail("长内容", lines)
	start := m.viewport.YOffset()

	next, _ := m.Update(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown}))
	updated := next.(runtimeConsoleModel)

	if updated.viewport.YOffset() <= start {
		t.Fatalf("expected mouse wheel to scroll detail viewport, got start=%d end=%d", start, updated.viewport.YOffset())
	}
}

func TestRenderFooterShowsRefreshHintDuringPulse(t *testing.T) {
	m := newRuntimeConsoleModel("/tmp/lan-proxy-gateway.log", "192.168.12.100", "en0", "data")
	m.width = 120
	m.refreshPulse = refreshPulseFrames

	got := plainText(m.renderHeader())
	if !strings.Contains(got, "↻") {
		t.Fatalf("expected header to show refresh marker during pulse, got %q", got)
	}
}

func TestRenderHeaderShowsRefreshMarker(t *testing.T) {
	m := newRuntimeConsoleModel("/tmp/lan-proxy-gateway.log", "192.168.12.100", "en0", "data")
	m.width = 120
	m.refreshPulse = refreshPulseFrames

	got := plainText(m.renderHeader())
	if !strings.Contains(got, "Gateway Console  ↻") {
		t.Fatalf("expected header to show refresh marker during pulse, got %q", got)
	}
}

func TestHandleCommandTunOffUpdatesRuntimeWorkspace(t *testing.T) {
	m := newRuntimeConsoleModel("/tmp/lan-proxy-gateway.log", "192.168.12.100", "en0", "data")

	next, _ := m.handleCommand("/tun off")
	updated := next.(runtimeConsoleModel)

	if updated.detailTitle != detailTitleRuntimeWorkspace {
		t.Fatalf("expected tun command to open runtime workspace, got %q", updated.detailTitle)
	}
	if updated.cfg.Runtime.Tun.Enabled {
		t.Fatalf("expected tun command to disable TUN")
	}
}

func TestHandleCommandRuleAdsOffUpdatesRulesWorkspace(t *testing.T) {
	m := newRuntimeConsoleModel("/tmp/lan-proxy-gateway.log", "192.168.12.100", "en0", "data")

	next, _ := m.handleCommand("/rule ads off")
	updated := next.(runtimeConsoleModel)

	if updated.detailTitle != detailTitleRulesWorkspace {
		t.Fatalf("expected rule command to open rules workspace, got %q", updated.detailTitle)
	}
	if updated.cfg.Rules.AdsRejectEnabled() {
		t.Fatalf("expected ads rule to be disabled")
	}
}

func TestHandleCommandExtensionChainsUpdatesExtensionWorkspace(t *testing.T) {
	m := newRuntimeConsoleModel("/tmp/lan-proxy-gateway.log", "192.168.12.100", "en0", "data")

	next, _ := m.handleCommand("/extension chains")
	updated := next.(runtimeConsoleModel)

	if updated.detailTitle != detailTitleExtensionWorkspace {
		t.Fatalf("expected extension command to open extension workspace, got %q", updated.detailTitle)
	}
	if updated.cfg.Extension.Mode != "chains" {
		t.Fatalf("expected extension mode to be chains, got %q", updated.cfg.Extension.Mode)
	}
}
