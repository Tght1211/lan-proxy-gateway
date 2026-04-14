package cmd

import "strings"

func renderSimpleHelpLines(showAll bool) []string {
	lines := []string{
		renderSectionTitle("日常最常用"),
		"  nodes              打开节点工作台；展示延时并支持 T 重新测速",
		"  proxy              查看/切换代理来源（订阅/文件/直接代理）",
		"  direct             打开直接代理工作台（无订阅场景用）",
		"  subscription       打开订阅工作台；可更新订阅 / 切换当前订阅",
		"  extension          打开扩展模式工作台；可直接切到 chains",
		"",
		renderSectionTitle("工作台提示"),
		"  打开 proxy / direct / subscription / extension 后，",
		"  直接输入面板里的 1 / 2 / S / O / T ... 就能改，不用手敲整条命令",
		"",
		renderSectionTitle("补充常用"),
		"  status / summary   查看状态和配置摘要",
		"  tui                切进 TUI 工作台",
		"  logs / update      看日志 / 升级提示",
	}

	if showAll {
		lines = append(lines,
			"",
			renderSectionTitle("完整命令"),
			"  status / summary / config / device / logs / guide / update",
			"  nodes / tui / restart / stop / exit",
			"",
			renderSectionTitle("代理来源"),
			"  proxy source url|file|proxy     切换代理来源模式",
			"  proxy url <链接>                设置订阅链接",
			"  proxy file <路径>               设置本地配置文件",
			"  direct server|port|type|user|password|name <值>",
			"  subscription add url|file <名称> <链接或路径>",
			"  subscription use <名称>",
			"",
			renderSectionTitle("运行模式与规则"),
			"  tun on|off",
			"  bypass on|off",
			"  rule <lan|china|apple|nintendo|global|ads> [on|off|toggle]",
			"",
			renderSectionTitle("扩展"),
			"  extension chains|script|off",
			"  chain mode rule|global",
			"  chain server|port|type|airport|user|password <值>",
			"  chains            查看链式代理 / 扩展状态",
			"  chains setup      打开链式代理向导",
			"",
			noteLine("默认 help 只保留日常高频操作；低频命令放在 help all 里。"),
		)
	} else {
		lines = append(lines,
			"",
			noteLine("输入 help all 查看完整命令清单。"),
		)
	}

	return lines
}

func handleSimpleHelpCommand(raw string) bool {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return false
	}

	switch strings.ToLower(fields[0]) {
	case "help", "?":
		showAll := len(fields) > 1 && strings.EqualFold(fields[1], "all")
		printSimpleDetail("命令帮助", renderSimpleHelpLines(showAll))
		return true
	default:
		return false
	}
}
