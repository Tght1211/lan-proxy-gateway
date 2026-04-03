package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func saveConsoleConfig(cfg *config.Config) error {
	cfgPath := resolveConfigPath()
	if cfgPath == ".secret" {
		cfgPath = "gateway.yaml"
	}
	return config.Save(cfg, cfgPath)
}

func updateConsoleConfig(mut func(cfg *config.Config) error) (*config.Config, error) {
	cfg := loadConfigOrDefault()
	if err := mut(cfg); err != nil {
		return nil, err
	}
	if err := saveConsoleConfig(cfg); err != nil {
		return nil, err
	}
	return loadConfigOrDefault(), nil
}

func ensureConsoleChain(cfg *config.Config) *config.ResidentialChain {
	if cfg.Extension.ResidentialChain == nil {
		cfg.Extension.ResidentialChain = &config.ResidentialChain{}
	}
	if cfg.Extension.ResidentialChain.Mode == "" {
		cfg.Extension.ResidentialChain.Mode = "rule"
	}
	if cfg.Extension.ResidentialChain.ProxyType == "" {
		cfg.Extension.ResidentialChain.ProxyType = "socks5"
	}
	if cfg.Extension.ResidentialChain.AirportGroup == "" {
		cfg.Extension.ResidentialChain.AirportGroup = "Auto"
	}
	return cfg.Extension.ResidentialChain
}

func normalizeOnOffToggle(value string, current bool) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "toggle", "":
		return !current, nil
	case "on", "true", "enable", "enabled", "1":
		return true, nil
	case "off", "false", "disable", "disabled", "0":
		return false, nil
	default:
		return current, fmt.Errorf("请输入 on、off 或 toggle")
	}
}

func normalizeProxySource(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "url":
		return "url", nil
	case "file":
		return "file", nil
	default:
		return "", fmt.Errorf("代理来源仅支持 url 或 file")
	}
}

func normalizeExtensionMode(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "off":
		return "", nil
	case "chains":
		return "chains", nil
	case "script":
		return "script", nil
	default:
		return "", fmt.Errorf("扩展模式仅支持 chains、script 或 off")
	}
}

func normalizeChainMode(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "rule":
		return "rule", nil
	case "global":
		return "global", nil
	default:
		return "", fmt.Errorf("链式模式仅支持 rule 或 global")
	}
}

func normalizeProxyType(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "socks5":
		return "socks5", nil
	case "http":
		return "http", nil
	default:
		return "", fmt.Errorf("住宅代理协议仅支持 socks5 或 http")
	}
}

func validateExistingFile(path string) (string, error) {
	path = expandPath(strings.TrimSpace(path))
	if path == "" {
		return "", fmt.Errorf("路径不能为空")
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("文件不存在: %s", path)
	}
	return path, nil
}

func updateTunEnabled(enabled bool) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		cfg.Runtime.Tun.Enabled = enabled
		if !enabled {
			cfg.Runtime.Tun.BypassLocal = false
		}
		return nil
	})
}

func updateBypassLocal(enabled bool) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		cfg.Runtime.Tun.BypassLocal = enabled
		return nil
	})
}

func updateRuleToggle(rule string, enabled bool) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		switch rule {
		case "lan":
			cfg.Rules.LanDirect = boolPtr(enabled)
		case "china":
			cfg.Rules.ChinaDirect = boolPtr(enabled)
		case "apple":
			cfg.Rules.AppleRules = boolPtr(enabled)
		case "nintendo":
			cfg.Rules.NintendoProxy = boolPtr(enabled)
		case "global":
			cfg.Rules.GlobalProxy = boolPtr(enabled)
		case "ads":
			cfg.Rules.AdsReject = boolPtr(enabled)
		default:
			return fmt.Errorf("未知规则开关: %s", rule)
		}
		return nil
	})
}

func updateProxySource(source string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		switch source {
		case "url":
			if strings.TrimSpace(cfg.Proxy.SubscriptionURL) == "" {
				return fmt.Errorf("当前还没有订阅链接，先填写订阅链接再切到 url")
			}
		case "file":
			if strings.TrimSpace(cfg.Proxy.ConfigFile) == "" {
				return fmt.Errorf("当前还没有本地配置文件路径，先填写路径再切到 file")
			}
		default:
			return fmt.Errorf("代理来源仅支持 url 或 file")
		}
		cfg.Proxy.Source = source
		return nil
	})
}

func updateSubscriptionURL(url string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		url = strings.TrimSpace(url)
		if url == "" {
			return fmt.Errorf("订阅链接不能为空")
		}
		cfg.Proxy.SubscriptionURL = url
		return nil
	})
}

func updateProxyConfigFile(path string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		validated, err := validateExistingFile(path)
		if err != nil {
			return err
		}
		cfg.Proxy.ConfigFile = validated
		return nil
	})
}

func updateSubscriptionName(name string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("订阅名称不能为空")
		}
		cfg.Proxy.SubscriptionName = name
		return nil
	})
}

func updateExtensionMode(mode string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		mode, err := normalizeExtensionMode(mode)
		if err != nil {
			return err
		}
		if mode == "chains" {
			ensureConsoleChain(cfg)
		}
		cfg.Extension.Mode = mode
		return nil
	})
}

func updateScriptPath(path string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		validated, err := validateExistingFile(path)
		if err != nil {
			return err
		}
		cfg.Extension.ScriptPath = validated
		return nil
	})
}

func updateChainMode(mode string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		chain := ensureConsoleChain(cfg)
		normalized, err := normalizeChainMode(mode)
		if err != nil {
			return err
		}
		chain.Mode = normalized
		if cfg.Extension.Mode == "" {
			cfg.Extension.Mode = "chains"
		}
		return nil
	})
}

func updateChainServer(server string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		server = strings.TrimSpace(server)
		if server == "" {
			return fmt.Errorf("住宅代理服务器不能为空")
		}
		chain := ensureConsoleChain(cfg)
		chain.ProxyServer = server
		return nil
	})
}

func updateChainPort(value string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		value = strings.TrimSpace(value)
		port, err := strconv.Atoi(value)
		if err != nil || port <= 0 {
			return fmt.Errorf("端口必须是正整数")
		}
		chain := ensureConsoleChain(cfg)
		chain.ProxyPort = port
		return nil
	})
}

func updateChainProxyType(value string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		normalized, err := normalizeProxyType(value)
		if err != nil {
			return err
		}
		chain := ensureConsoleChain(cfg)
		chain.ProxyType = normalized
		return nil
	})
}

func updateChainAirportGroup(group string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		group = strings.TrimSpace(group)
		if group == "" {
			return fmt.Errorf("机场组名称不能为空")
		}
		chain := ensureConsoleChain(cfg)
		chain.AirportGroup = group
		return nil
	})
}

func updateChainUsername(name string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		chain := ensureConsoleChain(cfg)
		chain.ProxyUsername = strings.TrimSpace(name)
		return nil
	})
}

func updateChainPassword(password string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		chain := ensureConsoleChain(cfg)
		chain.ProxyPassword = strings.TrimSpace(password)
		return nil
	})
}

func renderProxyWorkspaceLines(cfg *config.Config, status string) []string {
	lines := []string{
		renderSectionTitle("当前代理来源"),
		"  来源: " + cfg.Proxy.Source,
		"  订阅名称: " + fallbackText(cfg.Proxy.SubscriptionName, "subscription"),
	}
	if cfg.Proxy.Source == "url" {
		lines = append(lines, "  订阅链接: "+shortText(cfg.Proxy.SubscriptionURL, 72))
	} else {
		lines = append(lines, "  本地配置: "+fallbackText(cfg.Proxy.ConfigFile, "未设置"))
	}
	if status != "" {
		lines = append(lines, "", status)
	}
	lines = append(lines,
		"",
		renderSectionTitle("工作台操作"),
		"  1 切到订阅链接模式",
		"  2 切到本地文件模式",
		"  U 编辑订阅链接",
		"  F 编辑本地配置文件路径",
		"  N 编辑订阅名称",
		"",
		noteLine("修改会写入 gateway.yaml，重启网关后生效。"),
	)
	return lines
}

func renderRuntimeWorkspaceLines(cfg *config.Config, status string) []string {
	lines := []string{
		renderSectionTitle("当前运行模式"),
		"  TUN: " + tuiOnOff(cfg.Runtime.Tun.Enabled),
		"  本机绕过代理: " + tuiOnOff(cfg.Runtime.Tun.BypassLocal),
		fmt.Sprintf("  端口: mixed %d | redir %d | api %d | dns %d", cfg.Runtime.Ports.Mixed, cfg.Runtime.Ports.Redir, cfg.Runtime.Ports.API, cfg.Runtime.Ports.DNS),
	}
	if status != "" {
		lines = append(lines, "", status)
	}
	lines = append(lines,
		"",
		renderSectionTitle("工作台操作"),
		"  1 切换 TUN 开关",
		"  2 切换本机绕过代理",
		"",
		noteLine("TUN 是局域网共享的核心开关；改完通常需要 sudo gateway restart。"),
	)
	return lines
}

func renderRulesWorkspaceLines(cfg *config.Config, status string) []string {
	lines := []string{
		renderSectionTitle("当前规则开关"),
		"  1 局域网直连: " + tuiOnOff(cfg.Rules.LanDirectEnabled()),
		"  2 国内直连: " + tuiOnOff(cfg.Rules.ChinaDirectEnabled()),
		"  3 Apple 规则: " + tuiOnOff(cfg.Rules.AppleRulesEnabled()),
		"  4 Nintendo 代理: " + tuiOnOff(cfg.Rules.NintendoProxyEnabled()),
		"  5 国外代理: " + tuiOnOff(cfg.Rules.GlobalProxyEnabled()),
		"  6 广告拦截: " + tuiOnOff(cfg.Rules.AdsRejectEnabled()),
	}
	if status != "" {
		lines = append(lines, "", status)
	}
	lines = append(lines,
		"",
		renderSectionTitle("工作台操作"),
		"  按 1-6 直接切换对应规则开关",
		"",
		noteLine("这组规则更偏向推荐默认值，适合先用再细调。"),
	)
	return lines
}

func renderExtensionWorkspaceLines(cfg *config.Config, status string) []string {
	lines := []string{
		renderSectionTitle("当前扩展模式"),
		"  模式: " + extensionModeName(cfg.Extension.Mode),
	}
	if cfg.Extension.Mode == "script" {
		lines = append(lines, "  script_path: "+fallbackText(cfg.Extension.ScriptPath, "未设置"))
	}
	if cfg.Extension.ResidentialChain != nil {
		lines = append(lines, "  chains 路由模式: "+fallbackText(cfg.Extension.ResidentialChain.Mode, "rule"))
	}
	if status != "" {
		lines = append(lines, "", status)
	}
	lines = append(lines,
		"",
		renderSectionTitle("工作台操作"),
		"  1 切到 chains",
		"  2 切到 script",
		"  0 关闭扩展",
		"  R 切换 chains 的 rule / global",
		"  P 编辑 script_path",
		"",
		noteLine("chains 适合 AI 客户端稳定使用；script 适合已有自定义脚本。"),
	)
	return lines
}

func renderChainWorkspaceLines(cfg *config.Config, status string) []string {
	chain := ensureConsoleChain(cfg)
	lines := []string{
		renderSectionTitle("住宅代理配置"),
		"  服务器: " + fallbackText(chain.ProxyServer, "未设置"),
		fmt.Sprintf("  端口: %s", fallbackText(strconv.Itoa(chain.ProxyPort), "未设置")),
		"  协议: " + fallbackText(chain.ProxyType, "socks5"),
		"  用户名: " + fallbackText(chain.ProxyUsername, "未设置"),
		"  密码: " + maskedSecret(chain.ProxyPassword),
		"  机场组: " + fallbackText(chain.AirportGroup, "Auto"),
	}
	if status != "" {
		lines = append(lines, "", status)
	}
	lines = append(lines,
		"",
		renderSectionTitle("工作台操作"),
		"  S 编辑住宅代理服务器",
		"  O 编辑住宅代理端口",
		"  T 切换代理协议 socks5 / http",
		"  U 编辑用户名",
		"  P 编辑密码",
		"  A 编辑机场出口组",
		"",
		noteLine("如果要启用 chains，还需要保证机场组名称和住宅代理参数都可用。"),
	)
	return lines
}

func maskedSecret(value string) string {
	if strings.TrimSpace(value) == "" {
		return "未设置"
	}
	return "已设置"
}
