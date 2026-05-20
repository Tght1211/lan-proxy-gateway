package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/webui"
)

var startForeground bool

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "启动网关",
	Long: `启动网关（默认后台运行）。

默认: 起 mihomo 后立即返回 shell，mihomo 作为孤儿进程在后台跑；
      之后运行 gateway 进主菜单，或 gateway stop 停止
      （Linux/macOS 需 sudo 前缀）。
--foreground: 阻塞当前终端直到 Ctrl+C 再 stop，给 launchd / systemd 用。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		maybeElevate()
		a, err := app.New()
		if err != nil {
			return err
		}
		if !a.Configured() {
			return fmt.Errorf("尚未完成初始化，请先运行 `gateway install` 或直接运行 `gateway` 进入向导")
		}
		if err := a.Start(cmd.Context()); err != nil {
			return err
		}
		a.StartSupervisor(cmd.Context())
		color.Green("✔ 网关已启动")
		color.New(color.Faint).Println(a.Engine.LogPath())
		app.InjectWebUIVersion(Version)

		if !startForeground {
			if err := startWebUIDaemon(a); err != nil {
				color.Yellow("Web 控制台后台启动失败（不影响代理本体）：%v", err)
			} else {
				printWebUIBanner(a)
			}
			color.New(color.Faint).Printf("\nmihomo 已在后台运行；CLI 菜单 %s，停止 %s。\n",
				elevatedCmd(""), elevatedCmd("stop"))
			return nil
		}

		// Web 控制台：监听 runtime.ports.web_ui（默认 19091），失败只 warn 不阻塞主流程。
		// foreground 模式下 server 在 Ctrl+C 后跟着 a.Stop 一起优雅关闭。
		webuiSrv := webui.New(
			webui.PortFromInt(a.Cfg.Runtime.Ports.WebUI),
			a.Cfg.Runtime.WebUIToken,
			app.NewWebUIController(a),
		)
		webuiOK := true
		if err := webuiSrv.Start(cmd.Context(), func(format string, args ...any) {
			color.New(color.Faint).Printf(format+"\n", args...)
		}); err != nil {
			color.Yellow("Web 控制台启动失败（不影响代理本体）：%v", err)
			webuiOK = false
		}
		if webuiOK {
			printWebUIBanner(a)
		}

		// --foreground: launchd / systemd 要前台进程，等 Ctrl+C 再优雅停止。
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		color.Yellow("正在停止…")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = webuiSrv.Shutdown(shutdownCtx)
		return a.Stop()
	},
}

func init() {
	startCmd.Flags().BoolVar(&startForeground, "foreground", false, "前台阻塞直到 Ctrl+C（给 launchd / systemd 用）")
}

// printWebUIBanner 在启动尾巴上印一个醒目横幅，把 WebUI URL 用 LAN IP 显式
// 推给用户。目的：用户跑完 `gateway start` 之后**不用再问"在哪改设置"** ——
// 直接告诉他打开浏览器到 http://<本机IP>:19091 即可全功能图形化操作；
// CLI 党仍然可以另开终端跑 `gateway` 进菜单。
//
// 用 Gateway.Detect() 取真正的 LAN IP；探测失败时退化成 localhost 以保证可访问。
func printWebUIBanner(a *app.App) {
	port := a.Cfg.Runtime.Ports.WebUI
	if port <= 0 {
		return
	}
	lanIP := ""
	if a.Gateway != nil {
		if err := a.Gateway.Detect(); err == nil {
			lanIP = a.Gateway.Info().IP
		}
	}
	mihomoPort := a.Cfg.Runtime.Ports.API
	// token 通过 URL fragment (#token=...) 传给前端：fragment 不会进 HTTP 请求行、
	// 不会进 access log、也不会被 referrer 泄漏；前端读完一次后立刻塞 sessionStorage
	// 并 history.replaceState 把 fragment 抹掉。
	token := a.Cfg.Runtime.WebUIToken
	frag := ""
	if token != "" {
		frag = "#token=" + token
	}

	bold := color.New(color.Bold)
	accent := color.New(color.FgHiCyan, color.Bold)
	faint := color.New(color.Faint)
	warn := color.New(color.FgYellow)

	fmt.Println()
	bold.Println("  ┌─ Web 控制台 ───────────────────────────────────────")
	if lanIP != "" {
		bold.Printf("  │ ")
		accent.Printf("http://%s:%d/%s\n", lanIP, port, frag)
		bold.Printf("  │ ")
		faint.Printf("局域网任意设备浏览器打开即可全功能操作\n")
	}
	bold.Printf("  │ ")
	accent.Printf("http://localhost:%d/%s\n", port, frag)
	bold.Printf("  │ ")
	faint.Printf("本机访问\n")
	bold.Println("  ├────────────────────────────────────────────────────")
	bold.Printf("  │ ")
	warn.Printf("⚠ URL 含 token，浏览器收藏后任何人拿到都能控制本机；")
	faint.Printf("\n  │   token 在 gateway.yaml runtime.web_ui_token 可改\n")
	bold.Println("  ├────────────────────────────────────────────────────")
	bold.Printf("  │ ")
	faint.Printf("Mihomo 完整控制台 (yacd) ")
	if lanIP != "" {
		fmt.Printf("http://%s:%d/ui/\n", lanIP, mihomoPort)
	} else {
		fmt.Printf("http://localhost:%d/ui/\n", mihomoPort)
	}
	bold.Printf("  │ ")
	faint.Printf("CLI 菜单（双开 / 不喜欢 Web 的用户）%s\n", elevatedCmd(""))
	bold.Println("  └────────────────────────────────────────────────────")
}
