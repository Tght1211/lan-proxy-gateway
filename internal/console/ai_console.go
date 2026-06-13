// ai_console.go 把主提示符上无法识别为命令的自然语言转给内置 AI 配网助手。
package console

import (
	"context"
	"fmt"
	"strings"

	"github.com/tght/lan-proxy-gateway/internal/aiagent"
)

// aiAvailable 报告 AI 是否启用且能构造客户端。
func (c *consoleUI) aiAvailable() bool {
	return c.app != nil && c.app.Cfg.AI.Enabled && len(c.app.Cfg.AI.Backends) > 0
}

// handleNaturalLanguage 把一句话交给 AI 配网助手跑一轮。
func (c *consoleUI) handleNaturalLanguage(ctx context.Context, line string) {
	if !c.aiAvailable() {
		warnC.Fprintln(c.out, "无效选项（回车 / R 刷新 · M 菜单 · N 切节点 · T 重测 · Q 退出）")
		return
	}
	backend := c.app.Cfg.AI.ActiveBackend()
	llm, err := aiagent.NewClient(backend)
	if err != nil {
		warnC.Fprintln(c.out, "AI 后端不可用: "+err.Error())
		return
	}
	confirm := func(plan string) bool {
		fmt.Fprintln(c.out, "\nAI 准备执行：")
		fmt.Fprintln(c.out, "  "+plan)
		fmt.Fprint(c.out, "确认? [y/N] ")
		ans := strings.ToLower(strings.TrimSpace(c.readLine()))
		return ans == "y" || ans == "yes"
	}
	exec := aiagent.NewExecutor(c.app, confirm)
	sess := aiagent.NewSession(llm, exec)

	fmt.Fprintln(c.out, "\n🤖 ")
	_, err = sess.Handle(ctx, line, func(s string) { fmt.Fprint(c.out, s) })
	fmt.Fprintln(c.out)
	if err != nil {
		warnC.Fprintln(c.out, err.Error())
	}
}
