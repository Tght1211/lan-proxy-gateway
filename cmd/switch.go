package cmd

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/mihomo"
	"github.com/tght/lan-proxy-gateway/internal/platform"
	"github.com/tght/lan-proxy-gateway/internal/proxy"
	tmpl "github.com/tght/lan-proxy-gateway/internal/template"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

var (
	switchDryRun      bool
	switchRegionCodes string
	switchGroupName   string
)

var switchCmd = &cobra.Command{
	Use:   "switch [url|file] [path]",
	Short: "切换代理来源或在限定地区内自动切换节点",
	Long: `切换代理来源模式，或按地区限制自动选择最佳节点。

用法:
  gateway switch                      # 查看当前模式
  gateway switch url                  # 切换到订阅链接模式
  gateway switch file                 # 切换到配置文件模式
  gateway switch file /path           # 切换并更新配置文件路径
  gateway switch best                 # 在限定地区内为 Auto 组选择最佳节点
  gateway switch best --dry-run       # 仅分析，不实际切换
  gateway switch best --region HK,JP  # 临时覆盖地区限制`,
	Args: cobra.MaximumNArgs(2),
	Run:  runSwitch,
}

func init() {
	rootCmd.AddCommand(switchCmd)
	switchCmd.Flags().BoolVar(&switchDryRun, "dry-run", false, "仅分析最佳节点，不实际切换")
	switchCmd.Flags().StringVar(&switchRegionCodes, "region", "", "临时指定地区代码，多个用逗号分隔，如 HK,JP")
	switchCmd.Flags().StringVar(&switchGroupName, "group", "Auto", "要切换的代理组名称")
}

type candidateNode struct {
	Name        string
	Alive       bool
	Delay       int
	MatchedCode []string
}

func runSwitch(cmd *cobra.Command, args []string) {
	cfgPath := resolveConfigPath()
	cfg := loadConfigOrDefault()

	if len(args) == 0 {
		printSwitchStatus(cfg)
		return
	}

	if args[0] == "best" {
		runBestSwitch(cfg)
		return
	}

	runSourceSwitch(cfgPath, cfg, args)
}

func printSwitchStatus(cfg *config.Config) {
	fmt.Println()
	fmt.Printf("  %s %s\n", color.New(color.Bold).Sprint("当前模式:"), color.CyanString(cfg.ProxySource))
	if cfg.ProxySource == "url" {
		url := cfg.SubscriptionURL
		if len(url) > 50 {
			url = url[:50] + "..."
		}
		fmt.Printf("  %s %s\n", color.New(color.Bold).Sprint("订阅链接:"), url)
	} else {
		fmt.Printf("  %s %s\n", color.New(color.Bold).Sprint("配置文件:"), cfg.ProxyConfigFile)
	}
	if cfg.Regions.Enabled {
		fmt.Printf("  %s %s\n", color.New(color.Bold).Sprint("地区限制:"), strings.Join(cfg.Regions.Include, ", "))
	}
	fmt.Println()
	fmt.Printf("  %s\n", color.New(color.Faint).Sprint("用法: gateway switch [url|file] [配置文件路径] | gateway switch best"))
	fmt.Println()
}

func runSourceSwitch(cfgPath string, cfg *config.Config, args []string) {
	target := args[0]
	if target != "url" && target != "file" {
		ui.Error("参数应为 url、file 或 best")
		os.Exit(1)
	}

	if target == "url" && cfg.SubscriptionURL == "" {
		ui.Error("未配置订阅链接，请先在 gateway.yaml 中设置 subscription_url")
		os.Exit(1)
	}

	if target == "file" {
		if len(args) >= 2 {
			path := args[1]
			if strings.HasPrefix(path, "~") {
				home, _ := os.UserHomeDir()
				path = filepath.Join(home, path[1:])
			}
			if _, err := os.Stat(path); os.IsNotExist(err) {
				ui.Error("文件不存在: %s", path)
				os.Exit(1)
			}
			count, err := proxy.ExtractProxies(path, os.DevNull)
			if err != nil {
				ui.Error("%s", err)
				os.Exit(1)
			}
			ui.Info("检测到 %d 个代理节点", count)
			cfg.ProxyConfigFile = path
		}
		if cfg.ProxyConfigFile == "" {
			ui.Error("未配置文件路径")
			fmt.Println("  用法: gateway switch file /path/to/config.yaml")
			os.Exit(1)
		}
	}

	oldSource := cfg.ProxySource
	cfg.ProxySource = target

	if cfgPath == ".secret" {
		cfgPath = "gateway.yaml"
	}
	if err := config.Save(cfg, cfgPath); err != nil {
		ui.Error("保存配置失败: %s", err)
		os.Exit(1)
	}

	if oldSource == target {
		ui.Info("当前已是 %s 模式，配置已更新", target)
	} else {
		ui.Success("已切换: %s → %s", oldSource, target)
	}

	fmt.Println()
	fmt.Print("是否立即重新生成配置？[Y/n] ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)

	if answer == "" || strings.ToLower(answer) == "y" {
		p := platform.New()
		dDir := ensureDataDir()
		iface, _ := p.DetectDefaultInterface()
		ip, _ := p.DetectInterfaceIP(iface)

		if cfg.ProxySource == "file" {
			providerFile := filepath.Join(dDir, "proxy_provider", cfg.SubscriptionName+".yaml")
			count, err := proxy.ExtractProxies(cfg.ProxyConfigFile, providerFile)
			if err != nil {
				ui.Error("提取代理节点失败: %s", err)
				os.Exit(1)
			}
			ui.Success("已提取 %d 个代理节点", count)
		}

		configPath := filepath.Join(dDir, "config.yaml")
		if err := tmpl.RenderTemplate(cfg, iface, ip, configPath); err != nil {
			ui.Error("配置生成失败: %s", err)
			os.Exit(1)
		}
		ui.Success("配置文件已生成")
		fmt.Println()
		ui.Info("如需生效，请重启网关: sudo gateway start")
	}
}

func runBestSwitch(cfg *config.Config) {
	regions := resolveSwitchRegions(cfg)
	if len(regions) == 0 {
		ui.Error("未启用地区限制，请先在 gateway.yaml 中配置 regions，或使用 --region 临时指定")
		os.Exit(1)
	}

	apiURL := mihomo.FormatAPIURL("127.0.0.1", cfg.Ports.API)
	client := mihomo.NewClient(apiURL, cfg.APISecret)
	if !client.IsAvailable() {
		ui.Error("mihomo API 不可用: %s", apiURL)
		os.Exit(1)
	}

	best, candidateCount, err := analyzeBestCandidate(client, cfg, switchGroupName, regions)
	if err != nil {
		ui.Error("分析节点失败: %s", err)
		os.Exit(1)
	}

	fmt.Println()
	ui.Separator()
	color.New(color.Bold).Println("  地区节点分析")
	ui.Separator()
	fmt.Printf("  目标代理组:  %s\n", switchGroupName)
	fmt.Printf("  限定地区:    %s\n", strings.Join(regions, ", "))
	fmt.Printf("  候选数量:    %d\n", candidateCount)
	fmt.Printf("  最优节点:    %s\n", color.CyanString(best.Name))
	if best.Delay == math.MaxInt {
		fmt.Printf("  预计延迟:    未知\n")
	} else {
		fmt.Printf("  预计延迟:    %d ms\n", best.Delay)
	}
	fmt.Printf("  命中地区:    %s\n", strings.Join(best.MatchedCode, ", "))
	fmt.Println()

	if switchDryRun {
		ui.Info("dry-run 模式，未执行切换")
		return
	}

	if err := client.SetProxyGroup(switchGroupName, best.Name); err != nil {
		ui.Error("切换节点失败: %s", err)
		os.Exit(1)
	}
	ui.Success("已切换 %s -> %s", switchGroupName, best.Name)
}

func resolveSwitchRegions(cfg *config.Config) []string {
	if switchRegionCodes == "" {
		return append([]string{}, cfg.Regions.Include...)
	}

	result := make([]string, 0)
	seen := make(map[string]struct{})
	for _, item := range strings.Split(strings.ToUpper(switchRegionCodes), ",") {
		code := strings.TrimSpace(item)
		if code == "" {
			continue
		}
		if _, ok := cfg.Regions.Mapping[code]; !ok {
			ui.Warn("忽略未知地区: %s", code)
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		result = append(result, code)
	}
	return result
}

func collectCandidates(client *mihomo.Client, cfg *config.Config, names, regions []string) ([]candidateNode, error) {
	candidates := make([]candidateNode, 0)
	for _, name := range names {
		if name == "DIRECT" || name == "REJECT" || strings.TrimSpace(name) == "" {
			continue
		}
		matched := matchRegions(name, cfg.Regions.Mapping, regions)
		if len(matched) == 0 {
			continue
		}
		node, err := client.GetProxyNode(name)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, candidateNode{
			Name:        name,
			Alive:       node.Alive,
			Delay:       latestDelay(node.History),
			MatchedCode: matched,
		})
	}
	return candidates, nil
}

func matchRegions(name string, mapping map[string][]string, regions []string) []string {
	matched := make([]string, 0)
	lowerName := strings.ToLower(name)
	for _, code := range regions {
		for _, keyword := range mapping[code] {
			if strings.Contains(lowerName, strings.ToLower(keyword)) {
				matched = append(matched, code)
				break
			}
		}
	}
	return matched
}

func latestDelay(history []mihomo.DelayHistory) int {
	best := math.MaxInt
	for _, item := range history {
		if item.Delay > 0 && item.Delay < best {
			best = item.Delay
		}
	}
	return best
}

func selectBestCandidate(candidates []candidateNode) candidateNode {
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Alive != candidates[j].Alive {
			return candidates[i].Alive
		}
		return candidates[i].Delay < candidates[j].Delay
	})
	return candidates[0]
}

func analyzeBestCandidate(client *mihomo.Client, cfg *config.Config, groupName string, regions []string) (candidateNode, int, error) {
	group, err := client.GetProxyGroup(groupName)
	if err != nil {
		return candidateNode{}, 0, err
	}

	candidates, err := collectCandidates(client, cfg, group.All, regions)
	if err != nil {
		return candidateNode{}, 0, err
	}
	if len(candidates) == 0 {
		return candidateNode{}, 0, fmt.Errorf("限定地区 %s 内没有可用节点", strings.Join(regions, ", "))
	}

	best := selectBestCandidate(candidates)
	return best, len(candidates), nil
}
