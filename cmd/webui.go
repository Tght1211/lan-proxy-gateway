package cmd

import (
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/tght/lan-proxy-gateway/internal/app"
)

var webuiOpen bool

// webuiCmd 让用户在已经跑着 mihomo 的情况下查 / 打开 Web 控制台 URL。
// 跟主菜单 / start banner 共享同一份地址来源（gateway.yaml 的 runtime.ports.web_ui
// + 实时探测的 LAN IP），保证三处显示一致。
//
// 设计取舍：
//   - 不要求 sudo（只读配置 + 探测网络，不改任何系统状态）
//   - 不验证 webui 是否真的在监听 —— 用户可能在另一台终端跑 `gateway start`，
//     这边只需要告诉他正确的 URL 即可。
//   - `--open` 调用本地 open/xdg-open/start 拉起默认浏览器，给"懒得复制 URL"的用户。
var webuiCmd = &cobra.Command{
	Use:   "webui",
	Short: "打印 Web 控制台 URL（或用 --open 直接打开浏览器）",
	Long: `打印当前 gateway 的 Web 控制台 URL。

适合场景：
  · 不记得端口 / 局域网 IP 时随手查
  · 已经 gateway start 在后台跑，这台终端想再开一下 WebUI

不需要 sudo，只读 gateway.yaml；--open 自动调用本地浏览器。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		port := a.Cfg.Runtime.Ports.WebUI
		if port <= 0 {
			return fmt.Errorf("gateway.yaml 里 runtime.ports.web_ui 为 0（已禁用）；改成 19091 后重启服务")
		}

		// 拿 LAN IP；失败就退化成 localhost。
		lanIP := ""
		if a.Gateway != nil {
			if err := a.Gateway.Detect(); err == nil {
				lanIP = a.Gateway.Info().IP
			}
		}

		token := a.Cfg.Runtime.WebUIToken
		frag := ""
		if token != "" {
			frag = "#token=" + token
		}
		urls := []string{fmt.Sprintf("http://localhost:%d/%s", port, frag)}
		if lanIP != "" {
			urls = append([]string{fmt.Sprintf("http://%s:%d/%s", lanIP, port, frag)}, urls...)
		}

		bold := color.New(color.Bold)
		accent := color.New(color.FgHiCyan, color.Bold)
		faint := color.New(color.Faint)

		bold.Println("Web 控制台地址：")
		for _, u := range urls {
			fmt.Print("  ")
			accent.Println(u)
		}

		// 顺手探一下，给用户一个"是否在跑"的提示，但不强求。
		if probeURL(urls[len(urls)-1]) {
			faint.Println("  · 服务已运行")
		} else {
			color.Yellow("  · 似乎尚未启动；请先 %s 起服务", elevatedCmd("start"))
		}

		if webuiOpen {
			target := urls[0]
			fmt.Println()
			faint.Printf("打开浏览器：%s\n", target)
			if err := openBrowser(target); err != nil {
				color.Yellow("自动打开失败：%v（请手动复制 URL）", err)
			}
		}
		return nil
	},
}

func init() {
	webuiCmd.Flags().BoolVarP(&webuiOpen, "open", "o", false, "顺便用默认浏览器打开第一个 URL")
	rootCmd.AddCommand(webuiCmd)
	rootCmd.AddCommand(webuiServeCmd)
}

// probeURL 一次 GET /api/ping，不接收返回体，纯探活。失败返回 false 不抛错。
func probeURL(base string) bool {
	u, err := url.Parse(base)
	if err != nil {
		return false
	}
	u.Path = "/api/ping"
	u.RawQuery = ""
	u.Fragment = ""

	cli := &http.Client{Timeout: 600 * time.Millisecond}
	resp, err := cli.Get(u.String())
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// openBrowser 跨平台调用系统默认浏览器；失败时把错误透出来交给上层日志。
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		// rundll32 比 start 在 cobra/MSYS 环境下更稳，避开 cmd.exe 注入风险
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default: // linux 等
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
