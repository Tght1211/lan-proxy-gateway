// Package console is the compact interactive control panel.
package console

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/config"
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
	return c.main(runCtx)
}

// RunOnboarding runs only the wizard (no main menu). Used by `install`.
func RunOnboarding(ctx context.Context, a *app.App) error {
	c := newConsole(a, os.Stdin, os.Stdout)
	return c.onboard(ctx)
}

type consoleUI struct {
	app *app.App
	in  *bufio.Reader
	out io.Writer
}

func newConsole(a *app.App, in io.Reader, out io.Writer) *consoleUI {
	return &consoleUI{
		app: a,
		in:  bufio.NewReader(in),
		out: out,
	}
}

// readLine 同步从 stdin 读一行。
func (c *consoleUI) readLine() string {
	line, _ := c.in.ReadString('\n')
	return strings.TrimSpace(line)
}

// --- Rendering helpers ---

var (
	titleC = color.New(color.FgCyan, color.Bold)
	okC    = color.New(color.FgGreen, color.Bold)
	warnC  = color.New(color.FgYellow)
	dimC   = color.New(color.Faint)
	badC   = color.New(color.FgRed, color.Bold)
	bar    = "────────────────────────────────────────────────"
)

// clearScreen clears the terminal before drawing a control screen.
func (c *consoleUI) clearScreen() {
	fmt.Fprint(c.out, "\033[H\033[2J\033[3J")
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func (c *consoleUI) banner(title string) {
	// 进任意屏前都把磁盘上的 gateway.yaml 重读一遍：外部（另一个终端 /
	// system-proxy 命令 / 守护进程）改了配置后，菜单立刻看到新值。
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
