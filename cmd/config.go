package cmd

// config.go 提供一套**无交互**配置命令，让脚本 / AI agent / Claude Code 能
// 完整地查看与修改网关配置，而不必驱动交互式 TUI 菜单。所有写命令都通过
// internal/app 的同一套 facade（存盘 + mihomo 在跑时热重载）。

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "非交互地查看 / 修改配置（供脚本和 AI agent 用）",
	Long: `非交互配置命令。每个子命令改完即存盘，mihomo 在跑时自动热重载。

例：
  gateway config show --json
  gateway config source --type subscription --url https://example.com/sub
  gateway config mode rule
  gateway config tun on
  gateway config adblock off
  gateway config rule add proxy DOMAIN-SUFFIX,openai.com
  gateway config rule list --json`,
}

// configView 是 config show 的机器可读快照（不含敏感的脚本路径细节）。
type configView struct {
	Source struct {
		Type   string `json:"type"`
		URL    string `json:"url,omitempty"`
		Path   string `json:"path,omitempty"`
		Server string `json:"server,omitempty"`
		Port   int    `json:"port,omitempty"`
		Kind   string `json:"kind,omitempty"`
	} `json:"source"`
	Mode        string `json:"mode"`
	TUN         bool   `json:"tun"`
	Adblock     bool   `json:"adblock"`
	GatewayMode string `json:"gateway_mode"`
	DNS         struct {
		Enabled bool `json:"enabled"`
		Port    int  `json:"port"`
	} `json:"dns"`
	Rules struct {
		Direct []string `json:"direct"`
		Proxy  []string `json:"proxy"`
		Reject []string `json:"reject"`
	} `json:"rules"`
	Ports config.RuntimePorts `json:"ports"`
}

func buildConfigView(a *app.App) configView {
	c := a.Cfg
	var v configView
	v.Source.Type = c.Source.Type
	v.Source.URL = c.Source.Subscription.URL
	v.Source.Path = c.Source.File.Path
	switch c.Source.Type {
	case config.SourceTypeExternal:
		v.Source.Server = c.Source.External.Server
		v.Source.Port = c.Source.External.Port
		v.Source.Kind = c.Source.External.Kind
	case config.SourceTypeRemote:
		v.Source.Server = c.Source.Remote.Server
		v.Source.Port = c.Source.Remote.Port
		v.Source.Kind = c.Source.Remote.Kind
	}
	v.Mode = c.Traffic.Mode
	v.TUN = c.Gateway.TUN.Enabled
	v.Adblock = c.Traffic.Adblock
	v.GatewayMode = c.Gateway.Mode
	if v.GatewayMode == "" {
		v.GatewayMode = config.GatewayModeTUN
	}
	v.DNS.Enabled = c.Gateway.DNS.Enabled
	v.DNS.Port = c.Gateway.DNS.Port
	v.Rules.Direct = c.Traffic.Extras.Direct
	v.Rules.Proxy = c.Traffic.Extras.Proxy
	v.Rules.Reject = c.Traffic.Extras.Reject
	v.Ports = c.Runtime.Ports
	return v
}

// ---- config show ----

var configShowJSON bool

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "打印当前配置（--json 输出机器可读）",
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		v := buildConfigView(a)
		if configShowJSON {
			b, _ := json.MarshalIndent(v, "", "  ")
			fmt.Println(string(b))
			return nil
		}
		fmt.Printf("源:       %s\n", v.Source.Type)
		if v.Source.URL != "" {
			fmt.Printf("  url:    %s\n", v.Source.URL)
		}
		if v.Source.Path != "" {
			fmt.Printf("  path:   %s\n", v.Source.Path)
		}
		if v.Source.Server != "" {
			fmt.Printf("  server: %s:%d (%s)\n", v.Source.Server, v.Source.Port, v.Source.Kind)
		}
		fmt.Printf("模式:     %s\n", v.Mode)
		fmt.Printf("TUN:      %v\n", v.TUN)
		fmt.Printf("去广告:   %v\n", v.Adblock)
		fmt.Printf("网关模式: %s\n", v.GatewayMode)
		fmt.Printf("DNS:      enabled=%v port=%d\n", v.DNS.Enabled, v.DNS.Port)
		fmt.Printf("端口:     mixed=%d api=%d redir=%d\n", v.Ports.Mixed, v.Ports.API, v.Ports.Redir)
		fmt.Printf("自定义规则: direct=%d proxy=%d reject=%d\n",
			len(v.Rules.Direct), len(v.Rules.Proxy), len(v.Rules.Reject))
		return nil
	},
}

// ---- config source ----

var (
	srcType, srcURL, srcPath, srcServer, srcKind, srcUser, srcPass string
	srcPort                                                        int
)

var configSourceCmd = &cobra.Command{
	Use:   "source",
	Short: "设置代理源（--type subscription|file|external|remote|none）",
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		src := config.SourceConfig{Type: srcType}
		switch srcType {
		case config.SourceTypeSubscription:
			if srcURL == "" {
				return fmt.Errorf("--type subscription 需要 --url")
			}
			src.Subscription = config.SubscriptionSource{URL: srcURL, Name: "subscription"}
		case config.SourceTypeFile:
			if srcPath == "" {
				return fmt.Errorf("--type file 需要 --path")
			}
			src.File = config.FileSource{Path: srcPath}
		case config.SourceTypeExternal:
			src.External = config.ExternalProxy{Name: "外部代理", Server: srcServer, Port: srcPort, Kind: srcKind}
		case config.SourceTypeRemote:
			src.Remote = config.RemoteProxy{Name: "远程代理", Server: srcServer, Port: srcPort, Kind: srcKind, Username: srcUser, Password: srcPass}
		case config.SourceTypeNone, "":
			src.Type = config.SourceTypeNone
		default:
			return fmt.Errorf("不支持的源类型: %s", srcType)
		}
		if err := a.SetSource(context.Background(), src); err != nil {
			return err
		}
		fmt.Printf("✓ 代理源已设为 %s\n", src.Type)
		return nil
	},
}

// ---- config mode / tun / adblock / gateway-mode ----

var configModeCmd = &cobra.Command{
	Use:   "mode <rule|global|direct>",
	Short: "设置分流模式",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		if err := a.SetMode(context.Background(), args[0]); err != nil {
			return err
		}
		fmt.Printf("✓ 分流模式 = %s\n", args[0])
		return nil
	},
}

var configTUNCmd = &cobra.Command{
	Use:   "tun <on|off>",
	Short: "开关 TUN 透明代理",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		want, err := parseOnOff(args[0])
		if err != nil {
			return err
		}
		a, err := app.New()
		if err != nil {
			return err
		}
		if a.Status().TUN != want {
			if err := a.ToggleTUN(context.Background()); err != nil {
				return err
			}
		}
		fmt.Printf("✓ TUN = %v\n", want)
		return nil
	},
}

var configAdblockCmd = &cobra.Command{
	Use:   "adblock <on|off>",
	Short: "开关广告拦截",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		want, err := parseOnOff(args[0])
		if err != nil {
			return err
		}
		a, err := app.New()
		if err != nil {
			return err
		}
		if a.Status().Adblock != want {
			if err := a.ToggleAdblock(context.Background()); err != nil {
				return err
			}
		}
		fmt.Printf("✓ 去广告 = %v\n", want)
		return nil
	},
}

var configGatewayModeCmd = &cobra.Command{
	Use:   "gateway-mode <tun|forward>",
	Short: "设置网关模式（tun 透明 / forward 端口转发）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		if err := a.SetGatewayMode(context.Background(), args[0]); err != nil {
			return err
		}
		fmt.Printf("✓ 网关模式 = %s\n", args[0])
		return nil
	},
}

// ---- config rule add/list/rm ----

var configRuleCmd = &cobra.Command{Use: "rule", Short: "管理自定义分流规则"}

var configRuleAddCmd = &cobra.Command{
	Use:   "add <direct|proxy|reject> <rule>",
	Short: "新增一条自定义规则，如：add proxy DOMAIN-SUFFIX,openai.com",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		rule := strings.Join(args[1:], " ")
		if err := a.AddRule(context.Background(), args[0], rule); err != nil {
			return err
		}
		fmt.Printf("✓ 已加规则 [%s] %s\n", args[0], rule)
		return nil
	},
}

var configRuleListJSON bool

var configRuleListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出自定义规则（带索引，--json 机器可读）",
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		ex := a.Cfg.Traffic.Extras
		if configRuleListJSON {
			b, _ := json.MarshalIndent(map[string][]string{
				"direct": ex.Direct, "proxy": ex.Proxy, "reject": ex.Reject,
			}, "", "  ")
			fmt.Println(string(b))
			return nil
		}
		printRules := func(name string, rs []string) {
			fmt.Printf("[%s]\n", name)
			for i, r := range rs {
				fmt.Printf("  %d) %s\n", i, r)
			}
		}
		printRules("direct", ex.Direct)
		printRules("proxy", ex.Proxy)
		printRules("reject", ex.Reject)
		return nil
	},
}

var configRuleRmCmd = &cobra.Command{
	Use:   "rm <direct|proxy|reject> <index>",
	Short: "按索引删一条规则（索引来自 rule list）",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		idx, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("索引必须是数字: %s", args[1])
		}
		a, err := app.New()
		if err != nil {
			return err
		}
		if err := a.RemoveRule(context.Background(), args[0], idx); err != nil {
			return err
		}
		fmt.Printf("✓ 已删 [%s] 第 %d 条\n", args[0], idx)
		return nil
	},
}

func parseOnOff(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "on", "true", "1", "yes", "enable", "enabled":
		return true, nil
	case "off", "false", "0", "no", "disable", "disabled":
		return false, nil
	}
	return false, fmt.Errorf("无法识别的开关值 %q（用 on/off）", s)
}

func init() {
	configShowCmd.Flags().BoolVar(&configShowJSON, "json", false, "机器可读 JSON 输出")

	configSourceCmd.Flags().StringVar(&srcType, "type", "", "subscription|file|external|remote|none")
	configSourceCmd.Flags().StringVar(&srcURL, "url", "", "订阅 URL（type=subscription）")
	configSourceCmd.Flags().StringVar(&srcPath, "path", "", "本地 clash yaml 路径（type=file）")
	configSourceCmd.Flags().StringVar(&srcServer, "server", "", "主机（type=external/remote）")
	configSourceCmd.Flags().IntVar(&srcPort, "port", 0, "端口（type=external/remote）")
	configSourceCmd.Flags().StringVar(&srcKind, "kind", "http", "http|socks5")
	configSourceCmd.Flags().StringVar(&srcUser, "user", "", "用户名（type=remote，可选）")
	configSourceCmd.Flags().StringVar(&srcPass, "pass", "", "密码（type=remote，可选）")
	_ = configSourceCmd.MarkFlagRequired("type")

	configRuleListCmd.Flags().BoolVar(&configRuleListJSON, "json", false, "机器可读 JSON 输出")
	configRuleCmd.AddCommand(configRuleAddCmd, configRuleListCmd, configRuleRmCmd)

	configCmd.AddCommand(
		configShowCmd, configSourceCmd, configModeCmd,
		configTUNCmd, configAdblockCmd, configGatewayModeCmd, configRuleCmd,
	)
}
