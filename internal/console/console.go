// Package console is the numbered-menu interactive UI for lan-proxy-gateway.
//
// Design goals (per the v2 refactor):
//   - Low barrier. No arrow keys, no mouse. User types a number and presses Enter.
//     Works on every terminal, every SSH client, every Windows PowerShell.
//   - One path. Every action routes through internal/app; there is no duplicate
//     implementation in a cobra command.
//   - Three top-level screens matching the three feature layers:
//     1) 网关状态 & 设备接入  (gateway)
//     2) 流量控制               (traffic)
//     3) 代理端口                (source)
//   - First-time users go through a 3-step onboarding wizard before reaching the menu.
package console

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/devices"
	"github.com/tght/lan-proxy-gateway/internal/geoip"
)

// Run is the entry point. It blocks until the user exits.
func Run(ctx context.Context, a *app.App) error {
	c := newConsole(a, os.Stdin, os.Stdout)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if !a.Configured() {
		if err := c.onboard(runCtx); err != nil {
			return err
		}
	}
	// 拉起代理源健康 supervisor：mihomo 在跑时自动体检，挂了切 direct 保命。
	a.StartSupervisor(runCtx)
	return c.main(runCtx)
}

// RunOnboarding runs only the 3-step wizard (no main menu). Used by `install`
// which has its own post-wizard flow (predownload geodata, auto-start).
func RunOnboarding(ctx context.Context, a *app.App) error {
	c := newConsole(a, os.Stdin, os.Stdout)
	return c.onboard(ctx)
}

type consoleUI struct {
	app *app.App
	in  *bufio.Reader
	out io.Writer

	// 仪表盘相关：geo 打开失败时留 nil，Lookup 会安全返回空。
	// resolver 总是非 nil；DeviceLabels 变化时要调 SetLabels 同步。
	geo       *geoip.DB
	resolver  *devices.Resolver
	dashState dashboardState

	// 终端图表：网速柱状图（每帧 push 下行速率）+ 每分钟测速健康条。
	spark  *sparkline
	health *healthBar
}

func newConsole(a *app.App, in io.Reader, out io.Writer) *consoleUI {
	c := &consoleUI{
		app:    a,
		in:     bufio.NewReader(in),
		out:    out,
		spark:  newSparkline(40),
		health: newHealthBar(60),
	}
	// 打 country.mmdb。文件来自 EnsureGeodata；没下来就留 nil，Lookup 安全降级。
	if a != nil && a.Engine != nil {
		if db, err := geoip.Open(filepath.Join(a.Engine.Workdir(), "country.mmdb")); err == nil {
			c.geo = db
		}
	}
	var labels map[string]string
	if a != nil {
		labels = a.Cfg.Gateway.DeviceLabels
	}
	c.resolver = devices.NewResolver(labels)
	return c
}

// readLine 同步从 stdin 读一行。bubbletea 跑期间会接管 stdin，不要在 TUI
// 活着时调这个函数；进 simple screen（M 菜单、切节点 etc.）时 tea 已 Run 返
// 回，stdin 归还给我们。
func (c *consoleUI) readLine() string {
	line, _ := c.in.ReadString('\n')
	return strings.TrimSpace(line)
}

// --- Rendering helpers ---

var (
	titleC  = color.New(color.FgCyan, color.Bold)
	okC     = color.New(color.FgGreen, color.Bold)
	warnC   = color.New(color.FgYellow)
	dimC    = color.New(color.Faint)
	badC    = color.New(color.FgRed, color.Bold)
	speedC  = color.New(color.FgRed)   // 网速柱状图
	healthC = color.New(color.FgGreen) // 稳定性健康条
	bar     = "────────────────────────────────────────────────"
)

// clearScreen 把光标移到左上角并清屏，让实时首页原地重绘（btop / k9s 风格），
// 而不是每帧往下滚。只在自动刷新的首页用；滚动式子菜单不调它。
// ANSI \033[H 回 home，\033[2J 清屏，\033[3J 连 scrollback 一起清。
func (c *consoleUI) clearScreen() {
	fmt.Fprint(c.out, "\033[H\033[2J\033[3J")
}

func (c *consoleUI) banner(title string) {
	// 进任意屏前都先把磁盘上的 gateway.yaml 重读一遍：让"用户在外部（另一个终端 /
	// 手动编辑 gateway.yaml）改了东西之后切回菜单"立刻看到新值，避免 CLI 拿着内存里
	// 的旧 Cfg 显示陈旧数据，或更糟糕——基于旧值做 toggle 把外部刚改的设置回退掉。
	//
	// LoadFrom 失败时（罕见：被外部进程改坏 / 用户手动 rm）保留内存值，不影响菜单。
	if c.app != nil && c.app.Paths.ConfigFile != "" {
		if cfg, err := config.LoadFrom(c.app.Paths.ConfigFile); err == nil && cfg != nil {
			c.app.Cfg = cfg
		}
	}

	fmt.Fprintln(c.out)
	titleC.Fprintln(c.out, bar)
	titleC.Fprintf(c.out, "  %s\n", title)
	titleC.Fprintln(c.out, bar)
}

func (c *consoleUI) prompt(label string) string {
	fmt.Fprintf(c.out, "%s", label)
	return c.readLine()
}

func (c *consoleUI) ask(label, def string) string {
	if def != "" {
		dimC.Fprintf(c.out, "%s（回车=%s）: ", label, def)
	} else {
		fmt.Fprintf(c.out, "%s: ", label)
	}
	line := c.readLine()
	if line == "" {
		return def
	}
	return line
}

func (c *consoleUI) yesNo(label string, def bool) bool {
	hint := "(Y/n)"
	if !def {
		hint = "(y/N)"
	}
	for {
		line := strings.ToLower(c.prompt(fmt.Sprintf("%s %s ", label, hint)))
		switch line {
		case "":
			return def
		case "y", "yes":
			return true
		case "n", "no":
			return false
		}
		warnC.Fprintln(c.out, "请输入 y 或 n。")
	}
}
