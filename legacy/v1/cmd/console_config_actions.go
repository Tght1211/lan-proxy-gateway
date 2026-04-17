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

func activeProxyProfile(cfg *config.Config) config.ProxyProfile {
	return cfg.Proxy.ActiveProfile()
}

func listProxyProfiles(cfg *config.Config) []config.ProxyProfile {
	if cfg == nil {
		return nil
	}
	profiles := append([]config.ProxyProfile(nil), cfg.Proxy.Profiles...)
	if len(profiles) == 0 {
		profiles = []config.ProxyProfile{cfg.Proxy.ActiveProfile()}
	}
	return profiles
}

func applyActiveProxyProfile(cfg *config.Config, profile config.ProxyProfile) {
	cfg.Proxy.Source = profile.Source
	cfg.Proxy.SubscriptionName = profile.Name
	cfg.Proxy.SubscriptionURL = profile.SubscriptionURL
	cfg.Proxy.ConfigFile = profile.ConfigFile
	cfg.Proxy.CurrentProfile = profile.Name
}

func upsertProxyProfile(cfg *config.Config, profile config.ProxyProfile, activate bool) {
	profile.Name = strings.TrimSpace(profile.Name)
	if profile.Name == "" {
		profile.Name = "subscription"
	}
	profile.Source = strings.ToLower(strings.TrimSpace(profile.Source))
	if profile.Source == "" {
		profile.Source = "url"
	}
	profile.SubscriptionURL = strings.TrimSpace(profile.SubscriptionURL)
	profile.ConfigFile = strings.TrimSpace(profile.ConfigFile)

	profiles := listProxyProfiles(cfg)
	replaced := false
	for idx := range profiles {
		if profiles[idx].Name == profile.Name {
			profiles[idx] = profile
			replaced = true
			break
		}
	}
	if !replaced {
		profiles = append(profiles, profile)
	}
	cfg.Proxy.Profiles = profiles
	if activate {
		applyActiveProxyProfile(cfg, profile)
	}
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
	case "proxy", "direct":
		return "proxy", nil
	default:
		return "", fmt.Errorf("代理来源仅支持 url、file 或 proxy")
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
				return fmt.Errorf("还没有设置订阅链接，先用 proxy url <链接> 填写再切换")
			}
		case "file":
			if strings.TrimSpace(cfg.Proxy.ConfigFile) == "" {
				return fmt.Errorf("还没有设置本地配置文件路径，先用 proxy file <路径> 填写再切换")
			}
		case "proxy":
			// proxy 模式下 DirectProxy 为空时也允许切换，启动时会再次校验
			cfg.Proxy.Source = source
			cfg.Proxy.SubscriptionName = "direct"
			if cfg.Proxy.DirectProxy == nil {
				cfg.Proxy.DirectProxy = &config.DirectProxyConfig{
					Name: "MyProxy",
					Type: "socks5",
				}
			}
			return nil
		default:
			return fmt.Errorf("代理来源仅支持 url、file 或 proxy")
		}
		cfg.Proxy.Source = source
		upsertProxyProfile(cfg, activeProxyProfile(cfg), true)
		return nil
	})
}

func updateSubscriptionURL(url string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		url = strings.TrimSpace(url)
		if url == "" {
			return fmt.Errorf("订阅链接不能为空")
		}
		if _, err := validateSubscriptionURL(url); err != nil {
			return fmt.Errorf("订阅链接校验失败: %w", err)
		}
		cfg.Proxy.SubscriptionURL = url
		if cfg.Proxy.Source == "" {
			cfg.Proxy.Source = "url"
		}
		upsertProxyProfile(cfg, activeProxyProfile(cfg), true)
		return nil
	})
}

func updateProxyConfigFile(path string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		validated, _, err := validateSubscriptionFile(path)
		if err != nil {
			return err
		}
		cfg.Proxy.ConfigFile = validated
		if cfg.Proxy.Source == "" {
			cfg.Proxy.Source = "file"
		}
		upsertProxyProfile(cfg, activeProxyProfile(cfg), true)
		return nil
	})
}

func updateSubscriptionName(name string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("订阅名称不能为空")
		}
		current := strings.TrimSpace(cfg.Proxy.CurrentProfile)
		if current == "" {
			current = cfg.Proxy.SubscriptionName
		}
		for idx := range cfg.Proxy.Profiles {
			if cfg.Proxy.Profiles[idx].Name == current {
				cfg.Proxy.Profiles[idx].Name = name
				break
			}
		}
		cfg.Proxy.SubscriptionName = name
		cfg.Proxy.CurrentProfile = name
		upsertProxyProfile(cfg, activeProxyProfile(cfg), true)
		return nil
	})
}

func createSubscriptionProfile(name, source, value string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("订阅名称不能为空")
		}
		source, err := normalizeProxySource(source)
		if err != nil {
			return err
		}

		profile := config.ProxyProfile{
			Name:   name,
			Source: source,
		}
		switch source {
		case "url":
			value = strings.TrimSpace(value)
			if value == "" {
				return fmt.Errorf("订阅链接不能为空")
			}
			if _, err := validateSubscriptionURL(value); err != nil {
				return fmt.Errorf("订阅链接校验失败: %w", err)
			}
			profile.SubscriptionURL = value
		case "file":
			validated, _, err := validateSubscriptionFile(value)
			if err != nil {
				return err
			}
			profile.ConfigFile = validated
		}
		upsertProxyProfile(cfg, profile, true)
		return nil
	})
}

func switchSubscriptionProfile(name string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("请填写要切换的订阅名称")
		}
		for _, profile := range listProxyProfiles(cfg) {
			if profile.Name != name {
				continue
			}
			applyActiveProxyProfile(cfg, profile)
			return nil
		}
		return fmt.Errorf("未找到订阅: %s", name)
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

func providerSourceHost(cfg *config.Config) string {
	profile := activeProxyProfile(cfg)
	if profile.Source != "url" {
		return "本地文件"
	}
	value := strings.TrimSpace(profile.SubscriptionURL)
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	if idx := strings.Index(value, "/"); idx >= 0 {
		value = value[:idx]
	}
	if value == "" {
		return "未设置"
	}
	return value
}

func compactProfileList(cfg *config.Config) string {
	profiles := listProxyProfiles(cfg)
	if len(profiles) == 0 {
		return "无"
	}
	names := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		name := profile.Name
		if profile.Name == cfg.Proxy.CurrentProfile {
			name += " (current)"
		}
		names = append(names, name)
	}
	return strings.Join(names, " / ")
}

func renderSubscriptionProfilesLines(cfg *config.Config) []string {
	lines := []string{renderSectionTitle("订阅档案")}
	for _, profile := range listProxyProfiles(cfg) {
		current := ""
		if profile.Name == cfg.Proxy.CurrentProfile {
			current = "  (current)"
		}
		if profile.Source == "url" {
			lines = append(lines, "  - "+profile.Name+" · url · "+shortText(profile.SubscriptionURL, 56)+current)
		} else {
			lines = append(lines, "  - "+profile.Name+" · file · "+shortText(profile.ConfigFile, 56)+current)
		}
	}
	return lines
}

func renderSubscriptionWorkspaceLines(cfg *config.Config, status string) []string {
	profile := activeProxyProfile(cfg)
	lines := []string{
		renderSectionTitle("当前订阅"),
		"  当前档案: " + profile.Name,
		"  来源: " + profile.Source,
		"  订阅概览: " + compactProfileList(cfg),
	}
	if profile.Source == "url" {
		lines = append(lines, "  订阅链接: "+shortText(profile.SubscriptionURL, 72))
		lines = append(lines, "  来源站点: "+providerSourceHost(cfg))
	} else {
		lines = append(lines, "  本地配置: "+fallbackText(profile.ConfigFile, "未设置"))
	}
	lines = append(lines, "")
	lines = append(lines, renderSubscriptionProfilesLines(cfg)...)
	if status != "" {
		lines = append(lines, "", status)
	}
	lines = append(lines,
		"",
		renderSectionTitle("工作台操作"),
		"  1 新建 URL 订阅",
		"  2 新建本地文件订阅",
		"  3 切换当前订阅",
		"  4 编辑当前订阅链接",
		"  5 编辑当前本地配置路径",
		"  6 重命名当前订阅",
		"  0 返回上一级",
		"",
		noteLine("订阅切换会写入 gateway.yaml，重启网关后生效。"),
	)
	return lines
}

func renderProxyWorkspaceLines(cfg *config.Config, status string) []string {
	lines := []string{
		renderSectionTitle("当前代理来源"),
		"  来源模式: " + proxySourceSummary(cfg),
	}
	switch cfg.Proxy.Source {
	case "url":
		profile := activeProxyProfile(cfg)
		lines = append(lines, "  订阅名称: "+fallbackText(profile.Name, "subscription"))
		lines = append(lines, "  订阅链接: "+shortText(profile.SubscriptionURL, 72))
	case "file":
		profile := activeProxyProfile(cfg)
		lines = append(lines, "  订阅名称: "+fallbackText(profile.Name, "subscription"))
		lines = append(lines, "  本地配置: "+fallbackText(profile.ConfigFile, "未设置"))
	case "proxy":
		if dp := cfg.Proxy.DirectProxy; dp != nil {
			lines = append(lines, "  代理服务器: "+fallbackText(dp.Server, "未设置"))
			if dp.Port > 0 {
				lines = append(lines, fmt.Sprintf("  端口: %d", dp.Port))
			}
			lines = append(lines, "  协议: "+fallbackText(dp.Type, "socks5"))
		}
	}
	if status != "" {
		lines = append(lines, "", status)
	}
	lines = append(lines,
		"",
		renderSectionTitle("工作台操作"),
		"  1 切到订阅链接模式",
		"  2 切到本地文件模式",
		"  3 切到直接代理模式",
		"  U 编辑订阅链接",
		"  F 编辑本地配置文件路径",
		"  N 编辑订阅名称",
		"  D 配置直接代理服务器",
		"",
		noteLine("修改会写入 gateway.yaml，重启网关后生效。"),
	)
	return lines
}

func proxySourceSummary(cfg *config.Config) string {
	switch cfg.Proxy.Source {
	case "url":
		return "订阅链接 (url)"
	case "file":
		return "本地文件 (file)"
	case "proxy":
		if dp := cfg.Proxy.DirectProxy; dp != nil && dp.Server != "" {
			return fmt.Sprintf("直接代理 (proxy) — %s %s:%d", dp.Type, dp.Server, dp.Port)
		}
		return "直接代理 (proxy) — 未完整配置"
	}
	return cfg.Proxy.Source
}

func renderRuntimeWorkspaceLines(cfg *config.Config, status string) []string {
	lines := []string{
		renderSectionTitle("当前运行模式"),
		"  TUN: " + consoleOnOff(cfg.Runtime.Tun.Enabled),
		"  本机绕过代理: " + consoleOnOff(cfg.Runtime.Tun.BypassLocal),
		"  局域网共享: " + consoleState(cfg.Runtime.Tun.Enabled, "已开启（依赖 TUN）", "不可用"),
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
		"  0 返回上一级",
		"",
		noteLine(fmt.Sprintf("TUN 是局域网共享的核心开关；关闭后局域网设备无法再通过这台机器上网。改完通常需要 %s。", elevatedCmd("restart"))),
	)
	return lines
}

func renderRulesWorkspaceLines(cfg *config.Config, status string) []string {
	lines := []string{
		renderSectionTitle("当前规则开关"),
		"  1 局域网直连: " + consoleOnOff(cfg.Rules.LanDirectEnabled()),
		"  2 国内直连: " + consoleOnOff(cfg.Rules.ChinaDirectEnabled()),
		"  3 Apple 规则: " + consoleOnOff(cfg.Rules.AppleRulesEnabled()),
		"  4 Nintendo 代理: " + consoleOnOff(cfg.Rules.NintendoProxyEnabled()),
		"  5 国外代理: " + consoleOnOff(cfg.Rules.GlobalProxyEnabled()),
		"  6 广告拦截: " + consoleOnOff(cfg.Rules.AdsRejectEnabled()),
	}
	if status != "" {
		lines = append(lines, "", status)
	}
	lines = append(lines,
		"",
		renderSectionTitle("工作台操作"),
		"  按 1-6 直接切换对应规则开关",
		"  0 返回上一级",
		"",
		noteLine("这组规则更偏向推荐默认值，适合先用再细调。"),
	)
	return lines
}

func renderExtensionWorkspaceLines(cfg *config.Config, status string) []string {
	lines := []string{
		renderSectionTitle("当前扩展模式"),
		"  模式: " + extensionModeName(cfg.Extension.Mode),
		"  说明: script 与 chains 二选一",
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
		"  3 关闭扩展",
		"  4 切换 chains 的 rule / global",
		"  5 编辑 script_path",
		"  0 返回上一级",
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
		"  1 编辑住宅代理服务器",
		"  2 编辑住宅代理端口",
		"  3 切换代理协议 socks5 / http",
		"  4 编辑用户名",
		"  5 编辑密码",
		"  6 编辑机场出口组",
		"  0 返回上一级",
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

// ============================================================
// 直接代理服务器（source: proxy）管理
// ============================================================

func ensureDirectProxy(cfg *config.Config) *config.DirectProxyConfig {
	if cfg.Proxy.DirectProxy == nil {
		cfg.Proxy.DirectProxy = &config.DirectProxyConfig{
			Name: "MyProxy",
			Type: "socks5",
		}
	}
	if cfg.Proxy.DirectProxy.Type == "" {
		cfg.Proxy.DirectProxy.Type = "socks5"
	}
	if cfg.Proxy.DirectProxy.Name == "" {
		cfg.Proxy.DirectProxy.Name = "MyProxy"
	}
	return cfg.Proxy.DirectProxy
}

func updateDirectProxyServer(server string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		server = strings.TrimSpace(server)
		if server == "" {
			return fmt.Errorf("代理服务器地址不能为空")
		}
		dp := ensureDirectProxy(cfg)
		dp.Server = server
		cfg.Proxy.Source = "proxy"
		return nil
	})
}

func updateDirectProxyPort(value string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		value = strings.TrimSpace(value)
		var port int
		if _, err := fmt.Sscanf(value, "%d", &port); err != nil || port <= 0 || port > 65535 {
			return fmt.Errorf("端口必须是 1-65535 之间的整数")
		}
		dp := ensureDirectProxy(cfg)
		dp.Port = port
		cfg.Proxy.Source = "proxy"
		return nil
	})
}

func updateDirectProxyType(value string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "socks5" && value != "http" {
			return fmt.Errorf("协议类型仅支持 socks5 或 http")
		}
		dp := ensureDirectProxy(cfg)
		dp.Type = value
		cfg.Proxy.Source = "proxy"
		return nil
	})
}

func updateDirectProxyUsername(name string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		dp := ensureDirectProxy(cfg)
		dp.Username = strings.TrimSpace(name)
		return nil
	})
}

func updateDirectProxyPassword(password string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		dp := ensureDirectProxy(cfg)
		dp.Password = strings.TrimSpace(password)
		return nil
	})
}

func updateDirectProxyName(name string) (*config.Config, error) {
	return updateConsoleConfig(func(cfg *config.Config) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("节点名称不能为空")
		}
		dp := ensureDirectProxy(cfg)
		dp.Name = name
		return nil
	})
}

func renderDirectProxyWorkspaceLines(cfg *config.Config, status string) []string {
	dp := ensureDirectProxy(cfg)
	portStr := "未设置"
	if dp.Port > 0 {
		portStr = fmt.Sprintf("%d", dp.Port)
	}
	lines := []string{
		renderSectionTitle("直接代理服务器"),
		"  节点名称: " + fallbackText(dp.Name, "MyProxy"),
		"  服务器:   " + fallbackText(dp.Server, "未设置"),
		"  端口:     " + portStr,
		"  协议:     " + fallbackText(dp.Type, "socks5"),
		"  用户名:   " + fallbackText(dp.Username, "未设置（无认证）"),
		"  密码:     " + maskedSecret(dp.Password),
	}
	if status != "" {
		lines = append(lines, "", status)
	}
	lines = append(lines,
		"",
		renderSectionTitle("工作台操作"),
		"  S 编辑代理服务器地址",
		"  O 编辑代理端口",
		"  T 切换协议 socks5 / http",
		"  U 编辑用户名",
		"  P 编辑密码",
		"  N 编辑节点名称",
		"",
		noteLine("填好服务器和端口后，运行 "+elevatedCmd("restart")+" 即可让局域网设备蹭代理。"),
	)
	return lines
}
