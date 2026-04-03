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

type inputPrompt struct {
	title       string
	label       string
	placeholder string
	apply       func(m *runtimeConsoleModel, value string) error
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
	kind  consoleItemKind
	cmd   string
	enter string
}

type consoleItemKind int

const (
	consoleItemInfo consoleItemKind = iota
	consoleItemAction
	consoleItemConfirm
)

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
	consoleFocusDetail
	consoleFocusInput
)

type refreshPulseMsg struct{}
type navAlertPulseMsg struct{}
type pickerDelayResultMsg struct {
	node  string
	delay int
	err   error
}

const refreshPulseFrames = 4
const navAlertFrames = 5

type snapshot struct {
	modeSummary     string
	egressSummary   string
	panelURL        string
	configPath      string
	iface           string
	currentNode     string
	shareEntry      string
	refreshedAt     string
	activeProfile   config.ProxyProfile
	subscription    subscriptionSnapshot
	node            nodeSnapshot
	network         networkSnapshot
	traffic         trafficSnapshot
	addresses       addressSnapshot
	latency         latencySnapshot
	alerts          []alertMessage
	overviewSummary string
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
	lastPolled time.Time
	lastUp     int64
	lastDown   int64

	viewport    viewport.Model
	focus       consoleFocus
	inputValue  string
	inputCursor int
	history     []string
	historyPos  int

	action consoleAction

	pending *pendingConfirm
	prompt  *inputPrompt

	picker        pickerMode
	groups        []mihomo.ProxyGroup
	groupCursor   int
	nodeCursor    int
	tab           consoleTab
	cursor        int
	detailTitle   string
	refreshPulse  int
	navAlertPulse int
	pickerStatus  string
	nodeDelays    map[string]string
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
		focus:      consoleFocusHeader,
		historyPos: -1,
		nodeDelays: map[string]string{},
	}
	m.refreshSnapshot()
	m.setDetail("首页", renderHomeDashboardLines(m.snapshot))
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

	case navAlertPulseMsg:
		if m.navAlertPulse > 0 {
			m.navAlertPulse--
			if m.navAlertPulse > 0 {
				return m, navAlertTickCmd()
			}
		}
		return m, nil

	case tea.MouseWheelMsg:
		if m.focus == consoleFocusDetail {
			return m.handleDetailWheel(msg.Mouse().Button)
		}
		return m, nil

	case tea.MouseMsg:
		if m.focus == consoleFocusNav && m.picker == pickerModeNone {
			return m, nil
		}
		return m, nil

	case pickerDelayResultMsg:
		if m.nodeDelays == nil {
			m.nodeDelays = map[string]string{}
		}
		if msg.err != nil {
			m.nodeDelays[msg.node] = "失败"
			m.pickerStatus = "测速失败: " + msg.node
			m.setDetail("节点测速失败", []string{errorLine(msg.err.Error())})
			return m, nil
		}
		m.nodeDelays[msg.node] = fmt.Sprintf("%dms", msg.delay)
		m.pickerStatus = fmt.Sprintf("测速完成: %s · %dms", msg.node, msg.delay)
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

		if m.focus == consoleFocusDetail {
			if model, cmd, handled := m.handleDetailWorkspaceKey(msg); handled {
				return model, cmd
			}
			switch msg.String() {
			case "esc":
				m.focus = consoleFocusNav
				return m, nil
			case "left", "right":
				return m, nil
			case "up", "k":
				m.viewport.ScrollUp(1)
				return m, nil
			case "down", "j":
				m.viewport.ScrollDown(1)
				return m, nil
			case "pgup", "b":
				m.viewport.HalfPageUp()
				return m, nil
			case "pgdown", "f":
				m.viewport.HalfPageDown()
				return m, nil
			case "home":
				m.viewport.GotoTop()
				return m, nil
			case "end":
				m.viewport.GotoBottom()
				return m, nil
			}
		}

		switch msg.String() {
		case "esc":
			if m.focus == consoleFocusNav {
				m.focus = consoleFocusHeader
			}
			return m, nil
		case "left":
			if m.focus == consoleFocusNav {
				m.triggerNavBoundaryAlert()
				return m, navAlertTickCmd()
			}
			if m.focus != consoleFocusHeader {
				return m, nil
			}
			m.prevTab()
			m.refreshSelectionPreview()
			return m, nil
		case "right":
			if m.focus == consoleFocusNav {
				m.triggerNavBoundaryAlert()
				return m, navAlertTickCmd()
			}
			if m.focus != consoleFocusHeader {
				return m, nil
			}
			m.nextTab()
			m.refreshSelectionPreview()
			return m, nil
		case "up":
			if m.focus == consoleFocusHeader {
				return m, nil
			}
			m.navAlertPulse = 0
			m.moveCursor(-1)
			m.refreshSelectionPreview()
			return m, nil
		case "down":
			if m.focus == consoleFocusHeader {
				m.focus = consoleFocusNav
				m.refreshSelectionPreview()
				return m, nil
			}
			m.navAlertPulse = 0
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
				m.refreshSelectionPreview()
				return m, nil
			}
			m.navAlertPulse = 0
			return m.executeSelectedAction()
		}
	}

	return m, nil
}

func (m runtimeConsoleModel) View() tea.View {
	if m.width == 0 || m.height == 0 {
		v := tea.NewView("loading...")
		v.AltScreen = true
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}

	sections := make([]string, 0, 4)
	if alert := m.renderAlertBanner(); alert != "" {
		sections = append(sections, alert)
	}
	sections = append(sections, m.renderHeader(), m.renderMain(), m.renderFooter())
	page := lipgloss.JoinVertical(lipgloss.Left, sections...)
	if m.refreshPulseActive() {
		page = lipgloss.NewStyle().
			PaddingLeft(m.refreshPulseOffset()).
			Render(page)
	}
	if overlay := m.renderOverlay(); overlay != "" {
		page = overlay
	}

	v := tea.NewView(page)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
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
			noteLine("/proxy         打开代理来源工作台"),
			noteLine("/proxy source  切换 url / file"),
			noteLine("/tun on|off    切换 TUN"),
			noteLine("/bypass on|off 切换本机绕过代理"),
			noteLine("/rule <name>   切换 lan/china/apple/nintendo/global/ads"),
			noteLine("/chains        查看链式代理 / 扩展状态"),
			noteLine("/extension     打开扩展模式工作台"),
			noteLine("/chain         打开住宅代理工作台"),
			noteLine("/chains setup  打开链式代理向导"),
			noteLine("/nodes         打开节点工作台（切换节点 / 测延迟）"),
			noteLine("/speed         打开节点测速工作台"),
			noteLine("/device        查看设备接入说明"),
			noteLine("/logs          查看最近日志"),
			noteLine("/guide         查看功能导航"),
			noteLine("/update        查看升级提示"),
			noteLine("/clear         清空主屏记录"),
			noteLine("/restart       重启网关（需确认）"),
			noteLine("/stop          停止网关（需确认）"),
			noteLine("/exit          退出控制台"),
		})
		m.focus = consoleFocusDetail
	case "status":
		m.setDetail("运行状态", m.renderStatusDetailLines())
		m.focus = consoleFocusDetail
	case "summary":
		m.setDetail("配置摘要", renderConfigSummaryDetailLines(loadConfigOrDefault()))
		m.focus = consoleFocusDetail
	case "config":
		if len(args) > 0 && (args[0] == "open" || args[0] == "cli") {
			m.action = consoleActionOpenConfig
			return *m, tea.Quit
		}
		m.setDetail("配置中心", renderConfigCenterLines(loadConfigOrDefault()))
		m.focus = consoleFocusDetail
	case "proxy":
		if len(args) == 0 {
			m.openProxyWorkspace("")
			return *m, nil
		}
		switch strings.ToLower(args[0]) {
		case "source":
			if len(args) < 2 {
				m.openProxyWorkspace(errorLine("用法: /proxy source url|file"))
				return *m, nil
			}
			source, err := normalizeProxySource(args[1])
			if err != nil {
				m.openProxyWorkspace(errorLine(err.Error()))
				return *m, nil
			}
			cfg, err := updateProxySource(source)
			if err != nil {
				m.openProxyWorkspace(errorLine(err.Error()))
				return *m, nil
			}
			m.applyUpdatedConfig(cfg)
			m.openProxyWorkspace(successLine("代理来源已切换为 " + source))
			return *m, nil
		case "url":
			if len(args) < 2 {
				m.openProxyWorkspace(errorLine("用法: /proxy url <订阅链接>"))
				return *m, nil
			}
			cfg, err := updateSubscriptionURL(strings.Join(args[1:], " "))
			if err != nil {
				m.openProxyWorkspace(errorLine(err.Error()))
				return *m, nil
			}
			m.applyUpdatedConfig(cfg)
			m.openProxyWorkspace(successLine("订阅链接已更新"))
			return *m, nil
		case "file":
			if len(args) < 2 {
				m.openProxyWorkspace(errorLine("用法: /proxy file <本地配置文件路径>"))
				return *m, nil
			}
			cfg, err := updateProxyConfigFile(strings.Join(args[1:], " "))
			if err != nil {
				m.openProxyWorkspace(errorLine(err.Error()))
				return *m, nil
			}
			m.applyUpdatedConfig(cfg)
			m.openProxyWorkspace(successLine("本地配置文件路径已更新"))
			return *m, nil
		case "name":
			if len(args) < 2 {
				m.openProxyWorkspace(errorLine("用法: /proxy name <订阅名称>"))
				return *m, nil
			}
			cfg, err := updateSubscriptionName(strings.Join(args[1:], " "))
			if err != nil {
				m.openProxyWorkspace(errorLine(err.Error()))
				return *m, nil
			}
			m.applyUpdatedConfig(cfg)
			m.openProxyWorkspace(successLine("订阅名称已更新"))
			return *m, nil
		default:
			m.openProxyWorkspace(errorLine("支持: /proxy source|url|file|name"))
			return *m, nil
		}
	case "source":
		if len(args) < 1 {
			m.openProxyWorkspace("")
			return *m, nil
		}
		source, err := normalizeProxySource(args[0])
		if err != nil {
			m.openProxyWorkspace(errorLine(err.Error()))
			return *m, nil
		}
		cfg, err := updateProxySource(source)
		if err != nil {
			m.openProxyWorkspace(errorLine(err.Error()))
			return *m, nil
		}
		m.applyUpdatedConfig(cfg)
		m.openProxyWorkspace(successLine("代理来源已切换为 " + source))
		return *m, nil
	case "tun":
		if len(args) == 0 {
			m.openRuntimeWorkspace("")
			return *m, nil
		}
		enabled, err := normalizeOnOffToggle(args[0], loadConfigOrDefault().Runtime.Tun.Enabled)
		if err != nil {
			m.openRuntimeWorkspace(errorLine("用法: /tun on|off|toggle"))
			return *m, nil
		}
		cfg, err := updateTunEnabled(enabled)
		if err != nil {
			m.openRuntimeWorkspace(errorLine(err.Error()))
			return *m, nil
		}
		m.applyUpdatedConfig(cfg)
		m.openRuntimeWorkspace(successLine("TUN 已切换为 " + onOff(enabled)))
		return *m, nil
	case "bypass":
		if len(args) == 0 {
			m.openRuntimeWorkspace("")
			return *m, nil
		}
		enabled, err := normalizeOnOffToggle(args[0], loadConfigOrDefault().Runtime.Tun.BypassLocal)
		if err != nil {
			m.openRuntimeWorkspace(errorLine("用法: /bypass on|off|toggle"))
			return *m, nil
		}
		cfg, err := updateBypassLocal(enabled)
		if err != nil {
			m.openRuntimeWorkspace(errorLine(err.Error()))
			return *m, nil
		}
		m.applyUpdatedConfig(cfg)
		m.openRuntimeWorkspace(successLine("本机绕过代理已切换为 " + onOff(enabled)))
		return *m, nil
	case "rules":
		m.openRulesWorkspace("")
		return *m, nil
	case "rule":
		if len(args) == 0 {
			m.openRulesWorkspace("")
			return *m, nil
		}
		ruleName := strings.ToLower(args[0])
		currentCfg := loadConfigOrDefault()
		current := map[string]bool{
			"lan":      currentCfg.Rules.LanDirectEnabled(),
			"china":    currentCfg.Rules.ChinaDirectEnabled(),
			"apple":    currentCfg.Rules.AppleRulesEnabled(),
			"nintendo": currentCfg.Rules.NintendoProxyEnabled(),
			"global":   currentCfg.Rules.GlobalProxyEnabled(),
			"ads":      currentCfg.Rules.AdsRejectEnabled(),
		}[ruleName]
		enabled, err := normalizeOnOffToggle("", current)
		if len(args) > 1 {
			enabled, err = normalizeOnOffToggle(args[1], current)
		}
		if err != nil {
			m.openRulesWorkspace(errorLine("用法: /rule <lan|china|apple|nintendo|global|ads> [on|off|toggle]"))
			return *m, nil
		}
		cfg, err := updateRuleToggle(ruleName, enabled)
		if err != nil {
			m.openRulesWorkspace(errorLine(err.Error()))
			return *m, nil
		}
		m.applyUpdatedConfig(cfg)
		m.openRulesWorkspace(successLine(ruleName + " 已切换为 " + onOff(enabled)))
		return *m, nil
	case "chains":
		if len(args) > 0 && args[0] == "mode" {
			if len(args) < 2 {
				m.openExtensionWorkspace(errorLine("用法: /chains mode rule|global"))
				return *m, nil
			}
			mode, err := normalizeChainMode(args[1])
			if err != nil {
				m.openExtensionWorkspace(errorLine(err.Error()))
				return *m, nil
			}
			cfg, err := updateChainMode(mode)
			if err != nil {
				m.openExtensionWorkspace(errorLine(err.Error()))
				return *m, nil
			}
			m.applyUpdatedConfig(cfg)
			m.openExtensionWorkspace(successLine("chains 路由模式已切换为 " + mode))
			return *m, nil
		}
		if len(args) > 0 && args[0] == "setup" {
			m.action = consoleActionOpenChainsSetup
			return *m, tea.Quit
		}
		m.setDetail("扩展状态", renderExtensionStatusLines(loadConfigOrDefault()))
		m.focus = consoleFocusDetail
	case "extension", "mode":
		if len(args) == 0 {
			m.openExtensionWorkspace("")
			return *m, nil
		}
		mode, err := normalizeExtensionMode(args[0])
		if err != nil {
			m.openExtensionWorkspace(errorLine(err.Error()))
			return *m, nil
		}
		cfg, err := updateExtensionMode(mode)
		if err != nil {
			m.openExtensionWorkspace(errorLine(err.Error()))
			return *m, nil
		}
		m.applyUpdatedConfig(cfg)
		modeName := "off"
		if mode != "" {
			modeName = mode
		}
		m.openExtensionWorkspace(successLine("扩展模式已切换为 " + modeName))
		return *m, nil
	case "script":
		if len(args) == 0 {
			m.openExtensionWorkspace("")
			return *m, nil
		}
		cfg, err := updateScriptPath(strings.Join(args, " "))
		if err != nil {
			m.openExtensionWorkspace(errorLine(err.Error()))
			return *m, nil
		}
		m.applyUpdatedConfig(cfg)
		m.openExtensionWorkspace(successLine("script_path 已更新"))
		return *m, nil
	case "chain", "residential":
		if len(args) == 0 {
			m.openChainWorkspace("")
			return *m, nil
		}
		switch strings.ToLower(args[0]) {
		case "server":
			if len(args) < 2 {
				m.openChainWorkspace(errorLine("用法: /chain server <住宅代理服务器>"))
				return *m, nil
			}
			cfg, err := updateChainServer(strings.Join(args[1:], " "))
			if err != nil {
				m.openChainWorkspace(errorLine(err.Error()))
				return *m, nil
			}
			m.applyUpdatedConfig(cfg)
			m.openChainWorkspace(successLine("住宅代理服务器已更新"))
			return *m, nil
		case "port":
			if len(args) < 2 {
				m.openChainWorkspace(errorLine("用法: /chain port <端口>"))
				return *m, nil
			}
			cfg, err := updateChainPort(args[1])
			if err != nil {
				m.openChainWorkspace(errorLine(err.Error()))
				return *m, nil
			}
			m.applyUpdatedConfig(cfg)
			m.openChainWorkspace(successLine("住宅代理端口已更新"))
			return *m, nil
		case "type":
			if len(args) < 2 {
				m.openChainWorkspace(errorLine("用法: /chain type socks5|http"))
				return *m, nil
			}
			cfg, err := updateChainProxyType(args[1])
			if err != nil {
				m.openChainWorkspace(errorLine(err.Error()))
				return *m, nil
			}
			m.applyUpdatedConfig(cfg)
			m.openChainWorkspace(successLine("住宅代理协议已更新"))
			return *m, nil
		case "airport":
			if len(args) < 2 {
				m.openChainWorkspace(errorLine("用法: /chain airport <机场组名称>"))
				return *m, nil
			}
			cfg, err := updateChainAirportGroup(strings.Join(args[1:], " "))
			if err != nil {
				m.openChainWorkspace(errorLine(err.Error()))
				return *m, nil
			}
			m.applyUpdatedConfig(cfg)
			m.openChainWorkspace(successLine("机场出口组已更新"))
			return *m, nil
		case "user":
			cfg, err := updateChainUsername(strings.Join(args[1:], " "))
			if err != nil {
				m.openChainWorkspace(errorLine(err.Error()))
				return *m, nil
			}
			m.applyUpdatedConfig(cfg)
			m.openChainWorkspace(successLine("住宅代理用户名已更新"))
			return *m, nil
		case "password", "pass":
			cfg, err := updateChainPassword(strings.Join(args[1:], " "))
			if err != nil {
				m.openChainWorkspace(errorLine(err.Error()))
				return *m, nil
			}
			m.applyUpdatedConfig(cfg)
			m.openChainWorkspace(successLine("住宅代理密码已更新"))
			return *m, nil
		default:
			m.openChainWorkspace(errorLine("支持: /chain server|port|type|airport|user|password"))
			return *m, nil
		}
	case "groups":
		fallthrough
	case "nodes", "node":
		return m.openGroupPicker()
	case "speed":
		return m.openGroupPicker()
	case "device":
		m.setDetail("设备接入", renderDeviceSetupLines(m.ip, loadConfigOrDefault().Runtime.Ports.API))
		m.focus = consoleFocusDetail
	case "logs", "log":
		m.setDetail("最近日志", m.captureLogLines(30))
		m.focus = consoleFocusDetail
	case "guide":
		m.setDetail("功能导航", renderGuideDetailLines(loadConfigOrDefault(), m.logFile))
		m.focus = consoleFocusDetail
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
		m.focus = consoleFocusDetail
	case "clear":
		m.refreshSelectionPreview()
		m.focus = consoleFocusNav
	case "restart":
		m.pending = &pendingConfirm{prompt: "确认重启网关？", action: consoleActionRestart}
		m.setDetail("确认重启", []string{noteLine("等待确认: 输入 y / n")})
		m.focus = consoleFocusDetail
	case "stop":
		m.pending = &pendingConfirm{prompt: "确认停止网关？", action: consoleActionStop}
		m.setDetail("确认停止", []string{noteLine("等待确认: 输入 y / n")})
		m.focus = consoleFocusDetail
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
		if m.pending != nil {
			m.pending = nil
			m.setInputValue("")
			m.focus = consoleFocusNav
			m.refreshSelectionPreview()
			return *m, nil
		}
		if m.prompt != nil {
			m.clearInputPrompt()
			m.focus = consoleFocusDetail
			m.refreshCurrentDetail()
			return *m, nil
		}
		m.focus = consoleFocusNav
		return *m, nil
	case "enter":
		value := strings.TrimSpace(m.inputValue)
		m.pushHistory(value)
		if m.prompt != nil {
			prompt := m.prompt
			m.clearInputPrompt()
			m.historyPos = -1
			m.focus = consoleFocusDetail
			if prompt.apply == nil {
				return *m, nil
			}
			if err := prompt.apply(m, value); err != nil {
				m.setDetail(prompt.title, []string{
					errorLine(err.Error()),
					"",
					noteLine("可再次按对应快捷键重新输入。"),
				})
				return *m, nil
			}
			return *m, nil
		}
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
		return "节点工作台：↑/↓ 选择，Enter 确认切换，T 测延迟，Esc 返回"
	}
	if m.focus == consoleFocusInput {
		return "弹窗输入：Enter 保存，Esc 取消"
	}
	if m.focus == consoleFocusHeader {
		return "顶部聚焦：←/→ 切换分区，↓ / Enter 进入左侧功能"
	}
	if m.focus == consoleFocusDetail {
		return "内容聚焦：↑/↓ 或鼠标滚轮滚动，Esc 返回左侧"
	}
	return "导航模式：↑/↓ 功能，Enter 打开右侧页，Esc 回顶部"
}

func refreshPulseTickCmd() tea.Cmd {
	return tea.Tick(45*time.Millisecond, func(time.Time) tea.Msg {
		return refreshPulseMsg{}
	})
}

func navAlertTickCmd() tea.Cmd {
	return tea.Tick(60*time.Millisecond, func(time.Time) tea.Msg {
		return navAlertPulseMsg{}
	})
}

func (m *runtimeConsoleModel) handleDetailWheel(button tea.MouseButton) (tea.Model, tea.Cmd) {
	if m.picker != pickerModeNone {
		switch button {
		case tea.MouseWheelUp:
			return m.handlePickerKey(tea.KeyPressMsg{Code: tea.KeyUp})
		case tea.MouseWheelDown:
			return m.handlePickerKey(tea.KeyPressMsg{Code: tea.KeyDown})
		default:
			return *m, nil
		}
	}

	switch button {
	case tea.MouseWheelUp:
		m.viewport.ScrollUp(3)
	case tea.MouseWheelDown:
		m.viewport.ScrollDown(3)
	}
	return *m, nil
}

func proxyDelayCmd(client *mihomo.Client, node, testURL string) tea.Cmd {
	return func() tea.Msg {
		delay, err := client.GetProxyDelay(node, testURL, 5*time.Second)
		return pickerDelayResultMsg{node: node, delay: delay, err: err}
	}
}

func pickerTestURL(group mihomo.ProxyGroup) string {
	if strings.TrimSpace(group.TestURL) != "" {
		return group.TestURL
	}
	return "http://www.gstatic.com/generate_204"
}

func (m runtimeConsoleModel) refreshPulseActive() bool {
	return m.refreshPulse > 0
}

func (m runtimeConsoleModel) navAlertActive() bool {
	return m.navAlertPulse > 0
}

func (m runtimeConsoleModel) refreshPulseOffset() int {
	if !m.refreshPulseActive() {
		return 0
	}
	if m.refreshPulse%2 == 0 {
		return 2
	}
	return 1
}

func (m runtimeConsoleModel) refreshPulseBorderColor(defaultColor string) string {
	if !m.refreshPulseActive() {
		return defaultColor
	}
	if m.refreshPulse%2 == 0 {
		return "#67e8f9"
	}
	return "#22d3ee"
}

func (m runtimeConsoleModel) refreshPulseTitleColor(defaultColor string) string {
	if !m.refreshPulseActive() {
		return defaultColor
	}
	if m.refreshPulse%2 == 0 {
		return "#ecfeff"
	}
	return "#cffafe"
}

func (m runtimeConsoleModel) refreshPulseHintLine() string {
	if !m.refreshPulseActive() {
		return ""
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#67e8f9")).
		Render("↻ 正在刷新当前页面")
}

func (m runtimeConsoleModel) refreshPulseBodyStyle() lipgloss.Style {
	if !m.refreshPulseActive() {
		return lipgloss.NewStyle()
	}
	if m.refreshPulse%2 == 0 {
		return lipgloss.NewStyle().PaddingLeft(1)
	}
	return lipgloss.NewStyle().PaddingRight(1)
}

func (m runtimeConsoleModel) navAlertOffset() int {
	if !m.navAlertActive() {
		return 0
	}
	if m.navAlertPulse%2 == 0 {
		return 1
	}
	return 0
}

func (m *runtimeConsoleModel) triggerNavBoundaryAlert() {
	m.navAlertPulse = navAlertFrames
}

func (m *runtimeConsoleModel) setInputValue(value string) {
	m.inputValue = value
	m.inputCursor = len([]rune(value))
}

func (m *runtimeConsoleModel) openInputPrompt(prompt *inputPrompt, currentValue string, detailLines []string) {
	if prompt == nil {
		return
	}
	m.prompt = prompt
	m.focus = consoleFocusInput
	m.setInputValue(currentValue)
	if len(detailLines) == 0 {
		detailLines = []string{
			renderSectionTitle(prompt.label),
			noteLine("会弹出输入框；按 Enter 保存，按 Esc 取消。"),
		}
	}
	m.setDetail(prompt.title, detailLines)
}

func (m *runtimeConsoleModel) clearInputPrompt() {
	m.prompt = nil
	m.setInputValue("")
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
			m.pickerStatus = "已返回分组列表"
			return m, nil
		}
		m.picker = pickerModeNone
		m.focus = consoleFocusNav
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
	case "t":
		if m.picker != pickerModeNodes || len(m.groups) == 0 {
			m.pickerStatus = "先进入节点列表，再按 T 测当前节点延迟"
			return m, nil
		}
		group := m.groups[m.groupCursor]
		if len(group.All) == 0 {
			return m, nil
		}
		target := group.All[m.nodeCursor]
		if m.nodeDelays == nil {
			m.nodeDelays = map[string]string{}
		}
		m.nodeDelays[target] = "测速中..."
		m.pickerStatus = "正在测速: " + target
		return m, proxyDelayCmd(m.client, target, pickerTestURL(group))
	case "enter":
		if m.picker == pickerModeGroups {
			if len(m.groups) == 0 {
				return m, nil
			}
			m.picker = pickerModeNodes
			m.nodeCursor = 0
			m.pickerStatus = "已进入节点列表：Enter 切换节点，T 测当前节点延迟"
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
				m.picker = pickerModeNone
				m.focus = consoleFocusDetail
				m.setDetail("节点工作台", renderNodeWorkspaceLines(m.snapshot, errorLine("切换失败: "+err.Error())))
			} else {
				m.pickerStatus = fmt.Sprintf("已切换节点: %s -> %s", group.Name, target)
				m.picker = pickerModeNone
				m.focus = consoleFocusDetail
				m.refreshSnapshot()
				m.setDetail("节点工作台", renderNodeWorkspaceLines(m.snapshot, successLine(m.pickerStatus)))
			}
			return m, nil
		}
	}

	return m, nil
}

func (m *runtimeConsoleModel) openGroupPicker() (tea.Model, tea.Cmd) {
	groups, err := m.client.ListProxyGroups()
	if err != nil {
		m.setDetail("节点分组读取失败", []string{errorLine("无法读取节点分组: " + err.Error())})
		m.focus = consoleFocusDetail
		return *m, nil
	}
	if len(groups) == 0 {
		m.setDetail("节点切换器", []string{noteLine("当前没有可切换的节点分组。")})
		m.focus = consoleFocusDetail
		return *m, nil
	}

	m.groups = groups
	m.groupCursor = 0
	m.nodeCursor = 0
	m.picker = pickerModeGroups
	m.focus = consoleFocusDetail
	m.detailTitle = "节点工作台"
	m.nodeDelays = map[string]string{}
	m.pickerStatus = "先选一个分组，回车进入节点列表；进入后按 T 测当前节点延迟"
	return *m, nil
}

func (m *runtimeConsoleModel) refreshSnapshot() {
	m.refreshDashboardSnapshot()
}

func (m *runtimeConsoleModel) resize() {
	headerHeight := lipgloss.Height(m.renderHeader())
	alertHeight := 0
	if alert := m.renderAlertBanner(); alert != "" {
		alertHeight = lipgloss.Height(alert)
	}
	footerHeight := lipgloss.Height(m.renderFooter())
	mainHeight := m.height - headerHeight - footerHeight - alertHeight
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
		Foreground(lipgloss.Color(m.refreshPulseTitleColor("#f8fafc")))
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
		border = lipgloss.Color(m.refreshPulseBorderColor("#22d3ee"))
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
	title := m.detailDisplayTitle()
	if title == "" {
		title = "当前内容"
	}
	content := m.viewport.View()
	border := "#334155"
	headerColor := lipgloss.Color("#e2e8f0")
	if m.picker != pickerModeNone {
		title = "节点工作台 · 可操作页"
		content = m.renderPicker(width-4, max(3, m.mainHeight-4))
		border = "#f59e0b"
		headerColor = lipgloss.Color("#fde68a")
	} else if m.focus == consoleFocusDetail {
		border = "#38bdf8"
	}
	if m.refreshPulseActive() && m.picker == pickerModeNone {
		border = m.refreshPulseBorderColor("#22d3ee")
		headerColor = lipgloss.Color(m.refreshPulseTitleColor("#e2e8f0"))
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(border)).
		Padding(0, 1).
		Width(width)

	header := lipgloss.NewStyle().Bold(true).Foreground(headerColor).Render(title)
	bodyHeight := max(3, m.mainHeight-4)
	body := lipgloss.NewStyle().
		MaxWidth(width - 4).
		Height(bodyHeight).
		MaxHeight(bodyHeight).
		Render(content)
	if m.refreshPulseActive() && m.picker == pickerModeNone {
		body = m.refreshPulseBodyStyle().Render(body)
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
	if m.navAlertActive() {
		lines = append(lines, "", errorLine("左右键只在顶部可用。先按 Esc 回顶部菜单。"))
	}
	lines = append(lines,
		"",
		renderSectionTitle("快捷键"),
		"Ctrl+P 切节点",
		"Enter 打开右侧页",
		"R 刷新摘要",
		"Esc 回顶部菜单",
		"Q 退出控制台（确认）",
	)

	title := "导航区"
	border := "#334155"
	if m.focus == consoleFocusNav && m.picker == pickerModeNone {
		title = "导航区 · 当前聚焦"
		border = "#38bdf8"
	}
	if m.navAlertActive() {
		title = "导航区 · 先按 Esc 回顶部"
		border = "#ef4444"
	} else if m.refreshPulseActive() {
		border = m.refreshPulseBorderColor(border)
	}
	shakeOffset := m.navAlertOffset()
	cardWidth := width - shakeOffset
	if cardWidth < 24 {
		cardWidth = width
		shakeOffset = 0
	}
	card := m.renderCard(cardWidth, title, lines, border)
	return lipgloss.NewStyle().
		Width(width).
		Height(max(8, m.mainHeight)).
		MaxHeight(max(8, m.mainHeight)).
		Render(lipgloss.NewStyle().PaddingLeft(shakeOffset).Render(card))
}

func (m runtimeConsoleModel) renderCard(width int, title string, lines []string, borderColor string) string {
	titleColor := "#7dd3fc"
	if borderColor == "#ef4444" {
		titleColor = "#fca5a5"
	} else if borderColor == "#f59e0b" {
		titleColor = "#fde68a"
	} else if m.refreshPulseActive() {
		titleColor = m.refreshPulseTitleColor(titleColor)
	}
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(titleColor))

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


func (m runtimeConsoleModel) renderAlertBanner() string {
	if len(m.snapshot.alerts) == 0 {
		return ""
	}
	alert := m.snapshot.alerts[0]
	border := "#ef4444"
	titleColor := "#fecaca"
	if alert.level != "error" {
		border = "#f59e0b"
		titleColor = "#fde68a"
	}
	lines := []string{
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(titleColor)).Render(alert.title),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#fee2e2")).Render(alert.body),
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(border)).
		Padding(0, 1).
		Width(max(36, m.width-2)).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m runtimeConsoleModel) renderFooter() string {
	title := "操作提示"
	border := "#334155"
	if m.focus == consoleFocusDetail {
		title = "内容区提示"
		border = "#38bdf8"
	}
	if m.focus == consoleFocusNav {
		title = "导航区提示"
		border = "#38bdf8"
	}
	if m.focus == consoleFocusHeader {
		title = "顶部菜单"
		border = "#38bdf8"
	}
	if m.refreshPulseActive() {
		border = m.refreshPulseBorderColor(border)
	}

	lines := []string{
		m.focusHint(),
	}
	if m.focus == consoleFocusNav {
		lines = append(lines, "左右键在左侧导航区不会切顶部菜单；按 Esc 回顶部后再切分区。")
	} else if m.focus == consoleFocusHeader {
		lines = append(lines, "首页默认聚焦顶部；先看首页，再决定进入左侧的代理或订阅工作台。")
	} else if m.focus == consoleFocusDetail {
		lines = append(lines, "长内容支持鼠标滚轮滚动；可操作页会在页面里直接写明按键。")
	}

	return m.renderCard(max(36, m.width-2), title, lines, border)
}

func (m runtimeConsoleModel) renderOverlay() string {
	if m.focus != consoleFocusInput {
		return ""
	}
	title := "输入"
	lines := make([]string, 0, 5)
	switch {
	case m.pending != nil:
		title = "确认操作"
		lines = append(lines,
			noteLine(m.pending.prompt),
			noteLine("输入 y 确认，输入 n 取消。"),
			"",
			m.renderInputLine(max(24, min(64, m.width-20))),
		)
	case m.prompt != nil:
		title = m.prompt.title
		lines = append(lines,
			renderSectionTitle(m.prompt.label),
			noteLine("输入新值后按 Enter 保存，按 Esc 取消。"),
			"",
			m.renderInputLine(max(24, min(64, m.width-20))),
		)
	default:
		return ""
	}

	cardWidth := min(max(48, m.width/2), m.width-8)
	card := m.renderCard(cardWidth, title, lines, "#38bdf8")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, card, lipgloss.WithWhitespaceChars(" "))
}

func (m runtimeConsoleModel) renderTabs() string {
	tabs := activeConsoleTabs()
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
		label := fmt.Sprintf("%s %s %s", item.key, renderItemKindTag(item.kind), item.title)
		if i == m.cursor {
			label = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7dd3fc")).Render("› " + label)
		} else {
			label = lipgloss.NewStyle().Foreground(lipgloss.Color("#cbd5e1")).Render("  " + label)
		}
		lines = append(lines, label)
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#64748b")).Render("    "+item.desc))
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#94a3b8")).Render("Enter 打开右侧页  ·  Ctrl+P 切节点"))
	return lines
}


func (m runtimeConsoleModel) renderHeaderSummary() string {
	parts := []string{
		"首页 " + truncateText(plainText(m.snapshot.overviewSummary), 42),
		"TUN " + onOff(m.snapshot.network.TunEnabled),
		"共享 " + onOff(m.snapshot.network.LanSharing),
		"出口 " + truncateText(plainText(m.snapshot.egressSummary), 30),
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
		placeholder := "例如 /status、/nodes、/chains setup"
		if m.pending != nil {
			placeholder = "输入 y 或 n"
		}
		if m.prompt != nil && strings.TrimSpace(m.prompt.placeholder) != "" {
			placeholder = m.prompt.placeholder
		}
		line := promptStyle.Render(prompt) + placeholderStyle.Render(placeholder)
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

func (m runtimeConsoleModel) detailDisplayTitle() string {
	title := strings.TrimSpace(m.detailTitle)
	if title == "" {
		return "当前内容"
	}
	if m.picker != pickerModeNone {
		return title
	}
	if strings.HasPrefix(title, "预览 · ") {
		return title
	}
	if strings.HasSuffix(title, "工作台") {
		return title + " · 可操作页"
	}
	switch title {
	case "确认重启", "确认停止", "确认退出":
		return title + " · 确认页"
	default:
		return title + " · 信息页"
	}
}

func renderItemKindTag(kind consoleItemKind) string {
	switch kind {
	case consoleItemAction:
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fde68a")).
			Background(lipgloss.Color("#3f2f10")).
			Padding(0, 1).
			Render("操作")
	case consoleItemConfirm:
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fecaca")).
			Background(lipgloss.Color("#3f1d1d")).
			Padding(0, 1).
			Render("确认")
	default:
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#bfdbfe")).
			Background(lipgloss.Color("#172554")).
			Padding(0, 1).
			Render("查看")
	}
}

func itemModeLabel(item consoleMenuItem) string {
	switch item.kind {
	case consoleItemAction:
		return "可操作页"
	case consoleItemConfirm:
		return "确认页"
	default:
		return "信息页"
	}
}

func (m *runtimeConsoleModel) nextTab() {
	tabs := activeConsoleTabs()
	current := 0
	for idx, tab := range tabs {
		if tab == m.tab {
			current = idx
			break
		}
	}
	m.tab = tabs[(current+1)%len(tabs)]
	m.cursor = 0
}

func (m *runtimeConsoleModel) prevTab() {
	tabs := activeConsoleTabs()
	current := 0
	for idx, tab := range tabs {
		if tab == m.tab {
			current = idx
			break
		}
	}
	current--
	if current < 0 {
		current = len(tabs) - 1
	}
	m.tab = tabs[current]
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
		renderSectionTitle(item.title + " · 预览"),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#94a3b8")).Render(item.desc),
		"",
		renderSectionTitle("当前摘要"),
	}
	lines = append(lines, previewLinesForItem(item, m.snapshot, m.cfg, m.logFile)...)

	enterHint := item.enter
	if enterHint == "" {
		enterHint = "进入 " + item.title
	}
	cmdHint := item.cmd
	if cmdHint == "" {
		cmdHint = "—"
	}
	lines = append(lines,
		"",
		renderSectionTitle("下一步"),
		"  回车: "+enterHint,
		"  命令: "+cmdHint,
	)
	m.setDetail("预览 · "+item.title, lines)
}

func (m *runtimeConsoleModel) refreshCurrentDetail() {
	cfg := loadConfigOrDefault()
	m.cfg = cfg

	switch m.detailTitle {
	case "首页":
		m.setDetail("首页", renderHomeDashboardLines(m.snapshot))
	case "连接与流量":
		m.setDetail("连接与流量", renderTrafficDetailLines(m.snapshot))
	case "IP 与延迟":
		m.setDetail("IP 与延迟", renderLatencyDetailLines(m.snapshot))
	case "出口网络":
		m.setDetail("出口网络", m.renderEgressStatusLines())
	case "扩展状态":
		m.setDetail("扩展状态", renderExtensionStatusLines(cfg))
	case detailTitleProxyWorkspace:
		m.setDetail(detailTitleProxyWorkspace, renderProxyWorkspaceLines(cfg, ""))
	case detailTitleRuntimeWorkspace:
		m.setDetail(detailTitleRuntimeWorkspace, renderRuntimeWorkspaceLines(cfg, ""))
	case detailTitleRulesWorkspace:
		m.setDetail(detailTitleRulesWorkspace, renderRulesWorkspaceLines(cfg, ""))
	case detailTitleSubscriptionWorkspace:
		m.setDetail(detailTitleSubscriptionWorkspace, renderSubscriptionWorkspaceLines(cfg, ""))
	case detailTitleExtensionWorkspace:
		m.setDetail(detailTitleExtensionWorkspace, renderExtensionWorkspaceLines(cfg, ""))
	case detailTitleChainWorkspace:
		m.setDetail(detailTitleChainWorkspace, renderChainWorkspaceLines(cfg, ""))
	case "节点工作台":
		m.setDetail("节点工作台", renderNodeWorkspaceLines(m.snapshot, noteLine("按回车进入节点工作台，或按 Ctrl+P 直接打开。")))
	case "功能导航":
		m.setDetail("功能导航", renderGuideDetailLines(cfg, m.logFile))
	default:
		if strings.HasPrefix(m.detailTitle, "预览 · ") {
			m.refreshSelectionPreview()
		} else {
			m.setDetail("首页", renderHomeDashboardLines(m.snapshot))
		}
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
	case "home_dashboard":
		m.setDetail("首页", renderHomeDashboardLines(m.snapshot))
		m.focus = consoleFocusDetail
	case "home_traffic":
		m.setDetail("连接与流量", renderTrafficDetailLines(m.snapshot))
		m.focus = consoleFocusDetail
	case "home_latency":
		m.setDetail("IP 与延迟", renderLatencyDetailLines(m.snapshot))
		m.focus = consoleFocusDetail
	case "proxy_nodes":
		return m.openGroupPicker()
	case "proxy_runtime":
		m.openRuntimeWorkspace("")
	case "proxy_rules":
		m.openRulesWorkspace("")
	case "proxy_egress":
		m.setDetail("出口网络", m.renderEgressStatusLines())
		m.focus = consoleFocusDetail
	case "subscription_manage":
		m.openSubscriptionWorkspace("")
	case "subscription_extension":
		m.openExtensionWorkspace("")
	case "subscription_chain":
		m.openChainWorkspace("")
	case "subscription_status":
		m.setDetail("扩展状态", renderExtensionStatusLines(loadConfigOrDefault()))
		m.focus = consoleFocusDetail
	case "subscription_proxy":
		m.openProxyWorkspace("")
	case "system_stop":
		m.pending = &pendingConfirm{prompt: "确认停止网关？", action: consoleActionStop}
		m.setDetail("确认停止", []string{noteLine("等待确认: 输入 y / n，或继续用方向键查看其他内容。")})
		m.focus = consoleFocusInput
		m.setInputValue("")
	default:
		m.refreshSelectionPreview()
	}
	return *m, nil
}

func menuItemsForTab(tab consoleTab) []consoleMenuItem {
	switch tab {
	case consoleTabRouting:
		return []consoleMenuItem{
			{id: "proxy_nodes", key: "01", title: "节点与分组", desc: "切换策略组与节点，并测当前节点延迟", kind: consoleItemAction, cmd: "Ctrl+P", enter: "进入节点工作台"},
			{id: "proxy_runtime", key: "02", title: "网络设置", desc: "管理 TUN、局域网共享和本机绕过代理", kind: consoleItemAction, cmd: "/tun", enter: "进入网络设置工作台"},
			{id: "proxy_rules", key: "03", title: "规则推荐", desc: "快速切换国内直连、广告拦截等推荐规则", kind: consoleItemAction, cmd: "/rule", enter: "进入规则工作台"},
			{id: "proxy_egress", key: "04", title: "出口网络", desc: "查看入口节点、出口 IP 和当前链路模式", kind: consoleItemInfo, cmd: "/status", enter: "打开出口网络详情"},
		}
	case consoleTabExtension:
		return []consoleMenuItem{
			{id: "subscription_manage", key: "01", title: "订阅管理", desc: "新建订阅、切换当前订阅，并维护 URL / 文件来源", kind: consoleItemAction, enter: "进入订阅管理工作台"},
			{id: "subscription_proxy", key: "02", title: "代理来源", desc: "快速调整当前订阅的 url / file 来源和名称", kind: consoleItemAction, cmd: "/proxy", enter: "进入代理来源工作台"},
			{id: "subscription_extension", key: "03", title: "扩展模式", desc: "在全局扩展脚本和一键 chains 之间二选一", kind: consoleItemAction, cmd: "/extension", enter: "进入扩展模式工作台"},
			{id: "subscription_chain", key: "04", title: "住宅代理", desc: "配置住宅代理、机场组以及 rule / global 模式", kind: consoleItemAction, cmd: "/chain", enter: "进入住宅代理工作台"},
			{id: "subscription_status", key: "05", title: "订阅与扩展状态", desc: "查看当前订阅、扩展脚本和 chains 生效状态", kind: consoleItemInfo, cmd: "/chains", enter: "打开订阅与扩展状态页"},
		}
	default:
		return []consoleMenuItem{
			{id: "home_dashboard", key: "01", title: "首页仪表盘", desc: "查看订阅、节点、TUN、流量、IP 和站点延迟总览", kind: consoleItemInfo, cmd: "/status", enter: "打开首页仪表盘"},
			{id: "home_traffic", key: "02", title: "连接与流量", desc: "查看最近 10 次刷新、上下行速度、连接和内核占用", kind: consoleItemInfo, enter: "打开连接与流量详情"},
			{id: "home_latency", key: "03", title: "IP 与延迟", desc: "查看当前 / 入口 / 出口 IP，以及常用站点延迟", kind: consoleItemInfo, enter: "打开 IP 与延迟详情"},
		}
	}
}

func previewLinesForItem(item consoleMenuItem, snap snapshot, cfg *config.Config, logFile string) []string {
	switch item.id {
	case "home_dashboard":
		return []string{
			"  订阅: " + fallbackText(snap.subscription.Name, "未设置"),
			"  工作模式: " + fallbackText(snap.node.PolicyMode, "规则"),
			"  当前节点: " + fallbackText(snap.node.Node, "未识别"),
			"  局域网共享: " + onOff(snap.network.LanSharing),
			"  出口摘要: " + plainText(snap.egressSummary),
		}
	case "home_traffic":
		return []string{
			fmt.Sprintf("  活跃连接: %d", snap.traffic.ActiveConnections),
			fmt.Sprintf("  上行速度: %s/s", ui.FormatBytes(snap.traffic.UploadSpeed)),
			fmt.Sprintf("  下行速度: %s/s", ui.FormatBytes(snap.traffic.DownloadSpeed)),
			"  上行趋势: " + renderTrendSparkline(snap.traffic.UploadTrend),
			"  下行趋势: " + renderTrendSparkline(snap.traffic.DownloadTrend),
		}
	case "home_latency":
		return []string{
			"  当前 IP: " + fallbackText(snap.addresses.Current, "未获取"),
			"  入口 IP: " + fallbackText(snap.addresses.Entry, "未获取"),
			"  出口 IP: " + fallbackText(snap.addresses.Exit, "未获取"),
			"  YouTube: " + fallbackText(snap.latency.Sites["YouTube"], "未测"),
			"  GitHub: " + fallbackText(snap.latency.Sites["GitHub"], "未测"),
		}
	case "proxy_runtime":
		return []string{
			"  TUN: " + tuiOnOff(cfg.Runtime.Tun.Enabled),
			"  TUN 接口: " + fallbackText(snap.network.TunInterface, "未就绪"),
			"  局域网共享: " + tuiState(snap.network.LanSharing, "可用", "不可用"),
			"  本机绕过代理: " + tuiOnOff(cfg.Runtime.Tun.BypassLocal),
		}
	case "proxy_rules":
		return []string{
			"  国内直连: " + tuiOnOff(cfg.Rules.ChinaDirectEnabled()),
			"  广告拦截: " + tuiOnOff(cfg.Rules.AdsRejectEnabled()),
			"  Apple 规则: " + tuiOnOff(cfg.Rules.AppleRulesEnabled()),
			"  Nintendo 代理: " + tuiOnOff(cfg.Rules.NintendoProxyEnabled()),
		}
	case "proxy_nodes":
		return []string{
			"  当前策略组: " + fallbackText(snap.node.Group, "-"),
			"  当前策略: " + fallbackText(snap.node.Strategy, "未识别"),
			"  当前节点: " + fallbackText(snap.node.Node, "未识别"),
			"  说明: 进入后可切节点并测速",
		}
	case "proxy_egress":
		return []string{
			"  当前出口: " + fallbackText(snap.addresses.Current, "未获取"),
			"  入口 IP: " + fallbackText(snap.addresses.Entry, "未获取"),
			"  出口 IP: " + fallbackText(snap.addresses.Exit, "未获取"),
			"  链路摘要: " + plainText(snap.egressSummary),
		}
	case "subscription_manage":
		return []string{
			"  当前订阅: " + fallbackText(snap.subscription.Name, "未设置"),
			"  来源: " + fallbackText(snap.subscription.Source, "未设置"),
			"  概览: " + compactProfileList(cfg),
			"  来源站点: " + fallbackText(snap.subscription.SourceHost, "未获取"),
		}
	case "subscription_proxy":
		return []string{
			"  当前来源: " + fallbackText(activeProxyProfile(cfg).Source, "未设置"),
			"  当前名称: " + fallbackText(activeProxyProfile(cfg).Name, "未设置"),
			"  URL / 文件: " + shortText(fallbackText(activeProxyProfile(cfg).SubscriptionURL, activeProxyProfile(cfg).ConfigFile), 60),
			"  说明: 进入后可直接切 url / file 并修改当前来源",
		}
	case "subscription_extension":
		chainMode := "rule"
		if cfg.Extension.ResidentialChain != nil && strings.TrimSpace(cfg.Extension.ResidentialChain.Mode) != "" {
			chainMode = cfg.Extension.ResidentialChain.Mode
		}
		return []string{
			"  扩展模式: " + extensionModeName(cfg.Extension.Mode),
			"  chains 路由: " + chainMode,
			"  script_path: " + shortText(cfg.Extension.ScriptPath, 48),
			"  说明: 进入后可在 script 和 chains 之间二选一",
		}
	case "subscription_chain":
		chain := ensureConsoleChain(cfg)
		return []string{
			fmt.Sprintf("  住宅代理: %s:%d (%s)", fallbackText(chain.ProxyServer, "未设置"), chain.ProxyPort, fallbackText(chain.ProxyType, "socks5")),
			"  机场组: " + fallbackText(chain.AirportGroup, "Auto"),
			"  chains 模式: " + fallbackText(chain.Mode, "rule"),
			"  说明: 进入后可修改住宅代理、机场组和协议",
		}
	case "subscription_status":
		return []string{
			"  当前订阅: " + fallbackText(snap.subscription.Name, "未设置"),
			"  当前扩展: " + extensionModeName(cfg.Extension.Mode),
			"  住宅出口: " + fallbackText(snap.addresses.Exit, "未获取"),
			"  面板入口: " + snap.panelURL,
		}
	default:
		return []string{
			"  模式: " + plainText(snap.modeSummary),
			"  节点: " + fallbackText(snap.currentNode, "未识别"),
			"  配置文件: " + snap.configPath,
			"  日志文件: " + logFile,
		}
	}
}

func activeTabDescription(tab consoleTab) string {
	switch tab {
	case consoleTabRouting:
		return "节点、TUN、局域网共享和规则"
	case consoleTabExtension:
		return "订阅档案、扩展脚本和一键 chains"
	default:
		return "首页总览：订阅、节点、TUN、流量、IP 和延迟"
	}
}

func tabLabel(tab consoleTab) string {
	switch tab {
	case consoleTabRouting:
		return "代理"
	case consoleTabExtension:
		return "订阅"
	default:
		return "首页"
	}
}

func defaultSuggestionsForTab(tab consoleTab) []string {
	switch tab {
	case consoleTabRouting:
		return []string{"/nodes", "/tun", "/rules", "/status"}
	case consoleTabExtension:
		return []string{"/subscription", "/extension", "/chain", "/chains"}
	default:
		return []string{"/status", "/summary", "/nodes", "/chains"}
	}
}

func activeConsoleTabs() []consoleTab {
	return []consoleTab{consoleTabOverview, consoleTabRouting, consoleTabExtension}
}

func consoleCommandSuggestions() []string {
	base := []string{
		"/help",
		"/status",
		"/summary",
		"/config",
		"/config open",
		"/proxy",
		"/proxy source url",
		"/proxy source file",
		"/proxy url",
		"/proxy file",
		"/proxy name",
		"/tun",
		"/tun on",
		"/tun off",
		"/bypass",
		"/bypass on",
		"/bypass off",
		"/rules",
		"/rule china",
		"/rule ads",
		"/chains",
		"/extension",
		"/extension chains",
		"/extension script",
		"/extension off",
		"/chain",
		"/chain server",
		"/chain port",
		"/chain airport",
		"/chains setup",
		"/nodes",
		"/speed",
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
		lines := []string{
			renderSectionTitle("当前阶段"),
			"  先选择一个节点分组，然后回车进入节点列表。",
			"  进入节点列表后，可以切换节点，也可以按 T 测当前节点延迟。",
		}
		if strings.TrimSpace(m.pickerStatus) != "" {
			lines = append(lines, "", noteLine(m.pickerStatus))
		}
		lines = append(lines, "", renderSectionTitle("节点分组"))
		for i, group := range m.groups {
			cursor := "  "
			style := lipgloss.NewStyle().Foreground(lipgloss.Color("#cbd5e1"))
			if i == m.groupCursor {
				cursor = "› "
				style = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7dd3fc"))
			}
			lines = append(lines, style.Render(fmt.Sprintf("%s%s  [%s]  当前: %s", cursor, group.Name, group.Type, group.Now)))
		}
		lines = append(lines, "", noteLine("Enter 进入节点列表  ·  Esc 返回左侧"))
		return lipgloss.NewStyle().
			Height(height).
			MaxHeight(height).
			Render(strings.Join(lines, "\n"))
	}

	group := m.groups[m.groupCursor]
	lines := []string{
		renderSectionTitle("当前分组"),
		fmt.Sprintf("  节点分组: %s", group.Name),
		fmt.Sprintf("  当前节点: %s", group.Now),
		fmt.Sprintf("  测试地址: %s", shortText(pickerTestURL(group), 56)),
		"",
		renderSectionTitle("节点列表"),
	}
	if strings.TrimSpace(m.pickerStatus) != "" {
		lines = append(lines, noteLine(m.pickerStatus), "")
	}
	for i, node := range group.All {
		cursor := "  "
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("#cbd5e1"))
		if i == m.nodeCursor {
			cursor = "› "
			style = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f59e0b"))
		}
		delayInfo := "未测速"
		if value := strings.TrimSpace(m.nodeDelays[node]); value != "" {
			delayInfo = value
		}
		if node == group.Now {
			node += "  (current)"
		}
		lines = append(lines, style.Render(fmt.Sprintf("%s%s  ·  %s", cursor, truncateText(node, 24), delayInfo)))
	}
	lines = append(lines, "", noteLine("Enter 切换节点  ·  T 测当前节点延迟  ·  Esc 返回分组"))
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
	out = append(out, noteLine("实时查看: "+followLogCommand(m.logFile)))
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
		noteLine("输入 /proxy 进入代理来源工作台"),
		noteLine("输入 /tun 或 /bypass 调整运行模式"),
		noteLine("输入 /rules 切换推荐规则"),
		noteLine("输入 /extension / /chain 管理 chains / script / 住宅代理"),
		noteLine("输入 /nodes 切换节点"),
		noteLine("输入 /chains setup 打开完整链式代理向导"),
		noteLine("输入 /config open 打开完整交互式配置中心"),
		"",
		renderSectionTitle("说明"),
		noteLine("常用配置现在可以直接在 TUI 和简单模式中修改。"),
		noteLine("需要问答式引导时，再使用 /config open。"),
	}
	return lines
}

func renderNodeWorkspaceLines(snap snapshot, statusLine string) []string {
	lines := []string{
		renderSectionTitle("节点工作台"),
		"  这是一个可操作页：进入后可以切换节点，也可以测当前节点延迟。",
		fmt.Sprintf("  当前节点: %s", snap.currentNode),
		fmt.Sprintf("  当前出口: %s", plainText(snap.egressSummary)),
	}
	if strings.TrimSpace(statusLine) != "" {
		lines = append(lines, "", statusLine)
	}
	lines = append(lines,
		"",
		renderSectionTitle("常用操作"),
		noteLine("回车进入节点工作台"),
		noteLine("进入后按 T 测当前节点延迟"),
		noteLine("也可以直接按 Ctrl+P 打开"),
	)
	return lines
}

func renderDeviceSetupLines(ip string, apiPort int) []string {
	return []string{
		renderSectionTitle("设备接入"),
		"  网关 (Gateway): " + ip,
		"  DNS: " + ip,
		"  IP: 同网段任意可用 IP",
		"  子网掩码: 255.255.255.0",
		"",
		renderSectionTitle("验证方式"),
		"  手机上优先验证浏览器 / YouTube / App Store。",
		"  游戏机和电视可优先验证 eShop / PSN / 流媒体。",
		"",
		noteLine(fmt.Sprintf("API 面板: http://%s:%d/ui", ip, apiPort)),
	}
}

func renderExtensionStatusLines(cfg *config.Config) []string {
	lines := []string{
		renderSectionTitle("扩展状态"),
		"  模式: " + extensionModeName(cfg.Extension.Mode),
	}
	switch cfg.Extension.Mode {
	case "chains":
		if cfg.Extension.ResidentialChain != nil {
			lines = append(lines,
				"  链式模式: "+fallbackText(cfg.Extension.ResidentialChain.Mode, "rule"),
				"  住宅代理: "+fmt.Sprintf("%s:%d (%s)", cfg.Extension.ResidentialChain.ProxyServer, cfg.Extension.ResidentialChain.ProxyPort, cfg.Extension.ResidentialChain.ProxyType),
				"  机场组: "+fallbackText(cfg.Extension.ResidentialChain.AirportGroup, "Auto"),
			)
		} else {
			lines = append(lines, errorLine("chains 已启用，但 residential_chain 配置为空"))
		}
	case "script":
		lines = append(lines, "  脚本路径: "+fallbackText(cfg.Extension.ScriptPath, "未配置"))
	default:
		lines = append(lines, noteLine("当前未启用扩展模式。可用 /chains setup 打开内置链式代理向导。"))
	}

	lines = append(lines,
		"",
		renderSectionTitle("快捷入口"),
		noteLine("/chains 查看当前状态"),
		noteLine("/chains setup 进入链式代理向导"),
		noteLine("/config open 打开完整配置中心"),
	)
	return lines
}

func renderGuideDetailLines(cfg *config.Config, logFile string) []string {
	lines := []string{
		renderSectionTitle("当前主线"),
	}
	if cfg.Runtime.Tun.Enabled {
		lines = append(lines, "  局域网共享已就绪：手机、Switch、PS5、Apple TV 改网关和 DNS 就能接入")
	} else {
		lines = append(lines, fmt.Sprintf("  先开启 TUN：运行 %s，再执行 %s", elevatedCmd("tun on"), elevatedCmd("restart")))
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
		"  "+followLogCommand(logFile),
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
