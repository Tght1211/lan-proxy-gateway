package cmd

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

type simpleWorkspace string

const (
	simpleWorkspaceNone         simpleWorkspace = ""
	simpleWorkspaceSubscription simpleWorkspace = "subscription"
	simpleWorkspaceProxy        simpleWorkspace = "proxy"
	simpleWorkspaceDirect       simpleWorkspace = "direct"
	simpleWorkspaceRuntime      simpleWorkspace = "runtime"
	simpleWorkspaceRules        simpleWorkspace = "rules"
	simpleWorkspaceExtension    simpleWorkspace = "extension"
	simpleWorkspaceChain        simpleWorkspace = "chain"
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

func promptSimpleValue(reader *bufio.Reader, label, current string, allowClear bool) (string, bool) {
	prompt := label
	if strings.TrimSpace(current) != "" {
		prompt += "（当前: " + current + "）"
	}
	if allowClear {
		prompt += "，输入 - 清空，回车取消"
	} else {
		prompt += "，回车取消"
	}
	fmt.Print("  " + prompt + ": ")

	input, _ := reader.ReadString('\n')
	value := strings.TrimSpace(input)
	if value == "" {
		fmt.Println("  已取消。")
		fmt.Println()
		return "", false
	}
	if allowClear && value == "-" {
		return "", true
	}
	return value, true
}

func expandSimpleWorkspaceShortcut(workspace simpleWorkspace, raw string, cfg *config.Config) string {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return raw
	}

	key := strings.ToLower(fields[0])
	var expanded string

	switch workspace {
	case simpleWorkspaceSubscription:
		switch key {
		case "1":
			expanded = "subscription add url"
		case "2":
			expanded = "subscription add file"
		case "3":
			expanded = "subscription use"
		case "u":
			expanded = "proxy url"
		case "f":
			expanded = "proxy file"
		case "n":
			expanded = "proxy name"
		}
	case simpleWorkspaceProxy:
		switch key {
		case "1":
			expanded = "proxy source url"
		case "2":
			expanded = "proxy source file"
		case "3":
			expanded = "proxy source proxy"
		case "u":
			expanded = "proxy url"
		case "f":
			expanded = "proxy file"
		case "n":
			expanded = "proxy name"
		case "d":
			expanded = "direct"
		}
	case simpleWorkspaceDirect:
		switch key {
		case "s":
			expanded = "direct server"
		case "o":
			expanded = "direct port"
		case "t":
			nextType := "http"
			if cfg != nil && cfg.Proxy.DirectProxy != nil && strings.EqualFold(cfg.Proxy.DirectProxy.Type, "http") {
				nextType = "socks5"
			}
			expanded = "direct type " + nextType
		case "u":
			expanded = "direct user"
		case "p":
			expanded = "direct password"
		case "n":
			expanded = "direct name"
		}
	case simpleWorkspaceRuntime:
		switch key {
		case "1":
			expanded = "tun toggle"
		case "2":
			expanded = "bypass toggle"
		}
	case simpleWorkspaceRules:
		switch key {
		case "1":
			expanded = "rule lan toggle"
		case "2":
			expanded = "rule china toggle"
		case "3":
			expanded = "rule apple toggle"
		case "4":
			expanded = "rule nintendo toggle"
		case "5":
			expanded = "rule global toggle"
		case "6":
			expanded = "rule ads toggle"
		}
	case simpleWorkspaceExtension:
		switch key {
		case "1":
			expanded = "extension chains"
		case "2":
			expanded = "extension script"
		case "0":
			expanded = "extension off"
		case "r":
			mode := "global"
			if cfg != nil && cfg.Extension.ResidentialChain != nil && strings.EqualFold(cfg.Extension.ResidentialChain.Mode, "global") {
				mode = "rule"
			}
			expanded = "chain mode " + mode
		case "p":
			expanded = "script"
		}
	case simpleWorkspaceChain:
		switch key {
		case "s":
			expanded = "chain server"
		case "o":
			expanded = "chain port"
		case "t":
			nextType := "http"
			if cfg != nil && cfg.Extension.ResidentialChain != nil && strings.EqualFold(cfg.Extension.ResidentialChain.ProxyType, "http") {
				nextType = "socks5"
			}
			expanded = "chain type " + nextType
		case "u":
			expanded = "chain user"
		case "p":
			expanded = "chain password"
		case "a":
			expanded = "chain airport"
		}
	}

	if expanded == "" {
		return raw
	}
	if len(fields) > 1 {
		return expanded + " " + strings.Join(fields[1:], " ")
	}
	return expanded
}

func handleSimpleConfigCommand(reader *bufio.Reader, workspace *simpleWorkspace, raw string) (consoleAction, bool) {
	cfg := loadConfigOrDefault()
	if workspace != nil {
		raw = expandSimpleWorkspaceShortcut(*workspace, raw, cfg)
	}

	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return consoleActionNone, false
	}

	cmd := strings.ToLower(fields[0])
	args := fields[1:]

	switch cmd {
	case "subscription", "subscriptions", "profile", "profiles", "sub":
		if workspace != nil {
			*workspace = simpleWorkspaceSubscription
		}
		if len(args) == 0 || strings.EqualFold(args[0], "list") {
			printSimpleDetail("订阅管理工作台", renderSubscriptionWorkspaceLines(cfg, ""))
			return consoleActionNone, true
		}
		switch strings.ToLower(args[0]) {
		case "use":
			if len(args) < 2 {
				name, ok := promptSimpleValue(reader, "输入要切换的订阅名称", cfg.Proxy.CurrentProfile, false)
				if !ok {
					return consoleActionNone, true
				}
				args = append(args, name)
			}
			cfg, err := switchSubscriptionProfile(strings.Join(args[1:], " "))
			if err != nil {
				printSimpleDetail("订阅管理工作台", renderSubscriptionWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("订阅管理工作台", renderSubscriptionWorkspaceLines(cfg, successLine("已切换当前订阅")))
			return consoleActionNone, true
		case "add":
			if len(args) < 2 {
				printSimpleDetail("订阅管理工作台", renderSubscriptionWorkspaceLines(cfg, errorLine("用法: subscription add url|file <名称> <链接或路径>")))
				return consoleActionNone, true
			}
			source := args[1]
			name := ""
			if len(args) >= 3 {
				name = args[2]
			} else {
				prompt, ok := promptSimpleValue(reader, "输入新订阅名称", "", false)
				if !ok {
					return consoleActionNone, true
				}
				name = prompt
			}
			value := ""
			if len(args) >= 4 {
				value = strings.Join(args[3:], " ")
			} else {
				label := "输入订阅链接"
				if strings.EqualFold(source, "file") {
					label = "输入本地配置文件路径"
				}
				prompt, ok := promptSimpleValue(reader, label, "", false)
				if !ok {
					return consoleActionNone, true
				}
				value = prompt
			}
			cfg, err := createSubscriptionProfile(name, source, value)
			if err != nil {
				printSimpleDetail("订阅管理工作台", renderSubscriptionWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("订阅管理工作台", renderSubscriptionWorkspaceLines(cfg, successLine("已新建并切换到订阅: "+name)))
			return consoleActionNone, true
		}
	case "proxy":
		if workspace != nil {
			*workspace = simpleWorkspaceProxy
		}
		if len(args) == 0 {
			printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(cfg, ""))
			return consoleActionNone, true
		}
		switch strings.ToLower(args[0]) {
		case "source":
			if len(args) < 2 {
				printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(loadConfigOrDefault(), errorLine("用法: proxy source url|file|proxy")))
				return consoleActionNone, true
			}
			source, err := normalizeProxySource(args[1])
			if err != nil {
				printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			nextCfg, err := updateProxySource(source)
			if err != nil {
				printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(nextCfg, successLine("代理来源已切换为 "+source)))
			return consoleActionNone, true
		case "url":
			if len(args) < 2 {
				prompt, ok := promptSimpleValue(reader, "输入订阅链接", activeProxyProfile(cfg).SubscriptionURL, false)
				if !ok {
					return consoleActionNone, true
				}
				args = append(args, prompt)
			}
			nextCfg, err := updateSubscriptionURL(strings.Join(args[1:], " "))
			if err != nil {
				printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(nextCfg, successLine("订阅链接已更新")))
			return consoleActionNone, true
		case "file":
			if len(args) < 2 {
				prompt, ok := promptSimpleValue(reader, "输入本地配置文件路径", activeProxyProfile(cfg).ConfigFile, false)
				if !ok {
					return consoleActionNone, true
				}
				args = append(args, prompt)
			}
			nextCfg, err := updateProxyConfigFile(strings.Join(args[1:], " "))
			if err != nil {
				printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(nextCfg, successLine("本地配置文件路径已更新")))
			return consoleActionNone, true
		case "name":
			if len(args) < 2 {
				prompt, ok := promptSimpleValue(reader, "输入订阅名称", activeProxyProfile(cfg).Name, false)
				if !ok {
					return consoleActionNone, true
				}
				args = append(args, prompt)
			}
			nextCfg, err := updateSubscriptionName(strings.Join(args[1:], " "))
			if err != nil {
				printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("代理来源工作台", renderProxyWorkspaceLines(nextCfg, successLine("订阅名称已更新")))
			return consoleActionNone, true
		}
	case "direct":
		if workspace != nil {
			*workspace = simpleWorkspaceDirect
		}
		if len(args) == 0 {
			printSimpleDetail("直接代理工作台", renderDirectProxyWorkspaceLines(cfg, ""))
			return consoleActionNone, true
		}
		switch strings.ToLower(args[0]) {
		case "server":
			if len(args) < 2 {
				current := ""
				if cfg.Proxy.DirectProxy != nil {
					current = cfg.Proxy.DirectProxy.Server
				}
				prompt, ok := promptSimpleValue(reader, "输入代理服务器地址", current, false)
				if !ok {
					return consoleActionNone, true
				}
				args = append(args, prompt)
			}
			nextCfg, err := updateDirectProxyServer(strings.Join(args[1:], " "))
			if err != nil {
				printSimpleDetail("直接代理工作台", renderDirectProxyWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("直接代理工作台", renderDirectProxyWorkspaceLines(nextCfg, successLine("代理服务器已更新")))
			return consoleActionNone, true
		case "port":
			if len(args) < 2 {
				current := ""
				if cfg.Proxy.DirectProxy != nil && cfg.Proxy.DirectProxy.Port > 0 {
					current = fmt.Sprintf("%d", cfg.Proxy.DirectProxy.Port)
				}
				prompt, ok := promptSimpleValue(reader, "输入代理端口", current, false)
				if !ok {
					return consoleActionNone, true
				}
				args = append(args, prompt)
			}
			nextCfg, err := updateDirectProxyPort(args[1])
			if err != nil {
				printSimpleDetail("直接代理工作台", renderDirectProxyWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("直接代理工作台", renderDirectProxyWorkspaceLines(nextCfg, successLine("代理端口已更新")))
			return consoleActionNone, true
		case "type":
			if len(args) < 2 {
				printSimpleDetail("直接代理工作台", renderDirectProxyWorkspaceLines(loadConfigOrDefault(), errorLine("用法: direct type socks5|http")))
				return consoleActionNone, true
			}
			nextCfg, err := updateDirectProxyType(args[1])
			if err != nil {
				printSimpleDetail("直接代理工作台", renderDirectProxyWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("直接代理工作台", renderDirectProxyWorkspaceLines(nextCfg, successLine("协议类型已更新")))
			return consoleActionNone, true
		case "user":
			if len(args) < 2 {
				current := ""
				if cfg.Proxy.DirectProxy != nil {
					current = cfg.Proxy.DirectProxy.Username
				}
				prompt, ok := promptSimpleValue(reader, "输入用户名（清空输入 -）", current, true)
				if !ok {
					return consoleActionNone, true
				}
				args = append(args, prompt)
			}
			nextCfg, err := updateDirectProxyUsername(strings.Join(args[1:], " "))
			if err != nil {
				printSimpleDetail("直接代理工作台", renderDirectProxyWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("直接代理工作台", renderDirectProxyWorkspaceLines(nextCfg, successLine("用户名已更新")))
			return consoleActionNone, true
		case "password", "pass":
			if len(args) < 2 {
				prompt, ok := promptSimpleValue(reader, "输入密码（清空输入 -）", "", true)
				if !ok {
					return consoleActionNone, true
				}
				args = append(args, prompt)
			}
			nextCfg, err := updateDirectProxyPassword(strings.Join(args[1:], " "))
			if err != nil {
				printSimpleDetail("直接代理工作台", renderDirectProxyWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("直接代理工作台", renderDirectProxyWorkspaceLines(nextCfg, successLine("密码已更新")))
			return consoleActionNone, true
		case "name":
			if len(args) < 2 {
				current := "MyProxy"
				if cfg.Proxy.DirectProxy != nil {
					current = cfg.Proxy.DirectProxy.Name
				}
				prompt, ok := promptSimpleValue(reader, "输入节点名称", current, false)
				if !ok {
					return consoleActionNone, true
				}
				args = append(args, prompt)
			}
			nextCfg, err := updateDirectProxyName(strings.Join(args[1:], " "))
			if err != nil {
				printSimpleDetail("直接代理工作台", renderDirectProxyWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("直接代理工作台", renderDirectProxyWorkspaceLines(nextCfg, successLine("节点名称已更新")))
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
		if workspace != nil {
			*workspace = simpleWorkspaceRuntime
		}
		if len(args) == 0 {
			printSimpleDetail("运行模式工作台", renderRuntimeWorkspaceLines(cfg, ""))
			return consoleActionNone, true
		}
		enabled, err := normalizeOnOffToggle(args[0], cfg.Runtime.Tun.Enabled)
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
		if workspace != nil {
			*workspace = simpleWorkspaceRuntime
		}
		if len(args) == 0 {
			printSimpleDetail("运行模式工作台", renderRuntimeWorkspaceLines(cfg, ""))
			return consoleActionNone, true
		}
		enabled, err := normalizeOnOffToggle(args[0], cfg.Runtime.Tun.BypassLocal)
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
		if workspace != nil {
			*workspace = simpleWorkspaceRules
		}
		printSimpleDetail("规则工作台", renderRulesWorkspaceLines(cfg, ""))
		return consoleActionNone, true
	case "rule":
		if workspace != nil {
			*workspace = simpleWorkspaceRules
		}
		if len(args) == 0 {
			printSimpleDetail("规则工作台", renderRulesWorkspaceLines(loadConfigOrDefault(), errorLine("用法: rule <lan|china|apple|nintendo|global|ads> [on|off|toggle]")))
			return consoleActionNone, true
		}
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
		if workspace != nil {
			*workspace = simpleWorkspaceExtension
		}
		if len(args) == 0 {
			printSimpleDetail("扩展模式工作台", renderExtensionWorkspaceLines(cfg, ""))
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
		if workspace != nil {
			*workspace = simpleWorkspaceExtension
		}
		if len(args) == 0 {
			prompt, ok := promptSimpleValue(reader, "输入 script_path", cfg.Extension.ScriptPath, false)
			if !ok {
				return consoleActionNone, true
			}
			args = append(args, prompt)
		}
		cfg, err := updateScriptPath(strings.Join(args, " "))
		if err != nil {
			printSimpleDetail("扩展模式工作台", renderExtensionWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
			return consoleActionNone, true
		}
		printSimpleDetail("扩展模式工作台", renderExtensionWorkspaceLines(cfg, successLine("script_path 已更新")))
		return consoleActionNone, true
	case "chain", "residential":
		if workspace != nil {
			*workspace = simpleWorkspaceChain
		}
		if len(args) == 0 {
			printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(cfg, ""))
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
				current := ""
				if cfg.Extension.ResidentialChain != nil {
					current = cfg.Extension.ResidentialChain.ProxyServer
				}
				prompt, ok := promptSimpleValue(reader, "输入住宅代理服务器", current, false)
				if !ok {
					return consoleActionNone, true
				}
				args = append(args, prompt)
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
				current := ""
				if cfg.Extension.ResidentialChain != nil && cfg.Extension.ResidentialChain.ProxyPort > 0 {
					current = fmt.Sprintf("%d", cfg.Extension.ResidentialChain.ProxyPort)
				}
				prompt, ok := promptSimpleValue(reader, "输入住宅代理端口", current, false)
				if !ok {
					return consoleActionNone, true
				}
				args = append(args, prompt)
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
				current := ""
				if cfg.Extension.ResidentialChain != nil {
					current = cfg.Extension.ResidentialChain.AirportGroup
				}
				prompt, ok := promptSimpleValue(reader, "输入机场出口组名称", current, false)
				if !ok {
					return consoleActionNone, true
				}
				args = append(args, prompt)
			}
			cfg, err := updateChainAirportGroup(strings.Join(args[1:], " "))
			if err != nil {
				printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(cfg, successLine("机场出口组已更新")))
			return consoleActionNone, true
		case "user":
			if len(args) < 2 {
				current := ""
				if cfg.Extension.ResidentialChain != nil {
					current = cfg.Extension.ResidentialChain.ProxyUsername
				}
				prompt, ok := promptSimpleValue(reader, "输入住宅代理用户名", current, true)
				if !ok {
					return consoleActionNone, true
				}
				args = append(args, prompt)
			}
			cfg, err := updateChainUsername(strings.Join(args[1:], " "))
			if err != nil {
				printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(loadConfigOrDefault(), errorLine(err.Error())))
				return consoleActionNone, true
			}
			printSimpleDetail("住宅代理工作台", renderChainWorkspaceLines(cfg, successLine("住宅代理用户名已更新")))
			return consoleActionNone, true
		case "password", "pass":
			if len(args) < 2 {
				prompt, ok := promptSimpleValue(reader, "输入住宅代理密码", maskedSecret(func() string {
					if cfg.Extension.ResidentialChain == nil {
						return ""
					}
					return cfg.Extension.ResidentialChain.ProxyPassword
				}()), true)
				if !ok {
					return consoleActionNone, true
				}
				args = append(args, prompt)
			}
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
