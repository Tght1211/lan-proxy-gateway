package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

const (
	runtimeConsoleNodeRefreshTTL   = 8 * time.Second
	runtimeConsoleNoticeRefreshTTL = 10 * time.Minute
)

type runtimeConsoleNodeCache struct {
	mu        sync.Mutex
	value     string
	hasValue  bool
	updatedAt time.Time
	fetching  bool
}

type runtimeConsoleNoticeCache struct {
	mu       sync.Mutex
	value    *updateNotice
	loadedAt time.Time
	loaded   bool
	fetching bool
}

var (
	runtimeNodeCache   runtimeConsoleNodeCache
	runtimeNoticeCache runtimeConsoleNoticeCache
)

func runMenuRuntimeConsole(logFile, ip string) consoleAction {
	reader := bufio.NewReader(os.Stdin)

	// 首次渲染前预热节点缓存，避免显示静态"加载中..."
	prewarmConsoleNodeCacheWithSpinner(loadConfigOrDefault())

	for {
		clearInteractiveScreen()

		cfg := loadConfigOrDefault()
		ui.ShowLogo()
		color.New(color.Bold).Println("运行控制台")
		fmt.Println()
		for _, line := range renderRuntimeConsoleHomeLines(cfg, ip) {
			if strings.TrimSpace(line) == "" {
				fmt.Println()
				continue
			}
			fmt.Printf("  %s\n", line)
		}

		ui.Separator()
		fmt.Println()
		fmt.Println("  1) 查看运行状态")
		fmt.Println("  2) 节点与策略组")
		fmt.Println("  3) 订阅与代理来源")
		fmt.Println("  4) 网络设置（TUN / 本机绕过）")
		fmt.Println("  5) 规则开关")
		fmt.Println("  6) 扩展模式 / 住宅代理")
		fmt.Println("  7) 配置中心")
		fmt.Println("  8) 设备接入说明")
		fmt.Println("  9) 最近日志")
		fmt.Println(" 10) 功能导航")
		fmt.Println(" 11) 升级提示")
		fmt.Println(" 12) 当前配置摘要")
		fmt.Println("  R) 重启网关")
		fmt.Println("  S) 停止网关")
		fmt.Println("  0) 退出控制台（网关继续运行）")
		fmt.Println()

		switch promptMenuChoice(reader, "请选择: ") {
		case "1":
			showOutputScreen(reader, "运行状态", func() {
				runStatus(nil, nil)
			})
		case "2":
			runSimpleGroupChooser(reader, cfg)
		case "3":
			runSubscriptionHubMenu(reader)
		case "4":
			runRuntimeWorkspaceMenu(reader)
		case "5":
			runRulesWorkspaceMenu(reader)
		case "6":
			runExtensionHubMenu(reader)
		case "7":
			clearInteractiveScreen()
			runConfigMenu(nil, nil)
		case "8":
			showOutputScreen(reader, "设备接入说明", func() {
				printDeviceSetupPanel(ip, loadConfigOrDefault().Runtime.Ports.API)
			})
		case "9":
			showLogsScreen(reader, logFile)
		case "10":
			showOutputScreen(reader, "功能导航", func() {
				printStartGuide(loadConfigOrDefault(), logFile)
			})
		case "11":
			showUpdateScreen(reader)
		case "12":
			showOutputScreen(reader, "当前配置摘要", func() {
				printConfigSummary(loadConfigOrDefault())
			})
		case "r":
			if confirmMenuAction(reader, "确认重启网关？") {
				return consoleActionRestart
			}
		case "s":
			if confirmMenuAction(reader, "确认停止网关？") {
				return consoleActionStop
			}
		case "0", "q", "quit", "exit":
			clearInteractiveScreen()
			fmt.Println("  已退出控制台，网关保持运行。")
			fmt.Printf("  重新进入: %s\n", elevatedCmd("console"))
			fmt.Println()
			return consoleActionExit
		default:
			fmt.Println("  请输入菜单编号。")
			fmt.Println()
			waitEnter(reader)
		}
	}
}

func renderRuntimeConsoleHomeLines(cfg *config.Config, ip string) []string {
	profile := activeProxyProfile(cfg)
	lines := []string{
		"当前入口: 菜单式 CLI 控制台",
		"网关 / DNS: " + fallbackText(ip, "未识别"),
		"当前节点: " + currentConsoleNodeCached(cfg),
		fmt.Sprintf("代理来源: %s (%s)", fallbackText(profile.Name, "subscription"), fallbackText(profile.Source, "未设置")),
		fmt.Sprintf("运行模式: TUN %s · bypass %s · %s", consoleOnOff(cfg.Runtime.Tun.Enabled), consoleOnOff(cfg.Runtime.Tun.BypassLocal), extensionModeName(cfg.Extension.Mode)),
		"配置文件: " + displayConfigPath(),
	}

	if notice := loadUpdateNoticeCached(); notice != nil {
		lines = append(lines, noteLine(fmt.Sprintf("发现新版本 %s，菜单 11 可查看升级提示。", notice.Latest)))
	}
	return lines
}

func currentConsoleNodeCached(cfg *config.Config) string {
	now := time.Now()

	runtimeNodeCache.mu.Lock()
	if runtimeNodeCache.hasValue && now.Sub(runtimeNodeCache.updatedAt) < runtimeConsoleNodeRefreshTTL {
		value := runtimeNodeCache.value
		runtimeNodeCache.mu.Unlock()
		return value
	}
	if runtimeNodeCache.fetching {
		if runtimeNodeCache.hasValue {
			value := runtimeNodeCache.value
			runtimeNodeCache.mu.Unlock()
			return value
		}
		runtimeNodeCache.mu.Unlock()
		return "加载中..."
	}
	if runtimeNodeCache.hasValue {
		value := runtimeNodeCache.value
		runtimeNodeCache.fetching = true
		runtimeNodeCache.mu.Unlock()
		go refreshCurrentConsoleNode(cfg)
		return value
	}
	runtimeNodeCache.fetching = true
	runtimeNodeCache.mu.Unlock()
	go refreshCurrentConsoleNode(cfg)
	return "加载中..."
}

// prewarmConsoleNodeCacheWithSpinner 在首次渲染前同步等待节点信息，显示 spinner 动画。
// 超过 2.5 秒则跳过，避免长时间阻塞。
func prewarmConsoleNodeCacheWithSpinner(cfg *config.Config) {
	runtimeNodeCache.mu.Lock()
	if runtimeNodeCache.hasValue {
		runtimeNodeCache.mu.Unlock()
		return
	}
	if !runtimeNodeCache.fetching {
		runtimeNodeCache.fetching = true
		runtimeNodeCache.mu.Unlock()
	} else {
		runtimeNodeCache.mu.Unlock()
	}

	done := make(chan string, 1)
	go func() {
		done <- currentConsoleNode(cfg)
	}()

	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(2500 * time.Millisecond)

	i := 0
	fmt.Printf("\r  %s 正在连接网关...", frames[0])
	for {
		select {
		case value := <-done:
			fmt.Print("\r\033[K")
			storeCurrentConsoleNode(value)
			return
		case <-timeout:
			fmt.Print("\r\033[K")
			// 超时：不覆盖 fetching 状态，让已有 goroutine 继续；
			// 下次刷新时缓存自然更新。
			runtimeNodeCache.mu.Lock()
			if !runtimeNodeCache.hasValue {
				runtimeNodeCache.value = "未获取"
				runtimeNodeCache.hasValue = true
				runtimeNodeCache.updatedAt = time.Now()
				runtimeNodeCache.fetching = false
			}
			runtimeNodeCache.mu.Unlock()
			return
		case <-ticker.C:
			i++
			fmt.Printf("\r  %s 正在连接网关...", frames[i%len(frames)])
		}
	}
}

func refreshCurrentConsoleNode(cfg *config.Config) {
	value := currentConsoleNode(cfg)
	storeCurrentConsoleNode(value)
}

func storeCurrentConsoleNode(value string) {
	runtimeNodeCache.mu.Lock()
	defer runtimeNodeCache.mu.Unlock()
	runtimeNodeCache.value = fallbackText(value, "未获取")
	runtimeNodeCache.hasValue = true
	runtimeNodeCache.updatedAt = time.Now()
	runtimeNodeCache.fetching = false
}

func loadUpdateNoticeCached() *updateNotice {
	now := time.Now()

	runtimeNoticeCache.mu.Lock()
	if runtimeNoticeCache.loaded && now.Sub(runtimeNoticeCache.loadedAt) < runtimeConsoleNoticeRefreshTTL {
		value := runtimeNoticeCache.value
		runtimeNoticeCache.mu.Unlock()
		return value
	}
	if runtimeNoticeCache.fetching {
		value := runtimeNoticeCache.value
		runtimeNoticeCache.mu.Unlock()
		return value
	}
	if runtimeNoticeCache.loaded {
		value := runtimeNoticeCache.value
		runtimeNoticeCache.fetching = true
		runtimeNoticeCache.mu.Unlock()
		go refreshUpdateNoticeCache()
		return value
	}
	runtimeNoticeCache.fetching = true
	runtimeNoticeCache.mu.Unlock()

	value := loadUpdateNotice()
	storeUpdateNotice(value)
	return value
}

func refreshUpdateNoticeCache() {
	storeUpdateNotice(loadUpdateNotice())
}

func storeUpdateNotice(value *updateNotice) {
	runtimeNoticeCache.mu.Lock()
	defer runtimeNoticeCache.mu.Unlock()
	runtimeNoticeCache.value = value
	runtimeNoticeCache.loaded = true
	runtimeNoticeCache.loadedAt = time.Now()
	runtimeNoticeCache.fetching = false
}

func currentConsoleNode(cfg *config.Config) string {
	client := newConsoleClient(cfg)
	if client == nil || !client.IsAvailable() {
		return "未获取"
	}
	group, err := client.GetProxyGroup("Proxy")
	if err != nil {
		return "未获取"
	}
	if strings.TrimSpace(group.Now) == "" {
		return "未获取"
	}
	return group.Now
}

func runSubscriptionHubMenu(reader *bufio.Reader) {
	for {
		clearInteractiveScreen()
		cfg := loadConfigOrDefault()
		lines := []string{
			renderSectionTitle("当前摘要"),
			"  当前订阅: " + fallbackText(activeProxyProfile(cfg).Name, "subscription"),
			"  当前来源: " + fallbackText(activeProxyProfile(cfg).Source, "未设置"),
			"  订阅概览: " + compactProfileList(cfg),
			"",
			renderSectionTitle("菜单"),
			"  1 订阅管理",
			"  2 代理来源",
			"  0 返回上一级",
		}
		printSimpleDetail("订阅与代理来源", lines)

		switch promptMenuChoice(reader, "请选择 [0-2]: ") {
		case "1":
			runSubscriptionWorkspaceMenu(reader)
		case "2":
			runProxyWorkspaceMenu(reader)
		case "0":
			return
		default:
			fmt.Println("  请输入 0-2。")
			fmt.Println()
			waitEnter(reader)
		}
	}
}

func runSubscriptionWorkspaceMenu(reader *bufio.Reader) {
	status := ""

	for {
		cfg := loadConfigOrDefault()
		clearInteractiveScreen()
		printSimpleDetail("订阅管理", renderSubscriptionWorkspaceLines(cfg, status))

		choice := promptMenuChoice(reader, "请选择 [0-6]: ")
		status = ""

		switch choice {
		case "1":
			name, ok := promptSimpleValue(reader, "输入新订阅名称", "", false)
			if !ok {
				continue
			}
			value, ok := promptSimpleValue(reader, "输入订阅链接", "", false)
			if !ok {
				continue
			}
			_, err := createSubscriptionProfile(name, "url", value)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("已新建并切换到 URL 订阅: " + name)
			}
		case "2":
			name, ok := promptSimpleValue(reader, "输入新订阅名称", "", false)
			if !ok {
				continue
			}
			value, ok := promptSimpleValue(reader, "输入本地配置文件路径", "", false)
			if !ok {
				continue
			}
			_, err := createSubscriptionProfile(name, "file", value)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("已新建并切换到本地文件订阅: " + name)
			}
		case "3":
			name, ok := chooseSubscriptionProfile(reader, cfg)
			if !ok {
				continue
			}
			_, err := switchSubscriptionProfile(name)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("已切换当前订阅: " + name)
			}
		case "4":
			value, ok := promptSimpleValue(reader, "输入当前订阅链接", activeProxyProfile(cfg).SubscriptionURL, false)
			if !ok {
				continue
			}
			_, err := updateSubscriptionURL(value)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("当前订阅链接已更新")
			}
		case "5":
			value, ok := promptSimpleValue(reader, "输入当前本地配置文件路径", activeProxyProfile(cfg).ConfigFile, false)
			if !ok {
				continue
			}
			_, err := updateProxyConfigFile(value)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("当前本地配置文件路径已更新")
			}
		case "6":
			value, ok := promptSimpleValue(reader, "输入当前订阅名称", activeProxyProfile(cfg).Name, false)
			if !ok {
				continue
			}
			_, err := updateSubscriptionName(value)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("当前订阅已重命名")
			}
		case "0":
			return
		default:
			status = errorLine("请输入 0-6。")
		}
	}
}

func chooseSubscriptionProfile(reader *bufio.Reader, cfg *config.Config) (string, bool) {
	profiles := listProxyProfiles(cfg)
	if len(profiles) == 0 {
		return "", false
	}

	status := ""
	for {
		clearInteractiveScreen()
		lines := []string{
			renderSectionTitle("可用订阅"),
		}
		for idx, profile := range profiles {
			current := ""
			if profile.Name == cfg.Proxy.CurrentProfile {
				current = "（当前）"
			}
			lines = append(lines, fmt.Sprintf("  %d %s · %s%s", idx+1, profile.Name, profile.Source, current))
		}
		if status != "" {
			lines = append(lines, "", status)
		}
		lines = append(lines,
			"",
			renderSectionTitle("菜单"),
			fmt.Sprintf("  1-%d 选择订阅", len(profiles)),
			"  0 返回上一级",
		)
		printSimpleDetail("切换订阅", lines)

		choice := promptMenuChoice(reader, fmt.Sprintf("请选择 [0-%d]: ", len(profiles)))
		if choice == "0" {
			return "", false
		}

		index := parseIndex(choice, len(profiles))
		if index >= 0 {
			return profiles[index].Name, true
		}
		status = errorLine("请输入有效的订阅编号。")
	}
}

func runProxyWorkspaceMenu(reader *bufio.Reader) {
	status := ""

	for {
		cfg := loadConfigOrDefault()
		clearInteractiveScreen()
		printSimpleDetail("代理来源", renderProxyWorkspaceLines(cfg, status))

		choice := promptMenuChoice(reader, "请选择 [0-5 / U/F/N/D]: ")
		status = ""

		switch strings.ToLower(choice) {
		case "1":
			_, err := updateProxySource("url")
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("代理来源已切换为 url")
			}
		case "2":
			_, err := updateProxySource("file")
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("代理来源已切换为 file")
			}
		case "3":
			_, err := updateProxySource("proxy")
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("代理来源已切换为 proxy")
			}
		case "u":
			value, ok := promptSimpleValue(reader, "输入订阅链接", activeProxyProfile(cfg).SubscriptionURL, false)
			if !ok {
				continue
			}
			_, err := updateSubscriptionURL(value)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("订阅链接已更新")
			}
		case "f":
			value, ok := promptSimpleValue(reader, "输入本地配置文件路径", activeProxyProfile(cfg).ConfigFile, false)
			if !ok {
				continue
			}
			_, err := updateProxyConfigFile(value)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("本地配置文件路径已更新")
			}
		case "n":
			value, ok := promptSimpleValue(reader, "输入订阅名称", activeProxyProfile(cfg).Name, false)
			if !ok {
				continue
			}
			_, err := updateSubscriptionName(value)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("订阅名称已更新")
			}
		case "d":
			runDirectProxyWorkspaceMenu(reader)
		case "0":
			return
		default:
			status = errorLine("请输入 0-5 或 U/F/N/D。")
		}
	}
}

func runRuntimeWorkspaceMenu(reader *bufio.Reader) {
	status := ""

	for {
		cfg := loadConfigOrDefault()
		clearInteractiveScreen()
		printSimpleDetail("网络设置", renderRuntimeWorkspaceLines(cfg, status))

		choice := promptMenuChoice(reader, "请选择 [0-2]: ")
		status = ""

		switch choice {
		case "1":
			_, err := updateTunEnabled(!cfg.Runtime.Tun.Enabled)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("TUN 已切换为 " + consoleOnOff(!cfg.Runtime.Tun.Enabled))
			}
		case "2":
			_, err := updateBypassLocal(!cfg.Runtime.Tun.BypassLocal)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("本机绕过代理已切换为 " + consoleOnOff(!cfg.Runtime.Tun.BypassLocal))
			}
		case "0":
			return
		default:
			status = errorLine("请输入 0-2。")
		}
	}
}

func runRulesWorkspaceMenu(reader *bufio.Reader) {
	status := ""

	for {
		cfg := loadConfigOrDefault()
		clearInteractiveScreen()
		printSimpleDetail("规则开关", renderRulesWorkspaceLines(cfg, status))

		choice := promptMenuChoice(reader, "请选择 [0-6]: ")
		status = ""

		rules := map[string]struct {
			name    string
			label   string
			enabled bool
		}{
			"1": {name: "lan", label: "局域网直连", enabled: cfg.Rules.LanDirectEnabled()},
			"2": {name: "china", label: "国内直连", enabled: cfg.Rules.ChinaDirectEnabled()},
			"3": {name: "apple", label: "Apple 规则", enabled: cfg.Rules.AppleRulesEnabled()},
			"4": {name: "nintendo", label: "Nintendo 代理", enabled: cfg.Rules.NintendoProxyEnabled()},
			"5": {name: "global", label: "国外代理", enabled: cfg.Rules.GlobalProxyEnabled()},
			"6": {name: "ads", label: "广告拦截", enabled: cfg.Rules.AdsRejectEnabled()},
		}

		switch choice {
		case "0":
			return
		case "1", "2", "3", "4", "5", "6":
			target := rules[choice]
			_, err := updateRuleToggle(target.name, !target.enabled)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine(target.label + " 已切换为 " + consoleOnOff(!target.enabled))
			}
		default:
			status = errorLine("请输入 0-6。")
		}
	}
}

func runExtensionHubMenu(reader *bufio.Reader) {
	for {
		clearInteractiveScreen()
		cfg := loadConfigOrDefault()
		lines := []string{
			renderSectionTitle("当前摘要"),
			"  扩展模式: " + extensionModeName(cfg.Extension.Mode),
			"  chains 路由模式: " + func() string {
				if cfg.Extension.ResidentialChain == nil {
					return "rule"
				}
				return fallbackText(cfg.Extension.ResidentialChain.Mode, "rule")
			}(),
			"  script_path: " + fallbackText(cfg.Extension.ScriptPath, "未设置"),
			"",
			renderSectionTitle("菜单"),
			"  1 扩展模式",
			"  2 住宅代理",
			"  3 查看当前扩展状态",
			"  4 打开 chains 配置向导",
			"  0 返回上一级",
		}
		printSimpleDetail("扩展模式 / 住宅代理", lines)

		switch promptMenuChoice(reader, "请选择 [0-4]: ") {
		case "1":
			runExtensionWorkspaceMenu(reader)
		case "2":
			runChainWorkspaceMenu(reader)
		case "3":
			showOutputScreen(reader, "当前扩展状态", func() {
				runChainsStatus(nil, nil)
			})
		case "4":
			clearInteractiveScreen()
			runChainsSetup(nil, nil)
		case "0":
			return
		default:
			fmt.Println("  请输入 0-4。")
			fmt.Println()
			waitEnter(reader)
		}
	}
}

func runExtensionWorkspaceMenu(reader *bufio.Reader) {
	status := ""

	for {
		cfg := loadConfigOrDefault()
		clearInteractiveScreen()
		printSimpleDetail("扩展模式", renderExtensionWorkspaceLines(cfg, status))

		choice := promptMenuChoice(reader, "请选择 [0-5]: ")
		status = ""

		switch choice {
		case "1":
			_, err := updateExtensionMode("chains")
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("扩展模式已切换为 chains")
			}
		case "2":
			if strings.TrimSpace(cfg.Extension.ScriptPath) == "" {
				value, ok := promptSimpleValue(reader, "输入 script_path", "", false)
				if !ok {
					continue
				}
				if _, err := updateScriptPath(value); err != nil {
					status = errorLine(err.Error())
					continue
				}
			}
			_, err := updateExtensionMode("script")
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("扩展模式已切换为 script")
			}
		case "3":
			_, err := updateExtensionMode("off")
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("扩展模式已关闭")
			}
		case "4":
			nextMode := "global"
			if cfg.Extension.ResidentialChain != nil && strings.EqualFold(cfg.Extension.ResidentialChain.Mode, "global") {
				nextMode = "rule"
			}
			_, err := updateChainMode(nextMode)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("chains 路由模式已切换为 " + nextMode)
			}
		case "5":
			value, ok := promptSimpleValue(reader, "输入 script_path", cfg.Extension.ScriptPath, false)
			if !ok {
				continue
			}
			_, err := updateScriptPath(value)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("script_path 已更新")
			}
		case "0":
			return
		default:
			status = errorLine("请输入 0-5。")
		}
	}
}

func runChainWorkspaceMenu(reader *bufio.Reader) {
	status := ""

	for {
		cfg := loadConfigOrDefault()
		clearInteractiveScreen()
		printSimpleDetail("住宅代理", renderChainWorkspaceLines(cfg, status))

		choice := promptMenuChoice(reader, "请选择 [0-6]: ")
		status = ""

		switch choice {
		case "1":
			current := ""
			if cfg.Extension.ResidentialChain != nil {
				current = cfg.Extension.ResidentialChain.ProxyServer
			}
			value, ok := promptSimpleValue(reader, "输入住宅代理服务器", current, false)
			if !ok {
				continue
			}
			_, err := updateChainServer(value)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("住宅代理服务器已更新")
			}
		case "2":
			current := ""
			if cfg.Extension.ResidentialChain != nil && cfg.Extension.ResidentialChain.ProxyPort > 0 {
				current = fmt.Sprintf("%d", cfg.Extension.ResidentialChain.ProxyPort)
			}
			value, ok := promptSimpleValue(reader, "输入住宅代理端口", current, false)
			if !ok {
				continue
			}
			_, err := updateChainPort(value)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("住宅代理端口已更新")
			}
		case "3":
			nextType := "http"
			if cfg.Extension.ResidentialChain != nil && strings.EqualFold(cfg.Extension.ResidentialChain.ProxyType, "http") {
				nextType = "socks5"
			}
			_, err := updateChainProxyType(nextType)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("住宅代理协议已切换为 " + nextType)
			}
		case "4":
			current := ""
			if cfg.Extension.ResidentialChain != nil {
				current = cfg.Extension.ResidentialChain.ProxyUsername
			}
			value, ok := promptSimpleValue(reader, "输入住宅代理用户名", current, true)
			if !ok {
				continue
			}
			_, err := updateChainUsername(value)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("住宅代理用户名已更新")
			}
		case "5":
			current := ""
			if cfg.Extension.ResidentialChain != nil {
				current = maskedSecret(cfg.Extension.ResidentialChain.ProxyPassword)
			}
			value, ok := promptSimpleValue(reader, "输入住宅代理密码", current, true)
			if !ok {
				continue
			}
			_, err := updateChainPassword(value)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("住宅代理密码已更新")
			}
		case "6":
			current := ""
			if cfg.Extension.ResidentialChain != nil {
				current = cfg.Extension.ResidentialChain.AirportGroup
			}
			value, ok := promptSimpleValue(reader, "输入机场出口组名称", current, false)
			if !ok {
				continue
			}
			_, err := updateChainAirportGroup(value)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("机场出口组已更新")
			}
		case "0":
			return
		default:
			status = errorLine("请输入 0-6。")
		}
	}
}

func runDirectProxyWorkspaceMenu(reader *bufio.Reader) {
	status := ""

	for {
		cfg := loadConfigOrDefault()
		clearInteractiveScreen()
		printSimpleDetail("直接代理", renderDirectProxyWorkspaceLines(cfg, status))

		choice := promptMenuChoice(reader, "请选择 [0 / S/O/T/U/P/N]: ")
		status = ""

		switch strings.ToLower(choice) {
		case "s":
			current := ""
			if cfg.Proxy.DirectProxy != nil {
				current = cfg.Proxy.DirectProxy.Server
			}
			value, ok := promptSimpleValue(reader, "输入代理服务器地址", current, false)
			if !ok {
				continue
			}
			_, err := updateDirectProxyServer(value)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("代理服务器已更新")
			}
		case "o":
			current := ""
			if cfg.Proxy.DirectProxy != nil && cfg.Proxy.DirectProxy.Port > 0 {
				current = fmt.Sprintf("%d", cfg.Proxy.DirectProxy.Port)
			}
			value, ok := promptSimpleValue(reader, "输入代理端口", current, false)
			if !ok {
				continue
			}
			_, err := updateDirectProxyPort(value)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("代理端口已更新")
			}
		case "t":
			nextType := "http"
			if cfg.Proxy.DirectProxy != nil && strings.EqualFold(cfg.Proxy.DirectProxy.Type, "http") {
				nextType = "socks5"
			}
			_, err := updateDirectProxyType(nextType)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("协议类型已切换为 " + nextType)
			}
		case "u":
			current := ""
			if cfg.Proxy.DirectProxy != nil {
				current = cfg.Proxy.DirectProxy.Username
			}
			value, ok := promptSimpleValue(reader, "输入用户名", current, true)
			if !ok {
				continue
			}
			_, err := updateDirectProxyUsername(value)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("用户名已更新")
			}
		case "p":
			current := ""
			if cfg.Proxy.DirectProxy != nil {
				current = maskedSecret(cfg.Proxy.DirectProxy.Password)
			}
			value, ok := promptSimpleValue(reader, "输入密码", current, true)
			if !ok {
				continue
			}
			_, err := updateDirectProxyPassword(value)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("密码已更新")
			}
		case "n":
			current := ""
			if cfg.Proxy.DirectProxy != nil {
				current = cfg.Proxy.DirectProxy.Name
			}
			value, ok := promptSimpleValue(reader, "输入节点名称", current, false)
			if !ok {
				continue
			}
			_, err := updateDirectProxyName(value)
			if err != nil {
				status = errorLine(err.Error())
			} else {
				status = successLine("节点名称已更新")
			}
		case "0":
			return
		default:
			status = errorLine("请输入 0 或 S/O/T/U/P/N。")
		}
	}
}

func showLogsScreen(reader *bufio.Reader, logFile string) {
	lines := []string{renderSectionTitle("最近日志")}
	data, err := os.ReadFile(logFile)
	if err != nil {
		lines = append(lines, errorLine("无法读取日志文件。"))
	} else {
		all := splitLines(string(data))
		start := len(all) - 30
		if start < 0 {
			start = 0
		}
		for _, line := range all[start:] {
			if strings.TrimSpace(line) == "" {
				continue
			}
			lines = append(lines, "  "+line)
		}
		lines = append(lines, "", noteLine("实时查看: "+followLogCommand(logFile)))
	}

	clearInteractiveScreen()
	printSimpleDetail("最近日志", lines)
	waitEnter(reader)
}

func showUpdateScreen(reader *bufio.Reader) {
	lines := []string{
		renderSectionTitle("升级提示"),
	}
	if notice := loadUpdateNotice(); notice != nil {
		for _, line := range renderUpdateNoticeLines(notice) {
			lines = append(lines, "  "+line)
		}
	} else {
		lines = append(lines, "  当前已经是最新版本，或本次未检测到更新。")
	}

	clearInteractiveScreen()
	printSimpleDetail("升级提示", lines)
	waitEnter(reader)
}
