// ai_console.go 把主提示符上无法识别为命令的自然语言转给内置 AI 配网助手。
package console

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/aiagent"
	"github.com/tght/lan-proxy-gateway/internal/config"
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
	// 暂停让用户看清 AI 回复，再回到会自动清屏重绘的实时首页。
	dimC.Fprint(c.out, "\n按回车返回实时面板…")
	c.readLine()
}

// screenAIBackends 是 AI 配网助手的后端管理页：列出 / 切换 / 新增 / 删除 / 测试。
// 永不显示明文 key（只显示 key:***）。
func (c *consoleUI) screenAIBackends(ctx context.Context) {
	for {
		c.banner("AI 配网助手 · 后端")
		ai := c.app.Cfg.AI
		if !ai.Enabled {
			warnC.Fprintln(c.out, "  AI 助手当前已禁用（ai.enabled=false），仍可在此配置后端")
		}
		fmt.Fprintln(c.out, "  可用后端（★ = 当前使用）：")
		for i, b := range ai.Backends {
			star := "  "
			if b.ID == ai.Active {
				star = "★ "
			}
			tag := ""
			if b.Builtin {
				tag = " [内置免费]"
			}
			keyState := "无 key"
			if b.APIKey != "" {
				keyState = "key:***"
			}
			fmt.Fprintf(c.out, "  %s%d) %s  [%s] %s  (%s)%s\n", star, i+1, b.ID, b.Format, b.Model, keyState, tag)
		}
		fmt.Fprintln(c.out)
		dimC.Fprintln(c.out, "  数字=切换当前后端 · A 新增 · D <编号> 删除(非内置) · T 测试当前连通 · 0/Q 返回")
		fmt.Fprint(c.out, "> ")
		choice := strings.ToLower(strings.TrimSpace(c.readLine()))
		switch {
		case choice == "" || choice == "0" || choice == "q":
			return
		case choice == "a":
			c.addAIBackend()
		case choice == "t":
			c.testAIBackend(ctx)
		case strings.HasPrefix(choice, "d"):
			c.deleteAIBackend(strings.TrimSpace(strings.TrimPrefix(choice, "d")))
		default:
			n, err := strconv.Atoi(choice)
			if err != nil || n < 1 || n > len(ai.Backends) {
				warnC.Fprintln(c.out, "无效选项")
				continue
			}
			c.app.Cfg.AI.Active = ai.Backends[n-1].ID
			if err := c.app.Save(); err != nil {
				warnC.Fprintln(c.out, "保存失败: "+err.Error())
			} else {
				okC.Fprintf(c.out, "已切换当前后端为: %s\n", c.app.Cfg.AI.Active)
			}
		}
	}
}

// addAIBackend 交互式新增一个用户后端（openai / anthropic 格式）。
func (c *consoleUI) addAIBackend() {
	fmt.Fprint(c.out, "格式 (1=openai  2=anthropic): ")
	format := "openai"
	if f := strings.TrimSpace(c.readLine()); f == "2" || strings.EqualFold(f, "anthropic") {
		format = "anthropic"
	}
	hint := "（OpenAI 格式需含 /v1，如 https://api.openai.com/v1）"
	if format == "anthropic" {
		hint = "（如 https://api.anthropic.com）"
	}
	fmt.Fprint(c.out, "Base URL "+hint+": ")
	base := strings.TrimSpace(c.readLine())
	fmt.Fprint(c.out, "API Key: ")
	key := strings.TrimSpace(c.readLine())
	fmt.Fprint(c.out, "模型名 (如 gpt-4o / claude-opus-4-8): ")
	model := strings.TrimSpace(c.readLine())
	fmt.Fprint(c.out, "起个 id (英文短名，如 my-openai): ")
	id := strings.TrimSpace(c.readLine())
	if id == "" || base == "" || model == "" {
		warnC.Fprintln(c.out, "id / base_url / model 不能为空，已取消")
		return
	}
	for _, b := range c.app.Cfg.AI.Backends {
		if b.ID == id {
			warnC.Fprintln(c.out, "该 id 已存在，已取消")
			return
		}
	}
	c.app.Cfg.AI.Backends = append(c.app.Cfg.AI.Backends, config.AIBackend{
		ID: id, Format: format, BaseURL: base, APIKey: key, Model: model,
	})
	if err := c.app.Save(); err != nil {
		warnC.Fprintln(c.out, "保存失败: "+err.Error())
		return
	}
	okC.Fprintf(c.out, "已新增后端 %s（输入其编号可切为当前，T 测试连通）。\n", id)
}

// testAIBackend 给当前后端发一句话，验证连通。
func (c *consoleUI) testAIBackend(ctx context.Context) {
	b := c.app.Cfg.AI.ActiveBackend()
	llm, err := aiagent.NewClient(b)
	if err != nil {
		warnC.Fprintln(c.out, "构造客户端失败: "+err.Error())
		return
	}
	fmt.Fprintf(c.out, "正在测试后端 %s（%s）...\n", b.ID, b.Model)
	tctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	reply, err := llm.Chat(tctx, []aiagent.Message{{Role: "user", Content: "只回复两个字：在吗"}}, nil)
	if err != nil {
		badC.Fprintln(c.out, "✗ 测试失败: "+err.Error())
		return
	}
	okC.Fprintf(c.out, "✓ 连通正常，回复: %s\n", strings.TrimSpace(reply))
}

// deleteAIBackend 删除一个非内置后端。
func (c *consoleUI) deleteAIBackend(arg string) {
	n, err := strconv.Atoi(strings.TrimSpace(arg))
	if err != nil || n < 1 || n > len(c.app.Cfg.AI.Backends) {
		warnC.Fprintln(c.out, "无效编号（格式: D 2）")
		return
	}
	b := c.app.Cfg.AI.Backends[n-1]
	if b.Builtin {
		warnC.Fprintln(c.out, "内置免费后端不可删除")
		return
	}
	c.app.Cfg.AI.Backends = append(c.app.Cfg.AI.Backends[:n-1], c.app.Cfg.AI.Backends[n:]...)
	if c.app.Cfg.AI.Active == b.ID {
		c.app.Cfg.AI.Active = config.BuiltinAIBackendID
	}
	if err := c.app.Save(); err != nil {
		warnC.Fprintln(c.out, "保存失败: "+err.Error())
		return
	}
	okC.Fprintf(c.out, "已删除后端 %s（当前后端已回退内置免费）。\n", b.ID)
}
