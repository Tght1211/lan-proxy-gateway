package console

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// screenLifecycle performs the single lifecycle action advertised on the main
// screen: start when stopped, stop when running.
func (c *consoleUI) screenLifecycle(ctx context.Context) {
	c.banner("旁路由")
	if c.app.Running() {
		if err := c.app.Stop(); err != nil {
			badC.Fprintf(c.out, "  停止失败: %v\n", err)
		} else {
			okC.Fprintln(c.out, "  已停止旁路由")
		}
		c.pause()
		return
	}

	admin, _ := c.app.Plat.IsAdmin()
	if !admin {
		warnC.Fprintln(c.out, "  启动需要管理员权限：sudo gateway start")
		c.pause()
		return
	}
	if err := c.app.Start(ctx); err != nil {
		badC.Fprintf(c.out, "  启动失败: %v\n", err)
	} else {
		okC.Fprintln(c.out, "  已启动旁路由")
	}
	c.pause()
}

// screenLogs shows a bounded log snapshot. Continuous tailing belongs in a
// regular shell (`tail -f`) and is intentionally absent from the control UI.
func (c *consoleUI) screenLogs() {
	c.banner("最近日志")
	if err := c.renderTail(c.app.Paths.LogFile, 40); err != nil {
		warnC.Fprintf(c.out, "  %v\n", err)
	}
	dimC.Fprintf(c.out, "\n  完整日志: %s\n", c.app.Paths.LogFile)
	c.pause()
}

func (c *consoleUI) renderTail(path string, n int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("暂无日志")
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	start := 0
	if len(lines) > n {
		start = len(lines) - n
	}
	for _, line := range lines[start:] {
		fmt.Fprintln(c.out, line)
	}
	return nil
}

func (c *consoleUI) pause() {
	dimC.Fprint(c.out, "\n回车返回")
	_ = c.readLine()
}
