package cmd

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/tght/lan-proxy-gateway/internal/config"
)

const (
	detailTitleProxyWorkspace        = "代理来源工作台"
	detailTitleRuntimeWorkspace      = "运行模式工作台"
	detailTitleRulesWorkspace        = "规则工作台"
	detailTitleSubscriptionWorkspace = "订阅管理工作台"
	detailTitleExtensionWorkspace    = "扩展模式工作台"
	detailTitleChainWorkspace        = "住宅代理工作台"
)

func (m *runtimeConsoleModel) applyUpdatedConfig(cfg *config.Config) {
	m.cfg = cfg
	m.client = newConsoleClient(cfg)
	m.refreshSnapshot()
}

func (m *runtimeConsoleModel) openProxyWorkspace(status string) {
	cfg := loadConfigOrDefault()
	m.applyUpdatedConfig(cfg)
	m.setDetail(detailTitleProxyWorkspace, renderProxyWorkspaceLines(cfg, status))
	m.focus = consoleFocusDetail
}

func (m *runtimeConsoleModel) openRuntimeWorkspace(status string) {
	cfg := loadConfigOrDefault()
	m.applyUpdatedConfig(cfg)
	m.setDetail(detailTitleRuntimeWorkspace, renderRuntimeWorkspaceLines(cfg, status))
	m.focus = consoleFocusDetail
}

func (m *runtimeConsoleModel) openRulesWorkspace(status string) {
	cfg := loadConfigOrDefault()
	m.applyUpdatedConfig(cfg)
	m.setDetail(detailTitleRulesWorkspace, renderRulesWorkspaceLines(cfg, status))
	m.focus = consoleFocusDetail
}

func (m *runtimeConsoleModel) openSubscriptionWorkspace(status string) {
	cfg := loadConfigOrDefault()
	m.applyUpdatedConfig(cfg)
	m.setDetail(detailTitleSubscriptionWorkspace, renderSubscriptionWorkspaceLines(cfg, status))
	m.focus = consoleFocusDetail
}

func (m *runtimeConsoleModel) openExtensionWorkspace(status string) {
	cfg := loadConfigOrDefault()
	m.applyUpdatedConfig(cfg)
	m.setDetail(detailTitleExtensionWorkspace, renderExtensionWorkspaceLines(cfg, status))
	m.focus = consoleFocusDetail
}

func (m *runtimeConsoleModel) openChainWorkspace(status string) {
	cfg := loadConfigOrDefault()
	m.applyUpdatedConfig(cfg)
	m.setDetail(detailTitleChainWorkspace, renderChainWorkspaceLines(cfg, status))
	m.focus = consoleFocusDetail
}

func (m *runtimeConsoleModel) handleConfigMutation(open func(string), successText string, fn func() (*config.Config, error)) (tea.Model, tea.Cmd) {
	cfg, err := fn()
	if err != nil {
		open(errorLine(err.Error()))
		return *m, nil
	}
	m.applyUpdatedConfig(cfg)
	open(successLine(successText))
	return *m, nil
}

func (m *runtimeConsoleModel) openWorkspacePrompt(title, label, placeholder, current string, apply func(m *runtimeConsoleModel, value string) error) {
	m.openInputPrompt(&inputPrompt{
		title:       title,
		label:       label,
		placeholder: placeholder,
		apply:       apply,
	}, current, []string{
		renderSectionTitle(label),
		noteLine("会弹出输入框；按 Enter 保存，按 Esc 取消。"),
	})
}

func (m *runtimeConsoleModel) handleDetailWorkspaceKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.detailTitle {
	case detailTitleProxyWorkspace:
		return m.handleProxyWorkspaceKey(msg)
	case detailTitleRuntimeWorkspace:
		return m.handleRuntimeWorkspaceKey(msg)
	case detailTitleRulesWorkspace:
		return m.handleRulesWorkspaceKey(msg)
	case detailTitleSubscriptionWorkspace:
		return m.handleSubscriptionWorkspaceKey(msg)
	case detailTitleExtensionWorkspace:
		return m.handleExtensionWorkspaceKey(msg)
	case detailTitleChainWorkspace:
		return m.handleChainWorkspaceKey(msg)
	default:
		return *m, nil, false
	}
}

func (m *runtimeConsoleModel) handleSubscriptionWorkspaceKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	cfg := loadConfigOrDefault()
	current := activeProxyProfile(cfg)

	switch strings.ToLower(msg.String()) {
	case "1":
		m.openWorkspacePrompt(detailTitleSubscriptionWorkspace, "新建 URL 订阅 · 第一步", "先输入订阅名称", "", func(m *runtimeConsoleModel, name string) error {
			name = strings.TrimSpace(name)
			if name == "" {
				return fmt.Errorf("订阅名称不能为空")
			}
			m.openWorkspacePrompt(detailTitleSubscriptionWorkspace, "新建 URL 订阅 · 第二步", "再粘贴订阅链接", "", func(m *runtimeConsoleModel, value string) error {
				cfg, err := createSubscriptionProfile(name, "url", value)
				if err != nil {
					return err
				}
				m.applyUpdatedConfig(cfg)
				m.openSubscriptionWorkspace(successLine("已新建并切换到 URL 订阅: " + name))
				return nil
			})
			return nil
		})
		return *m, nil, true
	case "2":
		m.openWorkspacePrompt(detailTitleSubscriptionWorkspace, "新建本地文件订阅 · 第一步", "先输入订阅名称", "", func(m *runtimeConsoleModel, name string) error {
			name = strings.TrimSpace(name)
			if name == "" {
				return fmt.Errorf("订阅名称不能为空")
			}
			m.openWorkspacePrompt(detailTitleSubscriptionWorkspace, "新建本地文件订阅 · 第二步", "再输入本地配置文件路径", "", func(m *runtimeConsoleModel, value string) error {
				cfg, err := createSubscriptionProfile(name, "file", value)
				if err != nil {
					return err
				}
				m.applyUpdatedConfig(cfg)
				m.openSubscriptionWorkspace(successLine("已新建并切换到本地文件订阅: " + name))
				return nil
			})
			return nil
		})
		return *m, nil, true
	case "3":
		m.openWorkspacePrompt(detailTitleSubscriptionWorkspace, "切换订阅", "输入要切换的订阅名称", cfg.Proxy.CurrentProfile, func(m *runtimeConsoleModel, value string) error {
			nextCfg, err := switchSubscriptionProfile(value)
			if err != nil {
				return err
			}
			m.applyUpdatedConfig(nextCfg)
			m.openSubscriptionWorkspace(successLine("已切换当前订阅: " + strings.TrimSpace(value)))
			return nil
		})
		return *m, nil, true
	case "u":
		if current.Source != "url" {
			m.openSubscriptionWorkspace(errorLine("当前订阅不是 url 模式，先切到 URL 订阅再编辑链接"))
			return *m, nil, true
		}
		m.openWorkspacePrompt(detailTitleSubscriptionWorkspace, "编辑订阅链接", "输入新的订阅链接", current.SubscriptionURL, func(m *runtimeConsoleModel, value string) error {
			nextCfg, err := updateSubscriptionURL(value)
			if err != nil {
				return err
			}
			m.applyUpdatedConfig(nextCfg)
			m.openSubscriptionWorkspace(successLine("订阅链接已更新"))
			return nil
		})
		return *m, nil, true
	case "f":
		if current.Source != "file" {
			m.openSubscriptionWorkspace(errorLine("当前订阅不是 file 模式，先切到本地文件订阅再编辑路径"))
			return *m, nil, true
		}
		m.openWorkspacePrompt(detailTitleSubscriptionWorkspace, "编辑本地配置文件路径", "例如 /path/to/clash.yaml", current.ConfigFile, func(m *runtimeConsoleModel, value string) error {
			nextCfg, err := updateProxyConfigFile(value)
			if err != nil {
				return err
			}
			m.applyUpdatedConfig(nextCfg)
			m.openSubscriptionWorkspace(successLine("本地配置文件路径已更新"))
			return nil
		})
		return *m, nil, true
	case "n":
		m.openWorkspacePrompt(detailTitleSubscriptionWorkspace, "重命名当前订阅", "输入新的订阅名称", current.Name, func(m *runtimeConsoleModel, value string) error {
			nextCfg, err := updateSubscriptionName(value)
			if err != nil {
				return err
			}
			m.applyUpdatedConfig(nextCfg)
			m.openSubscriptionWorkspace(successLine("当前订阅已重命名"))
			return nil
		})
		return *m, nil, true
	default:
		return *m, nil, false
	}
}

func (m *runtimeConsoleModel) handleProxyWorkspaceKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	cfg := loadConfigOrDefault()
	switch msg.String() {
	case "1":
		model, cmd := m.handleConfigMutation(m.openProxyWorkspace, "已切换到订阅链接模式", func() (*config.Config, error) {
			return updateProxySource("url")
		})
		return model, cmd, true
	case "2":
		model, cmd := m.handleConfigMutation(m.openProxyWorkspace, "已切换到本地文件模式", func() (*config.Config, error) {
			return updateProxySource("file")
		})
		return model, cmd, true
	case "u":
		m.openWorkspacePrompt(detailTitleProxyWorkspace, "订阅链接", "粘贴新的订阅链接", cfg.Proxy.SubscriptionURL, func(m *runtimeConsoleModel, value string) error {
			cfg, err := updateSubscriptionURL(value)
			if err != nil {
				return err
			}
			m.applyUpdatedConfig(cfg)
			m.openProxyWorkspace(successLine("订阅链接已更新"))
			return nil
		})
		return *m, nil, true
	case "f":
		m.openWorkspacePrompt(detailTitleProxyWorkspace, "本地配置文件路径", "例如 /path/to/clash.yaml", cfg.Proxy.ConfigFile, func(m *runtimeConsoleModel, value string) error {
			cfg, err := updateProxyConfigFile(value)
			if err != nil {
				return err
			}
			m.applyUpdatedConfig(cfg)
			m.openProxyWorkspace(successLine("本地配置文件路径已更新"))
			return nil
		})
		return *m, nil, true
	case "n":
		m.openWorkspacePrompt(detailTitleProxyWorkspace, "订阅名称", "例如 subscription / Auto", cfg.Proxy.SubscriptionName, func(m *runtimeConsoleModel, value string) error {
			cfg, err := updateSubscriptionName(value)
			if err != nil {
				return err
			}
			m.applyUpdatedConfig(cfg)
			m.openProxyWorkspace(successLine("订阅名称已更新"))
			return nil
		})
		return *m, nil, true
	default:
		return *m, nil, false
	}
}

func (m *runtimeConsoleModel) handleRuntimeWorkspaceKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	cfg := loadConfigOrDefault()
	switch msg.String() {
	case "1":
		model, cmd := m.handleConfigMutation(m.openRuntimeWorkspace, fmt.Sprintf("TUN 已切换为 %s", onOff(!cfg.Runtime.Tun.Enabled)), func() (*config.Config, error) {
			return updateTunEnabled(!cfg.Runtime.Tun.Enabled)
		})
		return model, cmd, true
	case "2":
		model, cmd := m.handleConfigMutation(m.openRuntimeWorkspace, fmt.Sprintf("本机绕过代理已切换为 %s", onOff(!cfg.Runtime.Tun.BypassLocal)), func() (*config.Config, error) {
			return updateBypassLocal(!cfg.Runtime.Tun.BypassLocal)
		})
		return model, cmd, true
	default:
		return *m, nil, false
	}
}

func (m *runtimeConsoleModel) handleRulesWorkspaceKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	ruleMap := map[string]string{
		"1": "lan",
		"2": "china",
		"3": "apple",
		"4": "nintendo",
		"5": "global",
		"6": "ads",
	}
	rule, ok := ruleMap[msg.String()]
	if !ok {
		return *m, nil, false
	}

	cfg := loadConfigOrDefault()
	current := map[string]bool{
		"lan":      cfg.Rules.LanDirectEnabled(),
		"china":    cfg.Rules.ChinaDirectEnabled(),
		"apple":    cfg.Rules.AppleRulesEnabled(),
		"nintendo": cfg.Rules.NintendoProxyEnabled(),
		"global":   cfg.Rules.GlobalProxyEnabled(),
		"ads":      cfg.Rules.AdsRejectEnabled(),
	}[rule]

	labelMap := map[string]string{
		"lan":      "局域网直连",
		"china":    "国内直连",
		"apple":    "Apple 规则",
		"nintendo": "Nintendo 代理",
		"global":   "国外代理",
		"ads":      "广告拦截",
	}
	model, cmd := m.handleConfigMutation(m.openRulesWorkspace, fmt.Sprintf("%s 已切换为 %s", labelMap[rule], onOff(!current)), func() (*config.Config, error) {
		return updateRuleToggle(rule, !current)
	})
	return model, cmd, true
}

func (m *runtimeConsoleModel) handleExtensionWorkspaceKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	cfg := loadConfigOrDefault()
	switch strings.ToLower(msg.String()) {
	case "1":
		model, cmd := m.handleConfigMutation(m.openExtensionWorkspace, "已切换到 chains 模式", func() (*config.Config, error) {
			return updateExtensionMode("chains")
		})
		return model, cmd, true
	case "2":
		model, cmd := m.handleConfigMutation(m.openExtensionWorkspace, "已切换到 script 模式", func() (*config.Config, error) {
			return updateExtensionMode("script")
		})
		return model, cmd, true
	case "0":
		model, cmd := m.handleConfigMutation(m.openExtensionWorkspace, "已关闭扩展模式", func() (*config.Config, error) {
			return updateExtensionMode("off")
		})
		return model, cmd, true
	case "r":
		chain := ensureConsoleChain(cfg)
		nextMode := "rule"
		if chain.Mode != "global" {
			nextMode = "global"
		}
		model, cmd := m.handleConfigMutation(m.openExtensionWorkspace, "chains 路由模式已切换为 "+nextMode, func() (*config.Config, error) {
			return updateChainMode(nextMode)
		})
		return model, cmd, true
	case "p":
		m.openWorkspacePrompt(detailTitleExtensionWorkspace, "script_path", "例如 ./script-demo.js", cfg.Extension.ScriptPath, func(m *runtimeConsoleModel, value string) error {
			cfg, err := updateScriptPath(value)
			if err != nil {
				return err
			}
			m.applyUpdatedConfig(cfg)
			m.openExtensionWorkspace(successLine("script_path 已更新"))
			return nil
		})
		return *m, nil, true
	default:
		return *m, nil, false
	}
}

func (m *runtimeConsoleModel) handleChainWorkspaceKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	cfg := loadConfigOrDefault()
	chain := ensureConsoleChain(cfg)

	switch strings.ToLower(msg.String()) {
	case "s":
		m.openWorkspacePrompt(detailTitleChainWorkspace, "住宅代理服务器", "例如 1.2.3.4 或 proxy.example.com", chain.ProxyServer, func(m *runtimeConsoleModel, value string) error {
			cfg, err := updateChainServer(value)
			if err != nil {
				return err
			}
			m.applyUpdatedConfig(cfg)
			m.openChainWorkspace(successLine("住宅代理服务器已更新"))
			return nil
		})
		return *m, nil, true
	case "o":
		current := ""
		if chain.ProxyPort > 0 {
			current = fmt.Sprintf("%d", chain.ProxyPort)
		}
		m.openWorkspacePrompt(detailTitleChainWorkspace, "住宅代理端口", "例如 443", current, func(m *runtimeConsoleModel, value string) error {
			cfg, err := updateChainPort(value)
			if err != nil {
				return err
			}
			m.applyUpdatedConfig(cfg)
			m.openChainWorkspace(successLine("住宅代理端口已更新"))
			return nil
		})
		return *m, nil, true
	case "t":
		nextType := "socks5"
		if chain.ProxyType == "socks5" {
			nextType = "http"
		}
		model, cmd := m.handleConfigMutation(m.openChainWorkspace, "住宅代理协议已切换为 "+nextType, func() (*config.Config, error) {
			return updateChainProxyType(nextType)
		})
		return model, cmd, true
	case "u":
		m.openWorkspacePrompt(detailTitleChainWorkspace, "住宅代理用户名", "无需认证可留空", chain.ProxyUsername, func(m *runtimeConsoleModel, value string) error {
			cfg, err := updateChainUsername(value)
			if err != nil {
				return err
			}
			m.applyUpdatedConfig(cfg)
			m.openChainWorkspace(successLine("住宅代理用户名已更新"))
			return nil
		})
		return *m, nil, true
	case "p":
		m.openWorkspacePrompt(detailTitleChainWorkspace, "住宅代理密码", "无需认证可留空", chain.ProxyPassword, func(m *runtimeConsoleModel, value string) error {
			cfg, err := updateChainPassword(value)
			if err != nil {
				return err
			}
			m.applyUpdatedConfig(cfg)
			m.openChainWorkspace(successLine("住宅代理密码已更新"))
			return nil
		})
		return *m, nil, true
	case "a":
		m.openWorkspacePrompt(detailTitleChainWorkspace, "机场出口组", "例如 Auto / 自动选择 / 最低延迟", chain.AirportGroup, func(m *runtimeConsoleModel, value string) error {
			cfg, err := updateChainAirportGroup(value)
			if err != nil {
				return err
			}
			m.applyUpdatedConfig(cfg)
			m.openChainWorkspace(successLine("机场出口组已更新"))
			return nil
		})
		return *m, nil, true
	default:
		return *m, nil, false
	}
}
