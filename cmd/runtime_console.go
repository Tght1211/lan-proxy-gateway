package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/egress"
	"github.com/tght/lan-proxy-gateway/internal/mihomo"
	"github.com/tght/lan-proxy-gateway/internal/platform"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

type consoleAction int

const (
	consoleActionNone consoleAction = iota
	consoleActionExit
	consoleActionRestart
	consoleActionStop
	consoleActionOpenConfig
	consoleActionOpenChainsSetup
	consoleActionOpenTUI
)

type pendingConfirm struct {
	prompt string
	action consoleAction
}

type consoleTab int

const (
	consoleTabOverview consoleTab = iota
	consoleTabRouting
	consoleTabExtension
	consoleTabDevices
	consoleTabSystem
)

type consoleMenuItem struct {
	id    string
	title string
	desc  string
	key   string
}

type pickerMode int

const (
	pickerModeNone pickerMode = iota
	pickerModeGroups
	pickerModeNodes
)

type consoleFocus int

const (
	consoleFocusHeader consoleFocus = iota
	consoleFocusNav
	consoleFocusInput
)

type refreshPulseMsg struct{}

const refreshPulseFrames = 4

type snapshot struct {
	modeSummary   string
	egressSummary string
	panelURL      string
	configPath    string
	iface         string
	currentNode   string
	shareEntry    string
	refreshedAt   string
}

type runtimeConsoleModel struct {
	width      int
	height     int
	mainHeight int
	logFile    string
	ip         string
	iface      string
	dataDir    string
	cfg        *config.Config
	client     *mihomo.Client
	snapshot   snapshot
	update     *updateNotice

	viewport    viewport.Model
	focus       consoleFocus
	inputValue  string
	inputCursor int
	history     []string
	historyPos  int

	action consoleAction

	pending *pendingConfirm

	picker       pickerMode
	groups       []mihomo.ProxyGroup
	groupCursor  int
	nodeCursor   int
	tab          consoleTab
	cursor       int
	detailTitle  string
	refreshPulse int
}

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func runRuntimeConsole(logFile, ip, iface, dataDir string) consoleAction {
	model := newRuntimeConsoleModel(logFile, ip, iface, dataDir)
	program := tea.NewProgram(model)
	finalModel, err := program.Run()
	if err != nil {
		fmt.Println("runtime console error:", err)
		return consoleActionExit
	}

	if m, ok := finalModel.(runtimeConsoleModel); ok {
		return m.action
	}
	return consoleActionExit
}

func newRuntimeConsoleModel(logFile, ip, iface, dataDir string) runtimeConsoleModel {
	cfg := loadConfigOrDefault()
	update := loadUpdateNotice()

	vp := viewport.New()

	m := runtimeConsoleModel{
		logFile:    logFile,
		ip:         ip,
		iface:      iface,
		dataDir:    dataDir,
		cfg:        cfg,
		client:     newConsoleClient(cfg),
		update:     update,
		viewport:   vp,
		tab:        consoleTabOverview,
		focus:      consoleFocusNav,
		historyPos: -1,
	}
	m.refreshSnapshot()
	m.refreshSelectionPreview()
	return m
}

func (m runtimeConsoleModel) Init() tea.Cmd {
	return nil
}

func (m runtimeConsoleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return m, nil

	case refreshPulseMsg:
		if m.refreshPulse > 0 {
			m.refreshPulse--
			if m.refreshPulse > 0 {
				return m, refreshPulseTickCmd()
			}
		}
		return m, nil

	case tea.KeyMsg:
		if m.picker != pickerModeNone {
			return m.handlePickerKey(msg)
		}
		switch msg.String() {
		case "ctrl+c":
			m.action = consoleActionExit
			return m, tea.Quit
		case "ctrl+p":
			return m.openGroupPicker()
		}

		if m.focus == consoleFocusInput {
			return m.handleInputKey(msg)
		}

		switch msg.String() {
		case "esc":
			if m.focus == consoleFocusNav {
				m.focus = consoleFocusHeader
			}
			return m, nil
		case "left":
			m.prevTab()
			m.refreshSelectionPreview()
			return m, nil
		case "right":
			m.nextTab()
			m.refreshSelectionPreview()
			return m, nil
		case "up":
			if m.focus == consoleFocusHeader {
				return m, nil
			}
			m.moveCursor(-1)
			m.refreshSelectionPreview()
			return m, nil
		case "down":
			if m.focus == consoleFocusHeader {
				m.focus = consoleFocusNav
				return m, nil
			}
			m.moveCursor(1)
			m.refreshSelectionPreview()
			return m, nil
		case "r":
			m.refreshSnapshot()
			m.refreshCurrentDetail()
			m.refreshPulse = refreshPulseFrames
			return m, refreshPulseTickCmd()
		case "q":
			m.pending = &pendingConfirm{prompt: "确认退出控制台？", action: consoleActionExit}
			m.focus = consoleFocusInput
			m.setInputValue("")
			m.setDetail("确认退出", []string{
				noteLine("输入 y / n 进行确认。"),
				noteLine("退出控制台不会停止网关。"),
			})
			return m, nil
		case "enter":
			if m.focus == consoleFocusHeader {
				m.focus = consoleFocusNav
				return m, nil
			}
			return m.executeSelectedAction()
		}

		if text := msg.Key().Text; text != "" {
			m.focus = consoleFocusInput
			m.insertInput(text)
			return m, nil
		}
	}

	return m, nil
}

func (m runtimeConsoleModel) View() tea.View {
	if m.width == 0 || m.height == 0 {
		v := tea.NewView("loading...")
		v.AltScreen = true
		return v
	}

	header := m.renderHeader()
	main := m.renderMain()
	input := m.renderInput()

	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, main, input))
	v.AltScreen = true
	return v
}

func (m *runtimeConsoleModel) handleCommand(value string) (tea.Model, tea.Cmd) {
	if value == "" {
		return *m, nil
	}

	if m.pending != nil {
		return m.handleConfirm(value)
	}

	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}

	fields := strings.Fields(strings.TrimPrefix(value, "/"))
	if len(fields) == 0 {
		m.setDetail("命令帮助", []string{noteLine("输入 /help 查看命令。")})
		return *m, nil
	}

	cmd := strings.ToLower(fields[0])
	args := fields[1:]

	switch cmd {
	case "help", "?":
		m.setDetail("命令帮助", []string{
			noteLine("/status        查看完整运行状态"),
			noteLine("/summary       查看配置摘要"),
			noteLine("/config        查看 TUI 配置中心"),
			noteLine("/config open   打开完整交互式配置中心"),
			noteLine("/chains        查看链式代理 / 扩展状态"),
			noteLine("/chains setup  打开链式代理向导"),
			noteLine("/nodes         切换节点（兼容 /groups）"),
			noteLine("/device        查看设备接入说明"),
			noteLine("/logs          查看最近日志"),
			noteLine("/guide         查看功能导航"),
			noteLine("/update        查看升级提示"),
			noteLine("/clear         清空主屏记录"),
			noteLine("/restart       重启网关（需确认）"),
			noteLine("/stop          停止网关（需确认）"),
			noteLine("/exit          退出控制台"),
		})
	case "status":
		m.setDetail("运行状态", m.renderStatusDetailLines())
	case "summary":
		m.setDetail("配置摘要", renderConfigSummaryDetailLines(loadConfigOrDefault()))
	case "config":
		if len(args) > 0 && (args[0] == "open" || args[0] == "cli") {
			m.action = consoleActionOpenConfig
			return *m, tea.Quit
		}
		m.setDetail("配置中心", renderConfigCenterLines(loadConfigOrDefault()))
	case "chains":
		if len(args) > 0 && args[0] == "setup" {
			m.action = consoleActionOpenChainsSetup
			return *m, tea.Quit
		}
		cfg := loadConfigOrDefault()
		if cfg.Extension.Mode == "chains" {
			m.showCapturedDetail("链式代理状态", func() { runChainsStatus(nil, nil) })
		} else {
			m.showCapturedDetail("扩展状态", func() { printExtensionStatus(cfg) })
		}
	case "groups":
		fallthrough
	case "nodes", "node":
		return m.openGroupPicker()
	case "device":
		cfg := loadConfigOrDefault()
		m.showCapturedDetail("设备接入", func() { printDeviceSetupPanel(m.ip, cfg.Runtime.Ports.API) })
	case "logs", "log":
		m.setDetail("最近日志", m.captureLogLines(30))
	case "guide":
		m.setDetail("功能导航", renderGuideDetailLines(loadConfigOrDefault(), m.logFile))
	case "update":
		if m.update == nil {
			m.setDetail("升级提示", []string{noteLine("当前已经是最新版本，或本次未检测到更新。")})
		} else {
			lines := make([]string, 0, len(renderUpdateNoticeLines(m.update)))
			for _, line := range renderUpdateNoticeLines(m.update) {
				lines = append(lines, noteLine(line))
			}
			m.setDetail("升级提示", lines)
		}
	case "clear":
		m.refreshSelectionPreview()
	case "restart":
		m.pending = &pendingConfirm{prompt: "确认重启网关？", action: consoleActionRestart}
		m.setDetail("确认重启", []string{noteLine("等待确认: 输入 y / n")})
	case "stop":
		m.pending = &pendingConfirm{prompt: "确认停止网关？", action: consoleActionStop}
		m.setDetail("确认停止", []string{noteLine("等待确认: 输入 y / n")})
	case "exit", "quit":
		m.pending = &pendingConfirm{prompt: "确认退出控制台？", action: consoleActionExit}
		m.focus = consoleFocusInput
		m.setInputValue("")
		m.setDetail("确认退出", []string{
			noteLine("输入 y / n 进行确认。"),
			noteLine("退出控制台不会停止网关。"),
		})
	default:
		m.setDetail("命令错误", []string{errorLine("未识别的命令。输入 /help 查看可用命令。")})
	}

	m.refreshSnapshot()
	return *m, nil
}

func (m *runtimeConsoleModel) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.focus = consoleFocusNav
		return *m, nil
	case "enter":
		value := strings.TrimSpace(m.inputValue)
		m.pushHistory(value)
		m.setInputValue("")
		m.historyPos = -1
		if value == "" {
			return *m, nil
		}
		return m.handleCommand(value)
	case "left":
		if m.inputCursor > 0 {
			m.inputCursor--
		}
		return *m, nil
	case "right":
		if m.inputCursor < len([]rune(m.inputValue)) {
			m.inputCursor++
		}
		return *m, nil
	case "home", "ctrl+a":
		m.inputCursor = 0
		return *m, nil
	case "end", "ctrl+e":
		m.inputCursor = len([]rune(m.inputValue))
		return *m, nil
	case "backspace":
		m.deleteBeforeCursor()
		return *m, nil
	case "delete":
		m.deleteAtCursor()
		return *m, nil
	case "ctrl+u":
		m.setInputValue("")
		m.historyPos = -1
		return *m, nil
	case "up":
		m.recallHistory(-1)
		return *m, nil
	case "down":
		m.recallHistory(1)
		return *m, nil
	case "tab":
		matches := m.matchingSuggestions(m.inputValue)
		if len(matches) > 0 {
			m.setInputValue(matches[0])
		}
		return *m, nil
	}

	if text := msg.Key().Text; text != "" {
		m.insertInput(text)
		return *m, nil
	}

	return *m, nil
}

func (m *runtimeConsoleModel) focusHint() string {
	if m.pending != nil {
		return "确认操作中，输入 y / n"
	}
	if m.picker != pickerModeNone {
		return "节点选择中，↑/↓ 选择，Enter 确认，Esc 返回"
	}
	if m.focus == consoleFocusInput {
		return "输入模式：Enter 执行，Tab 补全，Esc 返回导航"
	}
	if m.focus == consoleFocusHeader {
		return "顶部聚焦：←/→ 切换分区，↓ / Enter 进入功能列表，/ 开始输入命令"
	}
	return "导航模式：←/→ 分区，↑/↓ 功能，/ 开始输入命令"
}

func refreshPulseTickCmd() tea.Cmd {
	return tea.Tick(45*time.Millisecond, func(time.Time) tea.Msg {
		return refreshPulseMsg{}
	})
}

func (m runtimeConsoleModel) refreshPulseActive() bool {
	return m.refreshPulse > 0
}

func (m runtimeConsoleModel) refreshPulseOffset() int {
	if !m.refreshPulseActive() {
		return 0
	}
	if m.refreshPulse%2 == 0 {
		return 1
	}
	return 0
}

func (m *runtimeConsoleModel) setInputValue(value string) {
	m.inputValue = value
	m.inputCursor = len([]rune(value))
}

func (m *runtimeConsoleModel) insertInput(text string) {
	if text == "" {
		return
	}
	runes := []rune(m.inputValue)
	if m.inputCursor < 0 {
		m.inputCursor = 0
	}
	if m.inputCursor > len(runes) {
		m.inputCursor = len(runes)
	}
	insert := []rune(text)
	runes = append(runes[:m.inputCursor], append(insert, runes[m.inputCursor:]...)...)
	m.inputValue = string(runes)
	m.inputCursor += len(insert)
}

func (m *runtimeConsoleModel) deleteBeforeCursor() {
	runes := []rune(m.inputValue)
	if m.inputCursor <= 0 || len(runes) == 0 {
		return
	}
	runes = append(runes[:m.inputCursor-1], runes[m.inputCursor:]...)
	m.inputValue = string(runes)
	m.inputCursor--
}

func (m *runtimeConsoleModel) deleteAtCursor() {
	runes := []rune(m.inputValue)
	if m.inputCursor < 0 || m.inputCursor >= len(runes) {
		return
	}
	runes = append(runes[:m.inputCursor], runes[m.inputCursor+1:]...)
	m.inputValue = string(runes)
}

func (m *runtimeConsoleModel) pushHistory(value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if len(m.history) > 0 && m.history[len(m.history)-1] == value {
		return
	}
	m.history = append(m.history, value)
	if len(m.history) > 50 {
		m.history = m.history[len(m.history)-50:]
	}
}

func (m *runtimeConsoleModel) recallHistory(delta int) {
	if len(m.history) == 0 {
		return
	}
	if m.historyPos == -1 {
		if delta < 0 {
			m.historyPos = len(m.history) - 1
		} else {
			return
		}
	} else {
		m.historyPos += delta
		if m.historyPos < 0 {
			m.historyPos = 0
		}
		if m.historyPos >= len(m.history) {
			m.historyPos = -1
			m.setInputValue("")
			return
		}
	}
	m.setInputValue(m.history[m.historyPos])
}

func (m runtimeConsoleModel) matchingSuggestions(value string) []string {
	query := strings.ToLower(strings.TrimSpace(value))
	if query == "" {
		return defaultSuggestionsForTab(m.tab)
	}
	if !strings.HasPrefix(query, "/") {
		query = "/" + query
	}

	matches := make([]string, 0, 4)
	for _, item := range dedupeSuggestions(consoleCommandSuggestions()) {
		if strings.HasPrefix(strings.ToLower(item), query) {
			matches = append(matches, item)
		}
		if len(matches) >= 4 {
			break
		}
	}
	return matches
}

func (m *runtimeConsoleModel) handleConfirm(value string) (tea.Model, tea.Cmd) {
	answer := strings.ToLower(strings.TrimSpace(value))

	if answer != "y" && answer != "yes" {
		m.pending = nil
		m.focus = consoleFocusNav
		m.setDetail("已取消", []string{noteLine("已取消。")})
		return *m, nil
	}

	action := m.pending.action
	m.pending = nil
	switch action {
	case consoleActionRestart:
		m.setDetail("重启网关", []string{successLine("准备重启网关...")})
		m.action = consoleActionRestart
	case consoleActionExit:
		m.setDetail("退出控制台", []string{successLine("准备退出控制台...")})
		m.action = consoleActionExit
	case consoleActionStop:
		m.setDetail("停止网关", []string{successLine("准备停止网关...")})
		m.action = consoleActionStop
	}
	return *m, tea.Quit
}

func (m runtimeConsoleModel) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.picker == pickerModeNodes {
			m.picker = pickerModeGroups
			return m, nil
		}
		m.picker = pickerModeNone
		return m, nil
	case "up", "k":
		if m.picker == pickerModeGroups && m.groupCursor > 0 {
			m.groupCursor--
		}
		if m.picker == pickerModeNodes && m.nodeCursor > 0 {
			m.nodeCursor--
		}
	case "down", "j":
		if m.picker == pickerModeGroups && m.groupCursor < len(m.groups)-1 {
			m.groupCursor++
		}
		if m.picker == pickerModeNodes && m.groupCursor < len(m.groups) {
			current := m.groups[m.groupCursor]
			if m.nodeCursor < len(current.All)-1 {
				m.nodeCursor++
			}
		}
	case "enter":
		if m.picker == pickerModeGroups {
			if len(m.groups) == 0 {
				return m, nil
			}
			m.picker = pickerModeNodes
			m.nodeCursor = 0
			return m, nil
		}
		if m.picker == pickerModeNodes {
			if len(m.groups) == 0 {
				return m, nil
			}
			group := m.groups[m.groupCursor]
			if len(group.All) == 0 {
				return m, nil
			}
			target := group.All[m.nodeCursor]
			if err := m.client.SelectProxy(group.Name, target); err != nil {
				m.setDetail("节点切换失败", []string{errorLine("切换失败: " + err.Error())})
			} else {
				m.setDetail("节点已切换", []string{successLine(fmt.Sprintf("已切换节点: %s -> %s", group.Name, target))})
			}
			m.picker = pickerModeNone
			m.refreshSnapshot()
			m.refreshSelectionPreview()
			return m, nil
		}
	}

	return m, nil
}

func (m *runtimeConsoleModel) openGroupPicker() (tea.Model, tea.Cmd) {
	groups, err := m.client.ListProxyGroups()
	if err != nil {
		m.setDetail("节点分组读取失败", []string{errorLine("无法读取节点分组: " + err.Error())})
		return *m, nil
	}
	if len(groups) == 0 {
		m.setDetail("节点切换器", []string{noteLine("当前没有可切换的节点分组。")})
		return *m, nil
	}

	m.groups = groups
	m.groupCursor = 0
	m.nodeCursor = 0
	m.picker = pickerModeGroups
	return *m, nil
}

func (m *runtimeConsoleModel) refreshSnapshot() {
	m.cfg = loadConfigOrDefault()
	m.client = newConsoleClient(m.cfg)

	report := egress.Collect(m.cfg, m.dataDir, m.client)
	m.snapshot = snapshot{
		modeSummary:   compactModeSummary(m.cfg),
		egressSummary: compactEgressSummary(m.cfg, report),
		panelURL:      fmt.Sprintf("http://%s:%d/ui", m.ip, m.cfg.Runtime.Ports.API),
		configPath:    displayConfigPath(),
		iface:         m.iface,
		shareEntry:    m.ip,
		currentNode:   "未知",
		refreshedAt:   time.Now().Format("15:04:05"),
	}
	if pg, err := m.client.GetProxyGroup("Proxy"); err == nil && strings.TrimSpace(pg.Now) != "" {
		m.snapshot.currentNode = pg.Now
	}
}

func (m *runtimeConsoleModel) resize() {
	headerHeight := lipgloss.Height(m.renderHeader())
	inputHeight := lipgloss.Height(m.renderInput())
	mainHeight := m.height - headerHeight - inputHeight
	if mainHeight < 8 {
		mainHeight = 8
	}
	m.mainHeight = mainHeight

	detailWidth := m.detailPaneWidth()
	m.viewport.SetWidth(max(24, detailWidth-4))
	m.viewport.SetHeight(max(5, mainHeight-4))
}

func (m runtimeConsoleModel) renderHeader() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#f8fafc"))
	subStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#94a3b8"))

	title := "Gateway Console"
	if m.refreshPulseActive() {
		title += "  ↻"
	}
	line1 := titleStyle.Render(title)
	line2 := m.renderTabs()
	line3 := subStyle.Render(m.renderHeaderSummary())
	line4 := subStyle.Render(activeTabDescription(m.tab) + "  ·  " + m.focusHint())

	border := lipgloss.Color("#334155")
	if m.focus == consoleFocusHeader && m.picker == pickerModeNone {
		border = lipgloss.Color("#38bdf8")
	}
	if m.refreshPulseActive() {
		border = lipgloss.Color("#22d3ee")
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(max(36, m.width-2)).
		Render(lipgloss.JoinVertical(lipgloss.Left, line1, line2, line3, line4))
}

func (m runtimeConsoleModel) renderMain() string {
	menuWidth := 30
	if m.width < 120 {
		menuWidth = max(24, m.width/3)
	}
	detailWidth := max(38, m.width-menuWidth-3)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderNavigationCard(menuWidth),
		m.renderDetailPane(detailWidth),
	)
}

func (m runtimeConsoleModel) renderDetailPane(width int) string {
	title := m.detailTitle
	if title == "" {
		title = "当前内容"
	}
	content := m.viewport.View()
	border := "#334155"
	if m.picker != pickerModeNone {
		title = "节点选择器"
		content = m.renderPicker(width-4, max(3, m.mainHeight-4))
		border = "#f59e0b"
	}
	if m.refreshPulseActive() && m.picker == pickerModeNone {
		border = "#22d3ee"
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(border)).
		Padding(0, 1).
		Width(width)

	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#e2e8f0")).Render(title)
	bodyHeight := max(3, m.mainHeight-4)
	body := lipgloss.NewStyle().
		MaxWidth(width - 4).
		Height(bodyHeight).
		MaxHeight(bodyHeight).
		Render(content)
	if m.refreshPulseActive() && m.picker == pickerModeNone {
		body = lipgloss.NewStyle().
			PaddingLeft(m.refreshPulseOffset()).
			Render(body)
	}

	return box.Render(lipgloss.JoinVertical(lipgloss.Left, header, "", body))
}

func (m runtimeConsoleModel) renderNavigationCard(width int) string {
	if width <= 0 {
		return ""
	}

	lines := []string{
		renderSectionTitle(tabLabel(m.tab)),
	}
	lines = append(lines, m.renderMenuLines()...)
	lines = append(lines,
		"",
		renderSectionTitle("快捷键"),
		"/ 进入命令栏",
		"Ctrl+P 切节点",
		"R 刷新摘要",
		"Q 退出控制台（确认）",
	)

	title := "导航区"
	border := "#334155"
	if m.focus == consoleFocusNav && m.picker == pickerModeNone {
		title = "导航区 · 当前聚焦"
		border = "#38bdf8"
	}
	card := m.renderCard(width, title, lines, border)
	return lipgloss.NewStyle().
		Height(max(8, m.mainHeight)).
		MaxHeight(max(8, m.mainHeight)).
		Render(card)
}

func (m runtimeConsoleModel) renderCard(width int, title string, lines []string, borderColor string) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7dd3fc"))

	renderedLines := make([]string, 0, len(lines))
	for _, line := range lines {
		renderedLines = append(renderedLines, lipgloss.NewStyle().MaxWidth(width-4).Render(line))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(0, 1).
		Width(width)

	return box.Render(lipgloss.JoinVertical(lipgloss.Left, titleStyle.Render(title), "", strings.Join(renderedLines, "\n")))
}

func (m runtimeConsoleModel) renderInput() string {
	title := "命令栏 · 导航模式"
	border := lipgloss.Color("#334155")
	if m.focus == consoleFocusInput {
		title = "命令栏 · 输入模式"
		border = lipgloss.Color("#38bdf8")
	}
	if m.pending != nil {
		title = "命令栏 · 等待确认"
		border = lipgloss.Color("#f59e0b")
	}
	if m.picker != pickerModeNone {
		title = "命令栏 · 节点选择"
		border = lipgloss.Color("#f59e0b")
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(max(36, m.width-2))

	head := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#e2e8f0")).Render(title)
	inputLine := m.renderInputLine(max(24, m.width-10))
	hints := lipgloss.NewStyle().Foreground(lipgloss.Color("#94a3b8")).Render(m.renderCommandSuggestions())
	return box.Render(lipgloss.JoinVertical(lipgloss.Left, head, inputLine, hints))
}

func (m runtimeConsoleModel) renderTabs() string {
	tabs := []consoleTab{consoleTabOverview, consoleTabRouting, consoleTabExtension, consoleTabDevices, consoleTabSystem}
	parts := make([]string, 0, len(tabs))
	for _, tab := range tabs {
		style := lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(lipgloss.Color("#94a3b8")).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(lipgloss.Color("#334155"))
		if tab == m.tab {
			style = style.
				Bold(true).
				Foreground(lipgloss.Color("#f8fafc")).
				Background(lipgloss.Color("#1e293b")).
				BorderForeground(lipgloss.Color("#38bdf8"))
			if m.focus == consoleFocusHeader {
				style = style.Background(lipgloss.Color("#0f172a"))
			}
		}
		parts = append(parts, style.Render(tabLabel(tab)))
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, parts...)
}

func (m runtimeConsoleModel) renderMenuLines() []string {
	items := menuItemsForTab(m.tab)
	lines := make([]string, 0, len(items)+1)
	for i, item := range items {
		label := fmt.Sprintf("%s %s", item.key, item.title)
		if i == m.cursor {
			label = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7dd3fc")).Render("› " + label)
		} else {
			label = lipgloss.NewStyle().Foreground(lipgloss.Color("#cbd5e1")).Render("  " + label)
		}
		lines = append(lines, label)
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#64748b")).Render("    "+item.desc))
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#94a3b8")).Render("Enter 执行  ·  Ctrl+P 切节点"))
	return lines
}

func (m runtimeConsoleModel) renderCommandSuggestions() string {
	if m.pending != nil {
		return m.pending.prompt + "  ·  输入 y / n"
	}
	if m.picker != pickerModeNone {
		return "↑/↓ 选择  ·  Enter 确认  ·  Esc 返回"
	}

	matches := m.matchingSuggestions(m.inputValue)
	if len(matches) == 0 {
		matches = defaultSuggestionsForTab(m.tab)
	}

	if m.focus == consoleFocusInput || strings.TrimSpace(m.inputValue) != "" {
		return truncateText("Tab 补全  ·  Enter 执行  ·  Esc 返回导航  ·  建议: "+strings.Join(matches, "   "), max(30, m.width-10))
	}
	if m.focus == consoleFocusHeader {
		return truncateText("顶部区域已聚焦  ·  ←/→ 切换分区  ·  ↓ 进入功能列表  ·  / 开始输入命令", max(30, m.width-10))
	}

	return truncateText("按 / 开始输入命令  ·  ↑/↓ 选择功能  ·  Esc 回顶部  ·  Ctrl+P 切换节点", max(30, m.width-10))
}

func (m runtimeConsoleModel) renderHeaderSummary() string {
	parts := []string{
		"入口 " + truncateText(m.snapshot.shareEntry, 18),
		"节点 " + truncateText(m.snapshot.currentNode, 16),
		"模式 " + truncateText(plainText(m.snapshot.modeSummary), 24),
		"出口 " + truncateText(plainText(m.snapshot.egressSummary), 42),
		"刷新 " + m.snapshot.refreshedAt,
	}
	return truncateText(strings.Join(parts, "  ·  "), max(40, m.width-10))
}

func (m runtimeConsoleModel) renderInputLine(width int) string {
	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7dd3fc")).Bold(true)
	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e5e7eb"))
	placeholderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#64748b"))
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f8fafc")).Background(lipgloss.Color("#2563eb"))

	prompt := "› "
	value := m.inputValue
	cursorPos := m.inputCursor
	if cursorPos < 0 {
		cursorPos = 0
	}

	if value == "" {
		line := promptStyle.Render(prompt) + placeholderStyle.Render("例如 /status、/nodes、/chains setup")
		return lipgloss.NewStyle().MaxWidth(width).Render(line)
	}

	runes := []rune(value)
	if cursorPos > len(runes) {
		cursorPos = len(runes)
	}

	before := string(runes[:cursorPos])
	after := string(runes[cursorPos:])
	cursorGlyph := " "
	if cursorPos < len(runes) {
		cursorGlyph = string(runes[cursorPos])
		after = string(runes[cursorPos+1:])
	}

	line := promptStyle.Render(prompt) + textStyle.Render(before)
	if m.focus == consoleFocusInput {
		line += cursorStyle.Render(cursorGlyph)
	} else {
		line += textStyle.Render(cursorGlyph)
	}
	line += textStyle.Render(after)

	return lipgloss.NewStyle().MaxWidth(width).Render(line)
}

func (m runtimeConsoleModel) detailPaneWidth() int {
	width := int(float64(m.width) * 0.7)
	if width > m.width-30 {
		width = m.width - 30
	}
	if width < 60 {
		width = 60
	}
	return width
}

func renderSectionTitle(title string) string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7dd3fc")).
		Render(title)
}

func (m *runtimeConsoleModel) nextTab() {
	m.tab = (m.tab + 1) % 5
	m.cursor = 0
}

func (m *runtimeConsoleModel) prevTab() {
	if m.tab == 0 {
		m.tab = 4
	} else {
		m.tab--
	}
	m.cursor = 0
}

func (m *runtimeConsoleModel) moveCursor(delta int) {
	items := menuItemsForTab(m.tab)
	if len(items) == 0 {
		m.cursor = 0
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = len(items) - 1
	}
	if m.cursor >= len(items) {
		m.cursor = 0
	}
}

func (m runtimeConsoleModel) selectedItem() consoleMenuItem {
	items := menuItemsForTab(m.tab)
	if len(items) == 0 {
		return consoleMenuItem{}
	}
	if m.cursor < 0 || m.cursor >= len(items) {
		return items[0]
	}
	return items[m.cursor]
}

func (m *runtimeConsoleModel) refreshSelectionPreview() {
	item := m.selectedItem()
	if item.title == "" {
		m.setDetail("功能详情", []string{noteLine("当前没有可用功能。")})
		return
	}

	lines := []string{
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7dd3fc")).Render(item.title),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#94a3b8")).Render(item.desc),
		"",
	}
	lines = append(lines, previewLinesForItem(item, m.snapshot, m.cfg, m.logFile)...)
	m.setDetail(item.title, lines)
}

func (m *runtimeConsoleModel) refreshCurrentDetail() {
	cfg := loadConfigOrDefault()
	m.cfg = cfg

	switch m.detailTitle {
	case "运行状态":
		m.setDetail("运行状态", m.renderStatusDetailLines())
	case "配置摘要":
		m.setDetail("配置摘要", renderConfigSummaryDetailLines(cfg))
	case "配置中心":
		m.setDetail("配置中心", renderConfigCenterLines(cfg))
	case "功能导航":
		m.setDetail("功能导航", renderGuideDetailLines(cfg, m.logFile))
	case "最近日志":
		m.setDetail("最近日志", m.captureLogLines(30))
	case "设备接入":
		m.showCapturedDetail("设备接入", func() { printDeviceSetupPanel(m.ip, cfg.Runtime.Ports.API) })
	case "链式代理状态":
		m.showCapturedDetail("链式代理状态", func() { runChainsStatus(nil, nil) })
	case "扩展状态":
		m.showCapturedDetail("扩展状态", func() { printExtensionStatus(cfg) })
	case "升级提示":
		if m.update == nil {
			m.setDetail("升级提示", []string{noteLine("当前已经是最新版本，或本次未检测到更新。")})
		} else {
			lines := make([]string, 0, len(renderUpdateNoticeLines(m.update)))
			for _, line := range renderUpdateNoticeLines(m.update) {
				lines = append(lines, noteLine(line))
			}
			m.setDetail("升级提示", lines)
		}
	default:
		m.refreshSelectionPreview()
	}
}

func (m *runtimeConsoleModel) setDetail(title string, lines []string) {
	m.detailTitle = title
	m.viewport.SetContent(strings.Join(lines, "\n"))
	m.viewport.GotoTop()
}

func (m *runtimeConsoleModel) showCapturedDetail(title string, fn func()) {
	lines := outputBlock(m.capture(fn))
	if len(lines) == 0 {
		lines = []string{noteLine("没有可展示的输出。")}
	}
	m.setDetail(title, lines)
}

func (m *runtimeConsoleModel) executeSelectedAction() (tea.Model, tea.Cmd) {
	item := m.selectedItem()
	switch item.id {
	case "overview_status":
		m.setDetail("运行状态", m.renderStatusDetailLines())
	case "overview_summary":
		m.setDetail("配置摘要", renderConfigSummaryDetailLines(loadConfigOrDefault()))
	case "overview_config":
		m.setDetail("配置中心", renderConfigCenterLines(loadConfigOrDefault()))
	case "overview_guide":
		m.setDetail("功能导航", renderGuideDetailLines(loadConfigOrDefault(), m.logFile))
	case "routing_groups":
		return m.openGroupPicker()
	case "routing_egress":
		m.setDetail("出口网络", previewLinesForItem(item, m.snapshot, m.cfg, m.logFile))
	case "routing_logs":
		m.setDetail("最近日志", m.captureLogLines(30))
	case "routing_panel":
		m.setDetail("管理面板", previewLinesForItem(item, m.snapshot, m.cfg, m.logFile))
	case "extension_status":
		cfg := loadConfigOrDefault()
		if cfg.Extension.Mode == "chains" {
			m.showCapturedDetail("链式代理状态", func() { runChainsStatus(nil, nil) })
		} else {
			m.showCapturedDetail("扩展状态", func() { printExtensionStatus(cfg) })
		}
	case "extension_setup":
		m.action = consoleActionOpenChainsSetup
		return *m, tea.Quit
	case "extension_update":
		if m.update == nil {
			m.setDetail("升级提示", []string{noteLine("当前已经是最新版本，或本次未检测到更新。")})
		} else {
			lines := make([]string, 0, len(renderUpdateNoticeLines(m.update)))
			for _, line := range renderUpdateNoticeLines(m.update) {
				lines = append(lines, noteLine(line))
			}
			m.setDetail("升级提示", lines)
		}
	case "extension_restart":
		m.pending = &pendingConfirm{prompt: "确认重启网关？", action: consoleActionRestart}
		m.setDetail("确认重启", []string{noteLine("等待确认: 输入 y / n，或继续用方向键查看其他内容。")})
	case "devices_setup":
		cfg := loadConfigOrDefault()
		m.showCapturedDetail("设备接入", func() { printDeviceSetupPanel(m.ip, cfg.Runtime.Ports.API) })
	case "devices_mobile":
		m.setDetail("手机 / 平板", previewLinesForItem(item, m.snapshot, m.cfg, m.logFile))
	case "devices_console":
		m.setDetail("游戏机 / 电视", previewLinesForItem(item, m.snapshot, m.cfg, m.logFile))
	case "devices_entry":
		m.setDetail("共享入口", previewLinesForItem(item, m.snapshot, m.cfg, m.logFile))
	case "system_stop":
		m.pending = &pendingConfirm{prompt: "确认停止网关？", action: consoleActionStop}
		m.setDetail("确认停止", []string{noteLine("等待确认: 输入 y / n，或继续用方向键查看其他内容。")})
	case "system_exit":
		m.action = consoleActionExit
		return *m, tea.Quit
	case "system_paths":
		m.setDetail("运行路径", previewLinesForItem(item, m.snapshot, m.cfg, m.logFile))
	case "system_config":
		m.setDetail("配置中心", renderConfigCenterLines(loadConfigOrDefault()))
	default:
		m.refreshSelectionPreview()
	}
	return *m, nil
}

func menuItemsForTab(tab consoleTab) []consoleMenuItem {
	switch tab {
	case consoleTabRouting:
		return []consoleMenuItem{
			{id: "routing_groups", key: "01", title: "切换节点", desc: "打开节点分组和节点选择器"},
			{id: "routing_egress", key: "02", title: "出口网络", desc: "查看入口、普通出口和住宅出口"},
			{id: "routing_logs", key: "03", title: "最近日志", desc: "读取最近 30 行运行日志"},
			{id: "routing_panel", key: "04", title: "管理面板", desc: "查看 Web 面板入口和用途"},
		}
	case consoleTabExtension:
		return []consoleMenuItem{
			{id: "extension_status", key: "01", title: "扩展状态", desc: "查看 chains / script 当前状态"},
			{id: "extension_setup", key: "02", title: "Chains 向导", desc: "进入链式代理配置向导"},
			{id: "extension_update", key: "03", title: "升级提示", desc: "检查是否有新版本可用"},
			{id: "extension_restart", key: "04", title: "重启网关", desc: "应用配置变更并重启服务"},
		}
	case consoleTabDevices:
		return []consoleMenuItem{
			{id: "devices_setup", key: "01", title: "设备接入", desc: "展示网关和 DNS 的填写方式"},
			{id: "devices_mobile", key: "02", title: "手机 / 平板", desc: "iPhone / Android 快速接入提示"},
			{id: "devices_console", key: "03", title: "游戏机 / 电视", desc: "Switch / PS5 / Apple TV / 电视接入提示"},
			{id: "devices_entry", key: "04", title: "共享入口", desc: "查看当前局域网共享入口信息"},
		}
	case consoleTabSystem:
		return []consoleMenuItem{
			{id: "system_config", key: "01", title: "TUI 配置中心", desc: "在 TUI 内查看当前配置与快捷入口"},
			{id: "system_paths", key: "02", title: "运行路径", desc: "查看配置、日志和面板路径"},
			{id: "system_stop", key: "03", title: "停止网关", desc: "停止当前运行中的网关"},
			{id: "system_exit", key: "04", title: "退出控制台", desc: "退出 TUI，但保持网关继续运行"},
		}
	default:
		return []consoleMenuItem{
			{id: "overview_status", key: "01", title: "运行状态", desc: "查看完整运行状态和出口网络"},
			{id: "overview_summary", key: "02", title: "配置摘要", desc: "查看当前配置摘要和生效路径"},
			{id: "overview_config", key: "03", title: "配置中心", desc: "在 TUI 内查看配置中心和快捷入口"},
			{id: "overview_guide", key: "04", title: "功能导航", desc: "查看核心能力和下一步建议"},
		}
	}
}

func previewLinesForItem(item consoleMenuItem, snap snapshot, cfg *config.Config, logFile string) []string {
	switch item.id {
	case "overview_status":
		return []string{
			fmt.Sprintf("共享入口: %s", snap.shareEntry),
			fmt.Sprintf("当前节点: %s", snap.currentNode),
			fmt.Sprintf("运行模式: %s", plainText(snap.modeSummary)),
			fmt.Sprintf("出口摘要: %s", plainText(snap.egressSummary)),
			"",
			noteLine("回车后会在这里展开完整运行状态。"),
		}
	case "overview_summary":
		return []string{
			fmt.Sprintf("配置文件: %s", snap.configPath),
			fmt.Sprintf("面板入口: %s", snap.panelURL),
			fmt.Sprintf("网络接口: %s", snap.iface),
			"",
			noteLine("回车后会展开 TUI 版配置摘要。"),
		}
	case "overview_config", "system_config":
		return []string{
			"这里会留在 TUI 内展示配置中心和快捷入口。",
			"如果需要完整编辑，可在底部输入 /config open。",
			"",
			noteLine("回车后不会离开 TUI。"),
		}
	case "overview_guide":
		return []string{
			"1. 先确认共享入口和当前节点是否符合预期",
			"2. 再决定是继续调节点、配置 chains，还是拿设备接入",
			"3. 任何时候都可以从底部输入框直接执行命令",
			"",
			noteLine("回车后会展开完整功能导航。"),
		}
	case "routing_egress":
		return []string{
			fmt.Sprintf("当前节点: %s", snap.currentNode),
			fmt.Sprintf("出口摘要: %s", plainText(snap.egressSummary)),
			"",
			noteLine("更细的入口 / 普通出口 / 住宅出口信息可用 gateway status 查看。"),
		}
	case "routing_panel":
		return []string{
			fmt.Sprintf("面板地址: %s", snap.panelURL),
			"适合做节点测速、切换节点、查看连接和流量。",
			"如果你不想记命令，Web 面板和这个 TUI 可以配合使用。",
		}
	case "devices_mobile":
		return []string{
			fmt.Sprintf("把手机网关改成: %s", snap.shareEntry),
			fmt.Sprintf("把手机 DNS 改成: %s", snap.shareEntry),
			"手机和电脑需要在同一个 Wi-Fi / 路由器下。",
			noteLine("更完整的分步说明在 README 和设备指南里。"),
		}
	case "devices_console":
		return []string{
			fmt.Sprintf("Switch / PS5 / Apple TV / 电视的网关指向: %s", snap.shareEntry),
			"大多数设备还需要把 DNS 一起改成这台机器。",
			"配置完成后可以先用 YouTube、eShop、PSN、Netflix 做验证。",
		}
	case "devices_entry":
		return []string{
			fmt.Sprintf("共享入口 IP: %s", snap.shareEntry),
			fmt.Sprintf("控制面板: %s", snap.panelURL),
			fmt.Sprintf("配置文件: %s", snap.configPath),
			fmt.Sprintf("当前模式: %s", plainText(snap.modeSummary)),
		}
	case "system_paths":
		return []string{
			fmt.Sprintf("配置文件: %s", snap.configPath),
			fmt.Sprintf("日志文件: %s", logFile),
			fmt.Sprintf("数据目录: %s", ensureDataDir()),
			fmt.Sprintf("管理面板: %s", snap.panelURL),
		}
	default:
		return []string{
			noteLine(item.desc),
			noteLine("回车执行这个功能，或在底部输入框里直接输入命令。"),
			fmt.Sprintf("当前模式: %s", plainText(snap.modeSummary)),
			fmt.Sprintf("当前节点: %s", snap.currentNode),
			fmt.Sprintf("扩展模式: %s", cfg.Extension.Mode),
		}
	}
}

func activeTabDescription(tab consoleTab) string {
	switch tab {
	case consoleTabRouting:
		return "节点、出口、面板和日志"
	case consoleTabExtension:
		return "chains / script / update / restart"
	case consoleTabDevices:
		return "局域网接入与设备配置"
	case consoleTabSystem:
		return "配置中心、路径、停止和退出"
	default:
		return "总览当前运行状态与配置摘要"
	}
}

func tabLabel(tab consoleTab) string {
	switch tab {
	case consoleTabRouting:
		return "策略与节点"
	case consoleTabExtension:
		return "扩展"
	case consoleTabDevices:
		return "设备接入"
	case consoleTabSystem:
		return "系统"
	default:
		return "总览"
	}
}

func defaultSuggestionsForTab(tab consoleTab) []string {
	switch tab {
	case consoleTabRouting:
		return []string{"/nodes", "/status", "/logs", "/summary"}
	case consoleTabExtension:
		return []string{"/chains", "/chains setup", "/update", "/restart"}
	case consoleTabDevices:
		return []string{"/device", "/status", "/summary", "/guide"}
	case consoleTabSystem:
		return []string{"/config", "/config open", "/stop", "/exit"}
	default:
		return []string{"/status", "/summary", "/config", "/help"}
	}
}

func consoleCommandSuggestions() []string {
	base := []string{
		"/help",
		"/status",
		"/summary",
		"/config",
		"/config open",
		"/chains",
		"/chains setup",
		"/nodes",
		"/groups",
		"/device",
		"/logs",
		"/guide",
		"/update",
		"/clear",
		"/restart",
		"/stop",
		"/exit",
	}
	out := make([]string, 0, len(base)*2)
	for _, item := range base {
		out = append(out, item, strings.TrimPrefix(item, "/"))
	}
	return out
}

func dedupeSuggestions(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if !strings.HasPrefix(item, "/") {
			item = "/" + item
		}
		if slices.Contains(out, item) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (m runtimeConsoleModel) renderPicker(width, height int) string {
	if len(m.groups) == 0 {
		return "暂无可用节点分组"
	}

	if m.picker == pickerModeGroups {
		lines := []string{"先选择一个节点分组，然后回车进入节点列表。", ""}
		for i, group := range m.groups {
			cursor := "  "
			style := lipgloss.NewStyle().Foreground(lipgloss.Color("#cbd5e1"))
			if i == m.groupCursor {
				cursor = "› "
				style = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7dd3fc"))
			}
			lines = append(lines, style.Render(fmt.Sprintf("%s%s  [%s]  当前: %s", cursor, group.Name, group.Type, group.Now)))
		}
		return lipgloss.NewStyle().
			Height(height).
			MaxHeight(height).
			Render(strings.Join(lines, "\n"))
	}

	group := m.groups[m.groupCursor]
	lines := []string{
		fmt.Sprintf("节点分组: %s", group.Name),
		fmt.Sprintf("当前节点: %s", group.Now),
		"",
	}
	for i, node := range group.All {
		cursor := "  "
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("#cbd5e1"))
		if i == m.nodeCursor {
			cursor = "› "
			style = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f59e0b"))
		}
		if node == group.Now {
			node += "  (current)"
		}
		lines = append(lines, style.Render(cursor+node))
	}
	if width > 0 {
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			MaxHeight(height).
			Render(strings.Join(lines, "\n"))
	}
	return lipgloss.NewStyle().
		Height(height).
		MaxHeight(height).
		Render(strings.Join(lines, "\n"))
}

func (m runtimeConsoleModel) capture(fn func()) string {
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	r, w, err := os.Pipe()
	if err != nil {
		return "无法捕获输出"
	}

	os.Stdout = w
	os.Stderr = w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	_ = w.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	_ = r.Close()
	return <-done
}

func (m runtimeConsoleModel) captureLogLines(n int) []string {
	data, err := os.ReadFile(m.logFile)
	if err != nil {
		return []string{errorLine("无法读取日志文件。")}
	}
	lines := splitLines(string(data))
	start := len(lines) - n
	if start < 0 {
		start = 0
	}

	out := []string{noteLine("最近日志:")}
	for _, line := range lines[start:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, "  "+line)
	}
	out = append(out, noteLine("实时查看: tail -f "+m.logFile))
	return out
}

func newConsoleClient(cfg *config.Config) *mihomo.Client {
	apiURL := mihomo.FormatAPIURL("127.0.0.1", cfg.Runtime.Ports.API)
	return mihomo.NewClient(apiURL, cfg.Runtime.APISecret)
}

func commandLine(text string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7dd3fc")).Render("› " + text)
}
func noteLine(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#94a3b8")).Render("[note] " + text)
}
func successLine(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#4ade80")).Render("[ok] " + text)
}
func errorLine(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Render("[error] " + text)
}

func outputBlock(text string) []string {
	text = strings.TrimSpace(stripANSI(text))
	if text == "" {
		return nil
	}
	raw := splitLines(text)
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimRight(plainText(line), " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if isSeparatorLine(trimmed) {
			continue
		}
		if isConsoleSectionTitle(trimmed) {
			if len(out) > 0 {
				out = append(out, "")
			}
			out = append(out, renderSectionTitle(trimmed))
			continue
		}
		out = append(out, "  "+trimmed)
	}
	return out
}

func renderConfigSummaryDetailLines(cfg *config.Config) []string {
	lines := []string{
		renderSectionTitle("配置来源"),
		"  配置文件: " + displayConfigPath(),
		"  代理来源: " + cfg.Proxy.Source,
		"  订阅名称: " + cfg.Proxy.SubscriptionName,
	}
	if cfg.Proxy.Source == "url" {
		lines = append(lines, "  订阅链接: "+shortText(cfg.Proxy.SubscriptionURL, 72))
	} else {
		lines = append(lines, "  本地配置: "+cfg.Proxy.ConfigFile)
	}

	lines = append(lines,
		"",
		renderSectionTitle("运行模式"),
		"  TUN: "+tuiOnOff(cfg.Runtime.Tun.Enabled),
		"  本机绕过代理: "+tuiOnOff(cfg.Runtime.Tun.BypassLocal),
		fmt.Sprintf("  端口: mixed %d | redir %d | api %d | dns %d", cfg.Runtime.Ports.Mixed, cfg.Runtime.Ports.Redir, cfg.Runtime.Ports.API, cfg.Runtime.Ports.DNS),
		"",
		renderSectionTitle("扩展模式"),
		"  模式: "+extensionModeName(cfg.Extension.Mode),
	)

	if cfg.Extension.Mode == "script" {
		lines = append(lines, "  脚本路径: "+cfg.Extension.ScriptPath)
	}
	if cfg.Extension.Mode == "chains" && cfg.Extension.ResidentialChain != nil {
		lines = append(lines,
			"  链式模式: "+cfg.Extension.ResidentialChain.Mode,
			"  机场组: "+cfg.Extension.ResidentialChain.AirportGroup,
		)
	}

	lines = append(lines,
		"",
		renderSectionTitle("规则开关"),
		"  局域网直连: "+tuiOnOff(cfg.Rules.LanDirectEnabled()),
		"  国内直连: "+tuiOnOff(cfg.Rules.ChinaDirectEnabled()),
		"  Apple 规则: "+tuiOnOff(cfg.Rules.AppleRulesEnabled()),
		"  Nintendo 代理: "+tuiOnOff(cfg.Rules.NintendoProxyEnabled()),
		"  国外代理: "+tuiOnOff(cfg.Rules.GlobalProxyEnabled()),
		"  广告拦截: "+tuiOnOff(cfg.Rules.AdsRejectEnabled()),
		fmt.Sprintf("  自定义规则: 直连 %d | 代理 %d | 拦截 %d", len(cfg.Rules.ExtraDirectRules), len(cfg.Rules.ExtraProxyRules), len(cfg.Rules.ExtraRejectRules)),
	)
	return lines
}

func renderConfigCenterLines(cfg *config.Config) []string {
	lines := []string{
		renderSectionTitle("当前配置"),
		"  代理来源: " + cfg.Proxy.Source,
		"  TUN: " + tuiOnOff(cfg.Runtime.Tun.Enabled),
		"  本机绕过代理: " + tuiOnOff(cfg.Runtime.Tun.BypassLocal),
		"  扩展模式: " + extensionModeName(cfg.Extension.Mode),
		"  国内直连: " + tuiOnOff(cfg.Rules.ChinaDirectEnabled()),
		"  广告拦截: " + tuiOnOff(cfg.Rules.AdsRejectEnabled()),
		"",
		renderSectionTitle("快捷入口"),
		noteLine("输入 /summary 查看完整配置摘要"),
		noteLine("输入 /nodes 切换节点"),
		noteLine("输入 /chains setup 打开链式代理向导"),
		noteLine("输入 /config open 打开完整交互式配置中心"),
		"",
		renderSectionTitle("说明"),
		noteLine("当前 TUI 先提供查看和快捷入口，完整编辑仍可通过 /config open 进入。"),
	}
	return lines
}

func renderGuideDetailLines(cfg *config.Config, logFile string) []string {
	lines := []string{
		renderSectionTitle("当前主线"),
	}
	if cfg.Runtime.Tun.Enabled {
		lines = append(lines, "  局域网共享已就绪：手机、Switch、PS5、Apple TV 改网关和 DNS 就能接入")
	} else {
		lines = append(lines, "  先开启 TUN：运行 sudo gateway tun on，再执行 sudo gateway restart")
	}

	switch cfg.Extension.Mode {
	case "chains":
		mode := "rule"
		if cfg.Extension.ResidentialChain != nil && cfg.Extension.ResidentialChain.Mode != "" {
			mode = cfg.Extension.ResidentialChain.Mode
		}
		lines = append(lines, "  当前扩展模式: chains / "+mode+"，适合 Claude / ChatGPT / Codex / Cursor")
	case "script":
		lines = append(lines, "  当前扩展模式: script，可继续扩展自定义分流逻辑")
	default:
		lines = append(lines, "  当前未启用扩展模式，可运行 /chains setup 体验内置链式代理向导")
	}

	lines = append(lines,
		"",
		renderSectionTitle("下一步最常用"),
		"  1. 用 /nodes 或 Ctrl+P 切换节点，先把出口调到合适地区",
		"  2. 用 /summary 查看当前配置是否按预期生效",
		"  3. 用 /config open 进入完整配置中心，调整代理来源 / 规则 / 扩展",
		"",
		renderSectionTitle("常用入口"),
		"  /status       查看完整运行状态和出口网络",
		"  /device       查看设备接入说明",
		"  /logs         查看最近日志",
		"  tail -f "+logFile,
	)
	return lines
}

func (m *runtimeConsoleModel) renderStatusDetailLines() []string {
	cfg := loadConfigOrDefault()
	client := newConsoleClient(cfg)
	p := platform.New()

	running, pid, _ := p.IsRunning()
	forwarding, _ := p.IsIPForwardingEnabled()
	tunIf, _ := p.DetectTUNInterface()
	iface := m.iface
	if iface == "" {
		iface, _ = p.DetectDefaultInterface()
	}
	ip := m.ip
	if ip == "" {
		ip, _ = p.DetectInterfaceIP(iface)
	}

	lines := []string{
		renderSectionTitle("运行状态"),
	}
	if running {
		lines = append(lines, fmt.Sprintf("  mihomo: %s (PID: %d)", tuiGood("运行中"), pid))
	} else {
		lines = append(lines, "  mihomo: "+tuiWarn("未运行"))
	}
	lines = append(lines,
		"  IP 转发: "+tuiState(forwarding, "已开启", "未开启"),
		"  TUN 接口: "+fallbackText(tunIf, "未检测到"),
		"  网络接口: "+fallbackText(iface, "未识别"),
		"  局域网 IP: "+fallbackText(ip, "未识别"),
		"  扩展模式: "+plainText(extensionModeSummary(cfg)),
	)

	if client.IsAvailable() {
		lines = append(lines,
			"",
			renderSectionTitle("代理信息"),
		)
		if v, err := client.GetVersion(); err == nil {
			lines = append(lines, "  版本: "+v.Version)
		}
		if pg, err := client.GetProxyGroup("Proxy"); err == nil {
			lines = append(lines, "  当前节点: "+pg.Now)
		}
		if conn, err := client.GetConnections(); err == nil {
			lines = append(lines,
				fmt.Sprintf("  活跃连接: %d", len(conn.Connections)),
				"  上传总量: "+ui.FormatBytes(conn.UploadTotal),
				"  下载总量: "+ui.FormatBytes(conn.DownloadTotal),
			)
		}

		report := egress.Collect(cfg, m.dataDir, client)
		lines = append(lines, "", renderSectionTitle("出口网络"))
		lines = append(lines, renderEgressDetailLines(cfg, report)...)
	}

	lines = append(lines,
		"",
		renderSectionTitle("设备配置"),
		"  网关 (Gateway): "+fallbackText(ip, "未识别"),
		"  DNS: "+fallbackText(ip, "未识别"),
		fmt.Sprintf("  API 面板: http://%s:%d/ui", fallbackText(ip, "127.0.0.1"), cfg.Runtime.Ports.API),
	)
	return lines
}

func renderEgressDetailLines(cfg *config.Config, report *egress.Report) []string {
	if report == nil {
		return []string{"  探测状态: 探测中"}
	}

	lines := []string{
		"  探测来源: " + report.ProbeSource,
	}

	if cfg.Extension.Mode != "chains" {
		if report.ProxyExit != nil {
			lines = append(lines, "  当前出口: "+report.ProxyExit.Summary())
		} else {
			lines = append(lines, "  当前出口: 探测失败")
		}
		return lines
	}

	chainMode := "rule"
	if cfg.Extension.ResidentialChain != nil && cfg.Extension.ResidentialChain.Mode != "" {
		chainMode = cfg.Extension.ResidentialChain.Mode
	}
	lines = append(lines, "  链路模式: "+chainMode)
	if report.AirportNode != nil {
		lines = append(lines, "  入口节点: "+report.AirportNode.Summary())
	} else {
		lines = append(lines, "  入口节点: 未识别当前机场节点")
	}
	if chainMode == "rule" {
		if report.ProxyExit != nil {
			lines = append(lines, "  普通出口: "+report.ProxyExit.Summary())
		} else {
			lines = append(lines, "  普通出口: 探测失败")
		}
	}
	if report.ResidentialExit != nil {
		label := "  住宅出口: "
		if chainMode == "global" {
			label = "  全局出口: "
		}
		lines = append(lines, label+report.ResidentialExit.Summary())
	} else {
		lines = append(lines, "  住宅出口: 探测失败")
	}
	if chainMode == "rule" {
		lines = append(lines, noteLine("普通流量走机场出口，AI 相关流量走住宅出口"))
	} else {
		lines = append(lines, noteLine("当前为 global 模式，所有流量都会走住宅出口"))
	}
	return lines
}

func tuiOnOff(enabled bool) string {
	if enabled {
		return tuiGood("on")
	}
	return tuiWarn("off")
}

func tuiState(enabled bool, onText, offText string) string {
	if enabled {
		return tuiGood(onText)
	}
	return tuiWarn(offText)
}

func tuiGood(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#a3e635")).Render(text)
}

func tuiWarn(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#fbbf24")).Render(text)
}

func fallbackText(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func stripANSI(s string) string {
	return ansiEscapePattern.ReplaceAllString(s, "")
}

func plainText(s string) string {
	replacer := strings.NewReplacer(
		"\x1b[0m", "",
		"\x1b[1m", "",
		"\x1b[2m", "",
		"\x1b[31m", "",
		"\x1b[32m", "",
		"\x1b[33m", "",
		"\x1b[34m", "",
		"\x1b[35m", "",
		"\x1b[36m", "",
		"\x1b[37m", "",
	)
	return replacer.Replace(s)
}

func isSeparatorLine(s string) bool {
	return strings.Trim(s, "─- ") == ""
}

func isConsoleSectionTitle(s string) bool {
	if strings.Contains(s, ":") {
		return false
	}
	if strings.HasPrefix(s, "[") {
		return false
	}
	count := utf8.RuneCountInString(s)
	return count > 0 && count <= 8
}

func truncateText(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	if limit == 1 {
		return "…"
	}
	return string(runes[:limit-1]) + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
