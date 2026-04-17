package cmd

import (
	"bufio"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/mihomo"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

const (
	simpleNodeDelayTimeout     = 4 * time.Second
	simpleNodeDelayConcurrency = 6
)

type simpleNodeDelayEntry struct {
	Name       string
	Delay      int
	DelayLabel string
	Current    bool
	Reachable  bool
}

type simpleNodeMeasureSummary struct {
	Total     int
	Success   int
	Failure   int
	AllFailed bool
}

func runSimpleNodeChooser(reader *bufio.Reader, client *mihomo.Client, group mihomo.ProxyGroup, cfg *config.Config) {
	nodes := simpleSelectableNodes(group.All)
	if len(nodes) == 0 {
		ui.Info("当前分组没有可切换的节点")
		fmt.Println()
		return
	}

	results, testedAt := refreshSimpleNodeDelayEntries(client, group, nodes)
	consecutiveAllFailed := 0
	if buildSimpleNodeMeasureSummary(results).AllFailed {
		consecutiveAllFailed = 1
	}

	for {
		printSimpleNodeChooser(group, results, testedAt, cfg, consecutiveAllFailed)
		fmt.Printf("选择节点 [1-%d]，输入 T 重测，回车取消: ", len(results))

		rawNode, _ := reader.ReadString('\n')
		rawNode = strings.TrimSpace(rawNode)
		fmt.Println()

		if rawNode == "" {
			return
		}

		switch strings.ToLower(rawNode) {
		case "t", "r", "test", "retest", "测速":
			results, testedAt = refreshSimpleNodeDelayEntries(client, group, nodes)
			if buildSimpleNodeMeasureSummary(results).AllFailed {
				consecutiveAllFailed++
			} else {
				consecutiveAllFailed = 0
			}
			continue
		}

		nodeIndex := parseIndex(rawNode, len(results))
		if nodeIndex < 0 {
			ui.Warn("无效的节点编号")
			fmt.Println()
			continue
		}

		target := results[nodeIndex].Name
		if err := client.SelectProxy(group.Name, target); err != nil {
			ui.Error("切换失败: %v", err)
		} else {
			ui.Success("已切换节点: %s -> %s", group.Name, target)
		}
		fmt.Println()
		return
	}
}

func simpleSelectableNodes(nodes []string) []string {
	filtered := make([]string, 0, len(nodes))
	fallback := make([]string, 0, len(nodes))
	seen := make(map[string]struct{}, len(nodes))

	for _, raw := range nodes {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		fallback = append(fallback, name)
		if _, _, ok := parseSubscriptionHintItem(name); ok {
			continue
		}
		filtered = append(filtered, name)
	}

	if len(filtered) == 0 {
		return fallback
	}
	return filtered
}

func refreshSimpleNodeDelayEntries(client *mihomo.Client, group mihomo.ProxyGroup, nodes []string) ([]simpleNodeDelayEntry, time.Time) {
	fmt.Printf("  正在测速 %d 个节点，完成后会按低延时优先排序...\n\n", len(nodes))
	results := measureSimpleNodeDelayEntries(client, group, nodes)
	sortSimpleNodeDelayEntries(results)
	return results, time.Now()
}

func measureSimpleNodeDelayEntries(client *mihomo.Client, group mihomo.ProxyGroup, nodes []string) []simpleNodeDelayEntry {
	results := make([]simpleNodeDelayEntry, len(nodes))
	for i, node := range nodes {
		results[i] = simpleNodeDelayEntry{
			Name:       node,
			Delay:      -1,
			DelayLabel: "未测速",
			Current:    node == group.Now,
		}
	}
	if client == nil || len(nodes) == 0 {
		return results
	}

	testURL := pickerTestURL(group)
	concurrency := min(len(nodes), simpleNodeDelayConcurrency)
	if concurrency <= 0 {
		concurrency = 1
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	for i, node := range nodes {
		wg.Add(1)
		go func(i int, node string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			delay, err := client.GetProxyDelay(node, testURL, simpleNodeDelayTimeout)
			if err != nil {
				results[i].DelayLabel = renderSimpleNodeDelayError(err)
				return
			}
			results[i].Delay = delay
			results[i].DelayLabel = fmt.Sprintf("%dms", delay)
			results[i].Reachable = true
		}(i, node)
	}
	wg.Wait()
	return results
}

func pickerTestURL(group mihomo.ProxyGroup) string {
	if strings.TrimSpace(group.TestURL) != "" {
		return group.TestURL
	}
	return "http://www.gstatic.com/generate_204"
}

func renderSimpleNodeDelayError(err error) string {
	if err == nil {
		return "未测速"
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(message, "timeout") || strings.Contains(message, "timed out") || strings.Contains(message, "deadline exceeded") {
		return "超时"
	}
	return "失败"
}

func buildSimpleNodeMeasureSummary(entries []simpleNodeDelayEntry) simpleNodeMeasureSummary {
	summary := simpleNodeMeasureSummary{Total: len(entries)}
	for _, item := range entries {
		if item.Reachable {
			summary.Success++
		}
	}
	summary.Failure = summary.Total - summary.Success
	summary.AllFailed = summary.Total > 0 && summary.Success == 0
	return summary
}

func detectSimpleNodeSourceKind(cfg *config.Config) string {
	return detectActiveSubscriptionSourceKind(cfg)
}

func simpleNodeProviderSourceHost(cfg *config.Config) string {
	if cfg == nil {
		return "未设置"
	}
	value := currentSubscriptionURL(cfg)
	if value == "" {
		return providerSourceHost(cfg)
	}
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

func buildSimpleNodeConnectivityHint(cfg *config.Config, summary simpleNodeMeasureSummary, consecutiveAllFailed int) string {
	if summary.Total == 0 || !summary.AllFailed || consecutiveAllFailed < 2 {
		return ""
	}
	switch detectSimpleNodeSourceKind(cfg) {
	case "url":
		host := simpleNodeProviderSourceHost(cfg)
		if host != "" && host != "未设置" {
			return "提示：当前来源为订阅链接（" + host + "），节点测速连续全部失败，订阅链接可能异常或不可访问；也可能是当前网络环境或测速地址异常。"
		}
		return "提示：当前来源为订阅链接，节点测速连续全部失败，订阅链接可能异常或不可访问；也可能是当前网络环境或测速地址异常。"
	case "file":
		return "提示：当前来源为本地代理文件，节点测速连续全部失败，订阅文件可能无效或节点不可用；也可能是当前网络环境或测速地址异常。"
	default:
		return ""
	}
}

func sortSimpleNodeDelayEntries(entries []simpleNodeDelayEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		left := entries[i]
		right := entries[j]
		if left.Reachable != right.Reachable {
			return left.Reachable
		}
		if left.Reachable && right.Reachable && left.Delay != right.Delay {
			return left.Delay < right.Delay
		}
		if left.Current != right.Current {
			return left.Current
		}
		return left.Name < right.Name
	})
}

func printSimpleNodeChooser(group mihomo.ProxyGroup, results []simpleNodeDelayEntry, testedAt time.Time, cfg *config.Config, consecutiveAllFailed int) {
	ui.Separator()
	color.New(color.Bold).Printf("  节点列表 · %s\n", group.Name)
	ui.Separator()

	summary := buildSimpleNodeMeasureSummary(results)

	fmt.Printf("  当前节点: %s\n", fallbackText(strings.TrimSpace(group.Now), "未识别"))
	fmt.Printf("  测试地址: %s\n", pickerTestURL(group))
	fmt.Printf("  最近测速: %s\n", testedAt.Format("15:04:05"))
	fmt.Printf("  排序方式: 低延时优先，失败放最后\n")
	fmt.Printf("  测速结果: 成功 %d / 失败 %d\n", summary.Success, summary.Failure)
	if hint := buildSimpleNodeConnectivityHint(cfg, summary, consecutiveAllFailed); hint != "" {
		fmt.Printf("  %s\n", color.YellowString(hint))
	}
	fmt.Println()

	for i, item := range results {
		name := item.Name
		if item.Current {
			name += " (current)"
		}
		fmt.Printf("  %d) %s  ·  %s\n", i+1, name, item.DelayLabel)
	}

	fmt.Println()
	fmt.Println("  输入节点编号切换，输入 T 重新测速并排序，回车返回。")
	fmt.Println()
}
