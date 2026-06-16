// slash.go 实现首页的「斜杠命令」：输入 / 列出所有命令+描述（Claude Code 风格），
// 输入 /status、/node 等直接执行。非斜杠的普通文字则进 AI 连续对话。
package console

import (
	"context"
	"fmt"
	"strings"
)

// slashCmd 是一个斜杠命令的元信息（名字 + 一行描述），用于 / 清单展示。
type slashCmd struct {
	name string
	desc string
}

// slashCommands 是首页可用的斜杠命令清单。
var slashCommands = []slashCmd{
	{"/chat", "和 AI 配网助手连续对话（直接打字也行）"},
	{"/status", "查看运行状态（模式 / TUN / 源 / 端口…）"},
	{"/node", "切换代理节点"},
	{"/source", "代理源 / 订阅 · 连通测试"},
	{"/menu", "完整菜单：设备接入 · 分流规则 · 启停 · 看日志"},
	{"/ai", "AI 后端：切换 / 新增 / 测连通"},
	{"/help", "显示这个命令清单"},
	{"/quit", "退出控制台（网关留后台继续跑）"},
}

// printSlashHelp 打印斜杠命令清单 + 描述。
func (c *consoleUI) printSlashHelp() {
	c.banner("可用命令")
	for _, s := range slashCommands {
		titleC.Fprintf(c.out, "  %-9s", s.name)
		dimC.Fprintf(c.out, "  %s\n", s.desc)
	}
	fmt.Fprintln(c.out)
	dimC.Fprintln(c.out, "  提示：直接打字（不带 /）= 和 AI 助手连续对话。")
}

// handleSlash 分发斜杠命令。返回 true 表示用户要退出控制台。
func (c *consoleUI) handleSlash(ctx context.Context, raw string) (quit bool) {
	cmd := strings.ToLower(strings.Fields(raw)[0])
	switch cmd {
	case "/", "/help", "/?", "/h":
		c.printSlashHelp()
		c.pause()
	case "/status", "/s":
		c.banner("运行状态")
		c.printStatus()
		c.pause()
	case "/chat", "/talk", "/ai-chat":
		c.screenAIChat(ctx, "")
	case "/ai", "/backend", "/backends":
		c.screenAIBackends(ctx)
	case "/menu", "/m":
		return c.screenMenu(ctx)
	case "/node", "/nodes", "/n":
		c.screenSwitchNode(ctx)
	case "/source", "/src", "/sub":
		c.screenSource(ctx)
	case "/quit", "/exit", "/q":
		return true
	default:
		warnC.Fprintf(c.out, "未知命令 %q。输入 / 查看可用命令。\n", cmd)
		c.pause()
	}
	return false
}
