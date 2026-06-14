package console

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/engine"
)

func (c *consoleUI) screenLifecycle(ctx context.Context) {
	c.banner("启动 / 重启 / 停止")
	s := c.app.Status()
	runStatus := dimC.Sprint("○ 未启动")
	if s.Running {
		runStatus = okC.Sprint("● 运行中")
	}
	fmt.Fprintf(c.out, "  当前状态: %s\n\n", runStatus)
	fmt.Fprintln(c.out, "  1  启动")
	fmt.Fprintln(c.out, "  2  重启              （等同停止 + 启动，让新配置完整生效）")
	fmt.Fprintln(c.out, "  3  停止              （mihomo 结束，LAN 设备走直连）")
	fmt.Fprintln(c.out, "  4  清理残留 mihomo   （端口被占用时用，会杀掉系统里所有 mihomo 进程）")
	fmt.Fprintln(c.out)
	titleC.Fprintln(c.out, "  ── 操作 ── 0 返回主菜单（或按 Q）")
	admin, _ := c.app.Plat.IsAdmin()
	if !admin {
		// 运行中时重启/停止只是拉/杀用户态 mihomo，不需要 sudo；只有「首次启动」
		// 才要改系统状态（IP 转发、TUN 设备）。按是否在运行给两种语气的提示，
		// 避免旧文案把「启动/停止/清理全都要 sudo」这条过度告警给误导用户。
		if s.Running {
			if loopback, _ := c.app.Plat.LocalDNSIsLoopback(); loopback {
				warnC.Fprintln(c.out, "  （本机 DNS 指向 127.0.0.1；停止会恢复 DNS，建议用 sudo gateway stop）")
			} else {
				dimC.Fprintln(c.out, "  （未用 sudo：已在运行，重启/停止通常不需要 sudo；切 TUN 或绑 53 端口才需要）")
			}
		} else {
			warnC.Fprintln(c.out, "  （未用 sudo 运行：启动需要 sudo，请退出后运行 sudo gateway）")
		}
	}
	switch c.prompt("选择：> ") {
	case "1":
		if !admin {
			badC.Fprintln(c.out, "需要 sudo 权限。请退出后运行: sudo gateway start")
			return
		}
		c.tryStart(ctx)
	case "2":
		if err := c.app.Engine.Reload(ctx, c.app.Cfg); err != nil {
			badC.Fprintf(c.out, "重启失败: %v\n", err)
		} else {
			okC.Fprintln(c.out, "已重启")
		}
	case "3":
		if err := c.app.Stop(); err != nil {
			badC.Fprintln(c.out, err.Error())
		} else {
			okC.Fprintln(c.out, "已停止，并已检查本机 DNS")
		}
	case "4":
		c.cleanupStaleMihomo()
	case "0", "q", "Q", "":
		return
	default:
		warnC.Fprintln(c.out, "无效选项，按 0 或 Q 返回主菜单")
	}
}

// shutdownGateway stops mihomo (if running) and signals the main loop to exit.
// Returns true when the caller should return out of the menu loop.
func (c *consoleUI) shutdownGateway() bool {
	running := c.app.Engine != nil && c.app.Engine.Running()
	if running {
		warnC.Fprintln(c.out, "\n这会停止 mihomo 并退出控制台，LAN 里指向本机的设备会失去代理。")
		if !c.yesNo("确定要关闭 gateway？", false) {
			return false
		}
		if err := c.app.Stop(); err != nil {
			badC.Fprintf(c.out, "停止失败: %v\n", err)
			return false
		}
		okC.Fprintln(c.out, "已停止 mihomo，并已检查本机 DNS")
	}
	return true
}

// tryStart runs Start() and, on port conflict caused by a stale mihomo, offers
// to kill it and retry in-place. This is the most common recovery path for
// users who ran gateway once, crashed/killed the terminal, and now find the
// port squatted by an orphan process.
func (c *consoleUI) tryStart(ctx context.Context) {
	if c.app.Engine.Running() {
		okC.Fprintln(c.out, "已经在运行，无需再启动（如需应用新配置，选 2 重启 / 热重载）")
		return
	}
	err := c.app.Start(ctx)
	if err == nil {
		okC.Fprintln(c.out, "已启动")
		return
	}
	var pce *engine.PortConflictError
	if errors.As(err, &pce) && pce.HasStaleMihomo() {
		warnC.Fprintln(c.out, err.Error())
		pids := pce.StaleMihomoPIDs()
		fmt.Fprintf(c.out, "\n检测到残留的 mihomo 进程（PID %v），这是上一次运行没退干净留下的。\n", pids)
		if c.yesNo("是否自动干掉它们并重新启动？", true) {
			for _, pid := range pids {
				if kErr := engine.KillPID(pid); kErr != nil {
					badC.Fprintf(c.out, "  ✗ kill PID %d 失败: %v\n", pid, kErr)
				} else {
					okC.Fprintf(c.out, "  ✓ 已终止 PID %d\n", pid)
				}
			}
			// 再试一次
			if err := c.app.Start(ctx); err != nil {
				badC.Fprintf(c.out, "启动仍然失败: %v\n", err)
			} else {
				okC.Fprintln(c.out, "已启动")
			}
			return
		}
	}
	badC.Fprintf(c.out, "启动失败: %v\n", err)
}

// cleanupStaleMihomo finds every mihomo on the host and offers to kill them.
// Works even when no port conflict is known — useful after a hard crash.
func (c *consoleUI) cleanupStaleMihomo() {
	pids := engine.FindStaleMihomoPIDs()
	// Filter out our own child so we don't nuke a running gateway.
	if c.app.Engine != nil && c.app.Engine.Running() {
		ownPID := os.Getpid()
		filtered := pids[:0]
		for _, p := range pids {
			if p != ownPID {
				filtered = append(filtered, p)
			}
		}
		pids = filtered
	}
	if len(pids) == 0 {
		okC.Fprintln(c.out, "没发现残留 mihomo 进程")
		return
	}
	fmt.Fprintf(c.out, "发现 %d 个 mihomo 进程: %v\n", len(pids), pids)
	if !c.yesNo("全部杀掉？", true) {
		dimC.Fprintln(c.out, "取消")
		return
	}
	for _, pid := range pids {
		if err := engine.KillPID(pid); err != nil {
			badC.Fprintf(c.out, "  ✗ PID %d: %v\n", pid, err)
		} else {
			okC.Fprintf(c.out, "  ✓ 已终止 PID %d\n", pid)
		}
	}
}

func (c *consoleUI) screenLogs() {
	path := c.app.Engine.LogPath()
	tailN := 30
	rawMode := false // 默认走易读视图；r 切换回原始 mihomo 行
	for {
		view := "易读视图"
		if rawMode {
			view = "原始视图"
		}
		c.banner(fmt.Sprintf("日志（%s）· 末尾 %d 行 · %s", view, tailN, path))
		if err := c.renderTail(path, tailN, rawMode); err != nil {
			warnC.Fprintf(c.out, "%v\n", err)
		}
		dimC.Fprintln(c.out, "\n  [回车] 刷新   [t] tail 跟随   [r] 切换原始/易读   [数字] 改行数   [q] 返回")
		input := c.prompt("> ")
		switch {
		case input == "":
			// 手动刷新：重绘一遍即可
		case strings.EqualFold(input, "q") || strings.EqualFold(input, "quit"):
			return
		case strings.EqualFold(input, "t") || strings.EqualFold(input, "tail"):
			c.tailFollow(path, tailN, rawMode)
		case strings.EqualFold(input, "r") || strings.EqualFold(input, "raw"):
			rawMode = !rawMode
		default:
			if n, err := strconv.Atoi(input); err == nil && n > 0 {
				tailN = n
			} else {
				warnC.Fprintln(c.out, "无效输入（回车=刷新，t=tail，r=切视图，数字=改行数，q=返回）")
				c.pause()
			}
		}
	}
}

// renderTail 把日志文件的末尾 n 行打到输出。文件不存在时返回友好错误。
// rawMode=false 时每行走 humanizeMihomoLine 翻译成中文简要。
func (c *consoleUI) renderTail(path string, n int, rawMode bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("暂无日志 (%s)", path)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	start := 0
	if len(lines) > n {
		start = len(lines) - n
	}
	if rawMode {
		for _, l := range lines[start:] {
			fmt.Fprintln(c.out, l)
		}
		return nil
	}
	// 易读模式下同一告警每 15 秒重复一次会把末尾几十行刷成一片，走 dedup 折叠
	dd := newLineDeduper(c.out)
	for _, l := range lines[start:] {
		formatted, key, t := humanizeMihomoLineWithKey(l)
		dd.Write(formatted, key, t)
	}
	dd.Flush()
	return nil
}

// tailFollow 进入实时跟随模式：先打印末尾 n 行做上下文，然后轮询文件 size 把
// 新增字节流写到终端，按回车退出。文件被截断（mihomo rotate/重启）时重置 offset。
// rawMode=false 时按行缓冲并逐行 humanize。
func (c *consoleUI) tailFollow(path string, n int, rawMode bool) {
	c.banner("日志 tail · " + path)
	_ = c.renderTail(path, n, rawMode)

	f, err := os.Open(path)
	if err != nil {
		warnC.Fprintf(c.out, "无法打开 %s: %v\n", path, err)
		c.pause()
		return
	}
	defer f.Close()
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		warnC.Fprintf(c.out, "seek 失败: %v\n", err)
		c.pause()
		return
	}
	dimC.Fprintln(c.out, "\n（实时跟踪中，按回车退出）")

	// 等一个回车退出 tail。从 inputCh 读一行（已在后台持续接收），
	// 这样就不会和 startInputLoop 的 reader 争 c.in。
	done := make(chan struct{})
	go func() {
		_ = c.readLine()
		close(done)
	}()

	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	var lastSize int64
	if info, err := f.Stat(); err == nil {
		lastSize = info.Size()
	}
	buf := make([]byte, 8192)
	var pending strings.Builder // 易读模式下用来攒半行
	// 实时段独立一个 deduper：从 renderTail 过渡到实时可能短暂重复打印一条，
	// 容忍一下——比跨段共享 deduper 导致的锁/状态耦合简单。
	var dd *lineDeduper
	if !rawMode {
		dd = newLineDeduper(c.out)
		defer dd.Flush()
	}
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			info, err := f.Stat()
			if err != nil {
				continue
			}
			if info.Size() < lastSize {
				// 文件被截断（rotate / mihomo 重启），从头读
				_, _ = f.Seek(0, io.SeekStart)
				lastSize = 0
				pending.Reset()
				if dd != nil {
					dd.Flush()
				}
			}
			for {
				rn, rerr := f.Read(buf)
				if rn > 0 {
					if rawMode {
						_, _ = c.out.Write(buf[:rn])
					} else {
						pending.Write(buf[:rn])
						// 逐行切出来翻译。最后半行留给下一轮。
						for {
							s := pending.String()
							nl := strings.IndexByte(s, '\n')
							if nl < 0 {
								break
							}
							formatted, key, t := humanizeMihomoLineWithKey(s[:nl])
							dd.Write(formatted, key, t)
							pending.Reset()
							pending.WriteString(s[nl+1:])
						}
					}
				}
				if rerr != nil {
					break
				}
			}
			lastSize = info.Size()
		}
	}
}

func (c *consoleUI) pause() {
	dimC.Fprintln(c.out, "\n按回车返回…")
	_ = c.readLine()
}
