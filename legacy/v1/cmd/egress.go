package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/egress"
	"github.com/tght/lan-proxy-gateway/internal/mihomo"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

func printEgressReport(cfg *config.Config, dataDir string, client *mihomo.Client) {
	report := egress.Collect(cfg, dataDir, client)

	fmt.Println()
	ui.Separator()
	color.New(color.Bold).Println("  出口网络")
	ui.Separator()

	fmt.Printf("  %-14s %s\n", "探测来源:", report.ProbeSource)

	if cfg.Extension.Mode != "chains" {
		if report.ProxyExit != nil {
			fmt.Printf("  %-14s %s\n", "当前出口:", report.ProxyExit.Summary())
		} else {
			fmt.Printf("  %-14s %s\n", "当前出口:", color.YellowString("探测失败，请稍后运行 gateway status 重试"))
		}
		fmt.Println()
		return
	}

	chainMode := "rule"
	if cfg.Extension.ResidentialChain != nil && cfg.Extension.ResidentialChain.Mode != "" {
		chainMode = cfg.Extension.ResidentialChain.Mode
	}
	fmt.Printf("  %-14s %s\n", "链路模式:", chainMode)

	if report.AirportNode != nil {
		fmt.Printf("  %-14s %s\n", "入口节点:", report.AirportNode.Summary())
	} else {
		fmt.Printf("  %-14s %s\n", "入口节点:", color.YellowString("未识别当前机场节点"))
	}

	if chainMode == "rule" {
		if report.ProxyExit != nil {
			fmt.Printf("  %-14s %s\n", "普通出口:", report.ProxyExit.Summary())
		} else {
			fmt.Printf("  %-14s %s\n", "普通出口:", color.YellowString("探测失败"))
		}
	}

	if report.ResidentialExit != nil {
		label := "住宅出口:"
		if chainMode == "global" {
			label = "全局出口:"
		}
		fmt.Printf("  %-14s %s\n", label, report.ResidentialExit.Summary())
	} else {
		fmt.Printf("  %-14s %s\n", "住宅出口:", color.YellowString("探测失败"))
	}

	if chainMode == "rule" {
		fmt.Printf("  %-14s %s\n", "", color.New(color.Faint).Sprint("普通流量走机场出口，AI 相关流量走住宅出口"))
	} else {
		fmt.Printf("  %-14s %s\n", "", color.New(color.Faint).Sprint("当前为 global 模式，所有流量都会走住宅出口"))
	}
	fmt.Println()
}
