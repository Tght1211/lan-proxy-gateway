package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/egress"
	"github.com/tght/lan-proxy-gateway/internal/mihomo"
)

type consoleAction int

const (
	consoleActionNone consoleAction = iota
	consoleActionExit
	consoleActionRestart
	consoleActionStop
	consoleActionOpenConfig
	consoleActionOpenChainsSetup
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

type snapshot struct {
	modeSummary   string
	egressSummary string
	panelURL      string
	configPath    string
	iface         string
	currentNode   string
	shareEntry    string
}

type petTickMsg time.Time

type runtimeConsoleModel struct {
	width    int
	height   int
	mainHeight int
	logFile  string
	ip       string
	iface    string
	dataDir  string
	cfg      *config.Config
	client   *mihomo.Client
	snapshot snapshot
	update   *updateNotice

	input    textinput.Model
	viewport viewport.Model
	spin     spinner.Model

	history []string
	action  consoleAction

	pending *pendingConfirm

	picker      pickerMode
	groups      []mihomo.ProxyGroup
	groupCursor int
	nodeCursor  int
	tab         consoleTab
	cursor      int
	detailTitle string

	petFrame int
}

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
	ti := textinput.New()
	ti.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#7dd3fc")).Render("› ")
	ti.Placeholder = "/status, /groups, /chains setup"
	focusCmd := ti.Focus()
	ti.CharLimit = 512
	ti.SetWidth(48)
	ti.ShowSuggestions = true
	ti.SetSuggestions(consoleCommandSuggestions())
	ti.KeyMap.AcceptSuggestion = key.NewBinding(key.WithKeys("tab"))
	ti.KeyMap.NextSuggestion = key.NewBinding(key.WithKeys("down"))
	ti.KeyMap.PrevSuggestion = key.NewBinding(key.WithKeys("up"))

	sp := spinner.New()
	sp.Spinner = spinner.Points
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#f59e0b"))

	vp := viewport.New()

	m := runtimeConsoleModel{
		logFile:  logFile,
		ip:       ip,
		iface:    iface,
		dataDir:  dataDir,
		cfg:      cfg,
		client:   newConsoleClient(cfg),
		update:   update,
		input:    ti,
		viewport: vp,
		spin:     sp,
		tab:      consoleTabOverview,
	}
	m.refreshSnapshot()
	m.refreshSelectionPreview()
	if focusCmd != nil {
		_, _ = m.Update(focusCmd())
	}
	return m
}

func (m runtimeConsoleModel) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, petTickCmd(), m.input.Focus())
}

func (m runtimeConsoleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case petTickMsg:
		m.petFrame = (m.petFrame + 1) % len(petFrames())
		return m, petTickCmd()

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
		case "left":
			if strings.TrimSpace(m.input.Value()) == "" {
				m.prevTab()
				m.refreshSelectionPreview()
				return m, nil
			}
		case "right":
			if strings.TrimSpace(m.input.Value()) == "" {
				m.nextTab()
				m.refreshSelectionPreview()
				return m, nil
			}
		case "up":
			if strings.TrimSpace(m.input.Value()) == "" {
				m.moveCursor(-1)
				m.refreshSelectionPreview()
				return m, nil
			}
		case "down":
			if strings.TrimSpace(m.input.Value()) == "" {
				m.moveCursor(1)
				m.refreshSelectionPreview()
				return m, nil
			}
		case "r":
			if strings.TrimSpace(m.input.Value()) == "" {
				m.refreshSnapshot()
				m.refreshSelectionPreview()
				return m, nil
			}
		case "q":
			if strings.TrimSpace(m.input.Value()) == "" {
				m.action = consoleActionExit
				return m, tea.Quit
			}
		case "enter":
			value := strings.TrimSpace(m.input.Value())
			m.input.SetValue("")
			if value == "" {
				return m.executeSelectedAction()
			}
			return m.handleCommand(value)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
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
			noteLine("/config        打开配置中心"),
			noteLine("/chains        查看链式代理 / 扩展状态"),
			noteLine("/chains setup  打开链式代理向导"),
			noteLine("/groups        打开策略组选择器"),
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
		m.showCapturedDetail("运行状态", func() { runStatus(nil, nil) })
	case "summary":
		m.showCapturedDetail("配置摘要", func() { printConfigSummary(loadConfigOrDefault()) })
	case "config":
		m.action = consoleActionOpenConfig
		return *m, tea.Quit
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
		return m.openGroupPicker()
	case "device":
		cfg := loadConfigOrDefault()
		m.showCapturedDetail("设备接入", func() { printDeviceSetupPanel(m.ip, cfg.Runtime.Ports.API) })
	case "logs", "log":
		m.setDetail("最近日志", m.captureLogLines(30))
	case "guide":
		m.showCapturedDetail("功能导航", func() { printStartGuide(loadConfigOrDefault(), m.logFile) })
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
		m.action = consoleActionExit
		return *m, tea.Quit
	default:
		m.setDetail("命令错误", []string{errorLine("未识别的命令。输入 /help 查看可用命令。")})
	}

	m.refreshSnapshot()
	return *m, nil
}

func (m *runtimeConsoleModel) handleConfirm(value string) (tea.Model, tea.Cmd) {
	answer := strings.ToLower(strings.TrimSpace(value))

	if answer != "y" && answer != "yes" {
		m.pending = nil
		m.setDetail("已取消", []string{noteLine("已取消。")})
		return *m, nil
	}

	action := m.pending.action
	m.pending = nil
	switch action {
	case consoleActionRestart:
		m.setDetail("重启网关", []string{successLine("准备重启网关...")})
		m.action = consoleActionRestart
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
				m.setDetail("节点已切换", []string{successLine(fmt.Sprintf("已切换策略组 %s -> %s", group.Name, target))})
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
		m.setDetail("策略组读取失败", []string{errorLine("无法读取策略组: " + err.Error())})
		return *m, nil
	}
	if len(groups) == 0 {
		m.setDetail("策略组选择器", []string{noteLine("当前没有可切换的策略组。")})
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
	}
	if pg, err := m.client.GetProxyGroup("Proxy"); err == nil && strings.TrimSpace(pg.Now) != "" {
		m.snapshot.currentNode = pg.Now
	}
}

func (m *runtimeConsoleModel) refreshViewport() {
	m.viewport.SetContent(strings.Join(m.history, "\n"))
	m.viewport.GotoBottom()
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
	m.input.SetWidth(max(20, m.width-10))
	m.viewport.SetWidth(max(24, detailWidth-4))
	m.viewport.SetHeight(max(5, mainHeight-4))
}

func (m runtimeConsoleModel) renderHeader() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#f8fafc"))
	subStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#94a3b8"))
	ruleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#334155"))

	line1 := titleStyle.Render("Gateway Console")
	line2 := m.renderTabs()
	line3 := subStyle.Render(activeTabDescription(m.tab))
	line4 := subStyle.Render("←/→ 切换分区  ·  ↑/↓ 选择功能  ·  Enter 执行  ·  Ctrl+P 节点选择  ·  Tab 补全")
	rule := ruleStyle.Render(strings.Repeat("─", max(12, m.width-2)))

	return lipgloss.NewStyle().
		Padding(0, 1).
		Width(max(36, m.width-2)).
		Render(lipgloss.JoinVertical(lipgloss.Left, line1, line2, line3, line4, rule))
}

func (m runtimeConsoleModel) renderMain() string {
	detailWidth := m.detailPaneWidth()
	sideWidth := max(28, m.width-detailWidth-2)
	detail := m.renderTranscript(detailWidth)
	sidebar := m.renderSidebar(sideWidth)
	return lipgloss.JoinHorizontal(lipgloss.Top, detail, sidebar)
}

func (m runtimeConsoleModel) renderTranscript(width int) string {
	title := m.detailTitle
	if title == "" {
		title = "功能详情"
	}
	content := m.viewport.View()
	if m.picker != pickerModeNone {
		title = "策略组选择器"
		content = m.renderPicker(width-4, max(3, m.mainHeight-4))
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#334155")).
		Padding(0, 1).
		Width(width)

	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#e2e8f0")).Render(title)
	bodyHeight := max(3, m.mainHeight-4)
	body := lipgloss.NewStyle().
		MaxWidth(width - 4).
		Height(bodyHeight).
		MaxHeight(bodyHeight).
		Render(content)

	return box.Render(lipgloss.JoinVertical(lipgloss.Left, header, "", body))
}

func (m runtimeConsoleModel) renderSidebar(width int) string {
	if width <= 0 {
		return ""
	}

	lines := []string{
		renderSectionTitle("系统摘要"),
		"共享入口: " + m.snapshot.shareEntry,
		"当前节点: " + m.snapshot.currentNode,
		"运行模式: " + plainText(m.snapshot.modeSummary),
		"出口摘要: " + plainText(m.snapshot.egressSummary),
		"",
		renderSectionTitle(tabLabel(m.tab)),
	}
	lines = append(lines, m.renderMenuLines()...)

	if width >= 34 {
		pet := petFrames()[m.petFrame]
		lines = append(lines,
			"",
			renderSectionTitle("Gateway Pet"),
			pet,
			"mood: " + petMood(m.cfg),
		)
	}

	lines = append(lines,
		"",
		renderSectionTitle("快捷提示"),
		"R 刷新当前摘要",
		"Q 退出控制台",
		"底部输入框可直接执行命令",
	)

	card := m.renderCard(width, "控制区", lines)
	return lipgloss.NewStyle().
		Height(max(8, m.mainHeight)).
		MaxHeight(max(8, m.mainHeight)).
		Render(card)
}

func (m runtimeConsoleModel) renderCard(width int, title string, lines []string) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7dd3fc"))

	renderedLines := make([]string, 0, len(lines))
	for _, line := range lines {
		renderedLines = append(renderedLines, lipgloss.NewStyle().MaxWidth(width-4).Render(line))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#334155")).
		Padding(0, 1).
		Width(width)

	return box.Render(lipgloss.JoinVertical(lipgloss.Left, titleStyle.Render(title), "", strings.Join(renderedLines, "\n")))
}

func (m runtimeConsoleModel) renderInput() string {
	title := "输入命令"
	placeholder := lipgloss.NewStyle().Foreground(lipgloss.Color("#94a3b8")).Render("可直接输入命令，也可用右侧菜单。示例: /status /groups /chains setup")
	if m.pending != nil {
		title = "确认操作"
		placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#fbbf24")).Render(m.pending.prompt + "  输入 y / n")
	}
	if m.picker != pickerModeNone {
		title = "选择器"
		placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#fbbf24")).Render("↑/↓ 选择，Enter 确认，Esc 返回")
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#475569")).
		Padding(0, 1).
		Width(max(36, m.width-2))

	head := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#e2e8f0")).Render(title)
	return box.Render(lipgloss.JoinVertical(lipgloss.Left, head, "", placeholder, m.input.View(), m.renderCommandSuggestions()))
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
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#94a3b8"))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#cbd5e1"))

	lines := []string{
		infoStyle.Render("Tab 接受补全  ·  输入命令与菜单操作可以同时使用"),
	}

	matches := dedupeSuggestions(m.input.MatchedSuggestions())
	if len(matches) == 0 {
		matches = defaultSuggestionsForTab(m.tab)
	}
	if len(matches) > 0 {
		lines = append(lines, hintStyle.Render("建议: "+strings.Join(matches, "   ")))
	}

	return strings.Join(lines, "\n")
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
	m.refreshSnapshot()
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
		m.showCapturedDetail("运行状态", func() { runStatus(nil, nil) })
	case "overview_summary":
		m.showCapturedDetail("配置摘要", func() { printConfigSummary(loadConfigOrDefault()) })
	case "overview_config":
		m.action = consoleActionOpenConfig
		return *m, tea.Quit
	case "overview_guide":
		m.showCapturedDetail("功能导航", func() { printStartGuide(loadConfigOrDefault(), m.logFile) })
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
		m.action = consoleActionOpenConfig
		return *m, tea.Quit
	default:
		m.refreshSelectionPreview()
	}
	return *m, nil
}

func menuItemsForTab(tab consoleTab) []consoleMenuItem {
	switch tab {
	case consoleTabRouting:
		return []consoleMenuItem{
			{id: "routing_groups", key: "01", title: "策略组切换", desc: "打开策略组和节点选择器"},
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
			{id: "system_config", key: "01", title: "配置中心", desc: "打开交互式配置中心"},
			{id: "system_paths", key: "02", title: "运行路径", desc: "查看配置、日志和面板路径"},
			{id: "system_stop", key: "03", title: "停止网关", desc: "停止当前运行中的网关"},
			{id: "system_exit", key: "04", title: "退出控制台", desc: "退出 TUI，但保持网关继续运行"},
		}
	default:
		return []consoleMenuItem{
			{id: "overview_status", key: "01", title: "运行状态", desc: "查看完整运行状态和出口网络"},
			{id: "overview_summary", key: "02", title: "配置摘要", desc: "查看当前配置摘要和生效路径"},
			{id: "overview_config", key: "03", title: "配置中心", desc: "进入交互式配置中心"},
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
			noteLine("回车后会展开完整配置摘要。"),
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
			"适合做节点测速、切换策略组、查看连接和流量。",
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
		return []string{"/groups", "/status", "/logs", "/summary"}
	case consoleTabExtension:
		return []string{"/chains", "/chains setup", "/update", "/restart"}
	case consoleTabDevices:
		return []string{"/device", "/status", "/summary", "/guide"}
	case consoleTabSystem:
		return []string{"/config", "/stop", "/exit", "/logs"}
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
		"/chains",
		"/chains setup",
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
		if len(out) >= 4 {
			break
		}
	}
	return out
}

func (m runtimeConsoleModel) renderPicker(width, height int) string {
	if len(m.groups) == 0 {
		return "暂无可用策略组"
	}

	if m.picker == pickerModeGroups {
		lines := []string{"选择一个策略组，然后回车进入节点列表。", ""}
		for i, group := range m.groups {
			cursor := "  "
			style := lipgloss.NewStyle().Foreground(lipgloss.Color("#cbd5e1"))
			if i == m.groupCursor {
				cursor = m.spin.View() + " "
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
		fmt.Sprintf("策略组: %s", group.Name),
		fmt.Sprintf("当前节点: %s", group.Now),
		"",
	}
	for i, node := range group.All {
		cursor := "  "
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("#cbd5e1"))
		if i == m.nodeCursor {
			cursor = m.spin.View() + " "
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

func (m *runtimeConsoleModel) pushHistory(lines ...string) {
	m.history = append(m.history, lines...)
	if len(m.history) > 240 {
		m.history = m.history[len(m.history)-240:]
	}
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

func petTickCmd() tea.Cmd {
	return tea.Tick(320*time.Millisecond, func(t time.Time) tea.Msg {
		return petTickMsg(t)
	})
}

func petFrames() []string {
	return []string{
		" /\\_/\\\\\n( ^.^ )\n / >🌐",
		" /\\_/\\\\\n( o.o )\n / >✨",
		" /\\_/\\\\\n( -.- )\n / >🛰",
	}
}

func petMood(cfg *config.Config) string {
	if cfg.Extension.Mode == "chains" {
		return "guarding ai traffic"
	}
	if cfg.Runtime.Tun.Enabled {
		return "sharing lan gateway"
	}
	return "waiting for commands"
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
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	raw := splitLines(text)
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, "  "+line)
	}
	return out
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
