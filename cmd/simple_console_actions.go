package cmd

import (
	"fmt"
	"strings"

	"github.com/tght/lan-proxy-gateway/internal/ui"
)

func printSimpleDetail(title string, lines []string) {
	ui.Separator()
	fmt.Printf("  %s\n", title)
	ui.Separator()
	fmt.Println()
	for _, line := range lines {
		plain := strings.TrimSpace(plainText(line))
		if plain == "" {
			fmt.Println()
			continue
		}
		fmt.Printf("  %s\n", plain)
	}
	fmt.Println()
}

func handleSimpleConfigCommand(raw string) (consoleAction, bool) {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return consoleActionNone, false
	}

	cmd := strings.ToLower(fields[0])
	args := fields[1:]

	switch cmd {
	case "proxy":
		if len(args) == 0 {
			printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(loadConfigOrDefault(), ""))
			return consoleActionNone, true
		}
		switch strings.ToLower(args[0]) {
		case "source":
			if len(args) < 2 {
				printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(loadConfigOrDefault(), errorLine("用法: proxy source url|file")))
				return consoleActionNone, true
			}
			source, err := normalizeProxySource(args[1])
			if err != nil {
				printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			cfg, err := updateProxySource(source)
			if err != nil {
				printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(cfg, successLine("代理来源已切换为 "+source)))
			return consoleActionNone, true
		case "url":
			if len(args) < 2 {
				printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(loadConfigOrDefault(), errorLine("用法: proxy url <订阅链接>")))
				return consoleActionNone, true
			}
			cfg, err := updateSubscriptionURL(strings.Join(args[1:], " "))
			if err != nil {
				printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(cfg, successLine("订阅链接已更新")))
			return consoleActionNone, true
		case "file":
			if len(args) < 2 {
				printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(loadConfigOrDefault(), errorLine("用法: proxy file <本地配置文件路径>")))
				return consoleActionNone, true
			}
			cfg, err := updateProxyConfigFile(strings.Join(args[1:], " "))
			if err != nil {
				printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(cfg, successLine("本地配置文件路径已更新")))
			return consoleActionNone, true
		case "name":
			if len(args) < 2 {
				printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(loadConfigOrDefault(), errorLine("用法: proxy name <订阅名称>")))
				return consoleActionNone, true
			}
			cfg, err := updateSubscriptionName(strings.Join(args[1:], " "))
			if err != nil {
				printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(cfg, successLine("订阅名称已更新")))
			return consoleActionNone, true
		}
	case "source":
		if len(args) < 1 {
			printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(loadConfigOrDefault(), ""))
			return consoleActionNone, true
		}
		source, err := normalizeProxySource(args[0])
		if err != nil {
			printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
			return consoleActionNone, true
		}
		cfg, err := updateProxySource(source)
		if err != nil {
			printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
			return consoleActionNone, true
		}
		printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(cfg, successLine("代理来源已切换为 "+source)))
		return consoleActionNone, true
	case "tun":
		if len(args) == 0 {
			printSimpleDetail("运行模式工作台", renderRuntimeWorkspaceLines(loadConfigOrDefault(), ""))
			return consoleActionNone, true
		}
		enabled, err := normalizeOnOffToggle(args[0], loadConfigOrDefault().Runtime.Tun.Enabled)
		if err != nil {
			printSimpleDetail("运行模式工作台", renderRuntimeWorkspaceLines(loadConfigOrDefault(), errorLine("用法: tun on|off|toggle")))
			return consoleActionNone, true
		}
		cfg, err := updateTunEnabled(enabled)
		if err != nil {
			printSimpleDetail("运行模式工作台", renderRuntimeWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
			return consoleActionNone, true
		}
		printSimpleDetail("运行模式工作台", renderRuntimeWorkspaceLines(cfg, successLine("TUN 已切换为 "+onOff(enabled))))
		return consoleActionNone, true
	case "bypass":
		if len(args) == 0 {
			printSimpleDetail("运行模式工作台", renderRuntimeWorkspaceLines(loadConfigOrDefault(), ""))
			return consoleActionNone, true
		}
		enabled, err := normalizeOnOffToggle(args[0], loadConfigOrDefault().Runtime.Tun.BypassLocal)
		if err != nil {
			printSimpleDetail("运行模式工作台", renderRuntimeWorkspaceLines(loadConfigOrDefault(), errorLine("用法: bypass on|off|toggle")))
			return consoleActionNone, true
		}
		cfg, err := updateBypassLocal(enabled)
		if err != nil {
			printSimpleDetail("运行模式工作台", renderRuntimeWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
			return consoleActionNone, true
		}
		printSimpleDetail("运行模式工作台", renderRuntimeWorkspaceLines(cfg, successLine("本机绕过代理已切换为 "+onOff(enabled))))
		return consoleActionNone, true
	case "rules":
		printSimpleDetail("规则工作台", renderRulesWorkspaceLines(loadConfigOrDefault(), ""))
		return consoleActionNone, true
	case "rule":
		if len(args) == 0 {
			printSimpleDetail("规则工作台", renderRulesWorkspaceLines(loadConfigOrDefault(), errorLine("用法: rule <lan|china|apple|nintendo|global|ads> [on|off|toggle]")))
			return consoleActionNone, true
		}
		cfg := loadConfigOrDefault()
		ruleName := strings.ToLower(args[0])
		current := map[string]bool{
			"lan":      cfg.Rules.LanDirectEnabled(),
			"china":    cfg.Rules.ChinaDirectEnabled(),
			"apple":    cfg.Rules.AppleRulesEnabled(),
			"nintendo": cfg.Rules.NintendoProxyEnabled(),
			"global":   cfg.Rules.GlobalProxyEnabled(),
			"ads":      cfg.Rules.AdsRejectEnabled(),
		}[ruleName]
		enabled, err := normalizeOnOffToggle("", current)
		if len(args) > 1 {
			enabled, err = normalizeOnOffToggle(args[1], current)
		}
		if err != nil {
			printSimpleDetail("规则工作台", renderRulesWorkspaceLines(loadConfigOrDefault(), errorLine("用法: rule <lan|china|apple|nintendo|global|ads> [on|off|toggle]")))
			return consoleActionNone, true
		}
		nextCfg, err := updateRuleToggle(ruleName, enabled)
		if err != nil {
			printSimpleDetail("规则工作台", renderRulesWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
			return consoleActionNone, true
		}
		printSimpleDetail("规则工作台", renderRulesWorkspaceLines(nextCfg, successLine(ruleName+" 已切换为 "+onOff(enabled))))
		return consoleActionNone, true
	case "extension", "mode":
		if len(args) == 0 {
			printSimpleDetail("扩展模式工作台", renderExtensionWorkspaceLines(loadConfigOrDefault(), ""))
			return consoleActionNone, true
		}
		mode, err := normalizeExtensionMode(args[0])
		if err != nil {
			printSimpleDetail("扩展模式工作台", renderExtensionWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
			return consoleActionNone, true
		}
		cfg, err := updateExtensionMode(mode)
		if err != nil {
			printSimpleDetail("扩展模式工作台", renderExtensionWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
			return consoleActionNone, true
		}
		modeName := "off"
		if mode != "" {
			modeName = mode
		}
		printSimpleDetail("扩展模式工作台", renderExtensionWorkspaceLines(cfg, successLine("扩展模式已切换为 "+modeName)))
		return consoleActionNone, true
	case "script":
		if len(args) == 0 {
			printSimpleDetail("扩展模式工作台", renderExtensionWorkspaceLines(loadConfigOrDefault(), errorLine("用法: script <脚本路径>")))
			return consoleActionNone, true
		}
		cfg, err := updateScriptPath(strings.Join(args, " "))
		if err != nil {
			printSimpleDetail("扩展模式工作台", renderExtensionWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
			return consoleActionNone, true
		}
		printSimpleDetail("扩展模式工作台", renderExtensionWorkspaceLines(cfg, successLine("script_path 已更新")))
		return consoleActionNone, true
	case "chain", "residential":
		if len(args) == 0 {
			printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(loadConfigOrDefault(), ""))
			return consoleActionNone, true
		}
		switch strings.ToLower(args[0]) {
		case "mode":
			if len(args) < 2 {
				printSimpleDetail("扩展模式工作台", renderExtensionWorkspaceLines(loadConfigOrDefault(), errorLine("用法: chain mode rule|global")))
				return consoleActionNone, true
			}
			cfg, err := updateChainMode(args[1])
			if err != nil {
				printSimpleDetail("扩展模式工作台", renderExtensionWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("扩展模式工作台", renderExtensionWorkspaceLines(cfg, successLine("chains 路由模式已更新")))
			return consoleActionNone, true
		case "server":
			if len(args) < 2 {
				printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(loadConfigOrDefault(), errorLine("用法: chain server <住宅代理服务器>")))
				return consoleActionNone, true
			}
			cfg, err := updateChainServer(strings.Join(args[1:], " "))
			if err != nil {
				printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(cfg, successLine("住宅代理服务器已更新")))
			return consoleActionNone, true
		case "port":
			if len(args) < 2 {
				printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(loadConfigOrDefault(), errorLine("用法: chain port <端口>")))
				return consoleActionNone, true
			}
			cfg, err := updateChainPort(args[1])
			if err != nil {
				printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(cfg, successLine("住宅代理端口已更新")))
			return consoleActionNone, true
		case "type":
			if len(args) < 2 {
				printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(loadConfigOrDefault(), errorLine("用法: chain type socks5|http")))
				return consoleActionNone, true
			}
			cfg, err := updateChainProxyType(args[1])
			if err != nil {
				printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(cfg, successLine("住宅代理协议已更新")))
			return consoleActionNone, true
		case "airport":
			if len(args) < 2 {
				printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(loadConfigOrDefault(), errorLine("用法: chain airport <机场组名称>")))
				return consoleActionNone, true
			}
			cfg, err := updateChainAirportGroup(strings.Join(args[1:], " "))
			if err != nil {
				printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(cfg, successLine("机场出口组已更新")))
			return consoleActionNone, true
		case "user":
			cfg, err := updateChainUsername(strings.Join(args[1:], " "))
			if err != nil {
				printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(cfg, successLine("住宅代理用户名已更新")))
			return consoleActionNone, true
		case "password", "pass":
			cfg, err := updateChainPassword(strings.Join(args[1:], " "))
			if err != nil {
				printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(cfg, successLine("住宅代理密码已更新")))
			return consoleActionNone, true
		}
	case "config", "config open":
		return consoleActionOpenConfig, true
	case "chains", "chains setup":
		if strings.EqualFold(strings.TrimSpace(raw), "chains setup") {
			return consoleActionOpenChainsSetup, true
		}
	}

	return consoleActionNone, false
}
