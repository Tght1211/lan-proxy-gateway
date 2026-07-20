package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/console"
	"github.com/tght/lan-proxy-gateway/internal/platform"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "一键安装：配置向导 + 启动 + 可选开机自启",
	RunE: func(cmd *cobra.Command, args []string) error {
		// install 会直接启动网关（绑定 53 端口、写防火墙规则），需要 root。
		// 先提权，这样用户只输一次 sudo 密码。
		maybeElevate()

		plat := platform.Current()

		// Step 1: 配置（走向导或加载已有）
		color.Cyan("[1/3] 配置旁路由网关…")
		a, err := app.New()
		if err != nil {
			return err
		}
		if a.Configured() {
			color.Green("  ✓ 已存在配置 %s （跳过向导，如需重配请删除后再来）", a.Paths.ConfigFile)
		} else {
			if err := console.RunOnboarding(cmd.Context(), a); err != nil {
				return err
			}
		}

		// Step 2: 启动
		color.Cyan("\n[2/3] 启动网关…")
		if err := a.Start(cmd.Context()); err != nil {
			color.Red("  ✗ 启动失败:")
			fmt.Println(indent("    ", fmt.Sprintf("%v", err)))
			color.Yellow("\n已进入主菜单 — 修好后在「旁路由启停」里重新启动。按 Q 随时退出。")
			fmt.Println()
			return console.Run(cmd.Context(), a)
		}
		color.Green("  ✓ 网关已启动")

		// Step 3: 开机自启（可选，默认 y）
		fmt.Println()
		color.Cyan("[3/3] 开机自启")
		fmt.Printf("  装上后，开机 / 重启会自动拉起网关，不用再手动 %s。\n", elevatedCmd(""))
		if askYesNo("  要装开机自启吗？", true) {
			if binPath, err := os.Executable(); err != nil {
				color.Yellow("  ⚠ 无法定位当前可执行文件: %v（可稍后手动 %s）", err, elevatedCmd("service install"))
			} else if err := plat.InstallService(binPath); err != nil {
				color.Yellow("  ⚠ 安装服务失败: %v（可稍后手动 %s）", err, elevatedCmd("service install"))
			} else {
				color.Green("  ✓ 开机自启已启用")
			}
		} else {
			color.New(color.Faint).Printf("  跳过。以后想装：%s\n", elevatedCmd("service install"))
		}

		fmt.Println()
		color.Cyan("设备接入指引：")
		printDeviceGuide(a)
		color.New(color.Faint).Printf("日志       %s\n", a.Paths.LogFile)
		color.New(color.Faint).Printf("打开控制面板 %-24s（旁路由、LAN 出口、系统代理、日志）\n", elevatedCmd(""))
		color.New(color.Faint).Printf("停止网关   %s\n", elevatedCmd("stop"))
		return nil
	},
}

// askYesNo 读一行 y/n，空回车走默认。失败 fallback 到默认（non-TTY 下也不卡）。
func askYesNo(label string, def bool) bool {
	hint := "(Y/n)"
	if !def {
		hint = "(y/N)"
	}
	fmt.Printf("%s %s ", label, hint)
	var line string
	if _, err := fmt.Scanln(&line); err != nil {
		return def
	}
	switch line {
	case "y", "Y", "yes", "YES":
		return true
	case "n", "N", "no", "NO":
		return false
	default:
		return def
	}
}

func indent(prefix, s string) string {
	out := ""
	for i, line := range splitLines(s) {
		if i > 0 {
			out += "\n"
		}
		out += prefix + line
	}
	return out
}

func splitLines(s string) []string {
	lines := []string{}
	cur := ""
	for _, r := range s {
		if r == '\n' {
			lines = append(lines, cur)
			cur = ""
		} else {
			cur += string(r)
		}
	}
	lines = append(lines, cur)
	return lines
}
