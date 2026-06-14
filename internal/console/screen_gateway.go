package console

import (
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/tght/lan-proxy-gateway/internal/gateway"
	"github.com/tght/lan-proxy-gateway/internal/platform"
)

// --- Screens ---

func (c *consoleUI) screenGateway() {
	for {
		c.banner("设备接入指引")
		_ = c.app.Gateway.Detect()
		fmt.Fprint(c.out, gateway.DeviceGuide(c.app.Status().Gateway, c.app.Cfg.Runtime.Ports.Mixed))

		// Windows 下 TUN 已经接管本机出向流量，方式 3 不需要切 DNS；且
		// SetLocalDNSToLoopback 在 Windows 上是 ErrNotSupported，按钮按下
		// 只会报错。整块隐藏，避免跟上面指引里"TUN 已开，自动走 mihomo"
		// 自相矛盾。
		if runtime.GOOS == "windows" {
			fmt.Fprintln(c.out)
			titleC.Fprintln(c.out, "  ── 操作 ── T 给 IP 起名字   0 返回（或按 Q）")
			switch strings.ToLower(strings.TrimSpace(c.prompt("选择：> "))) {
			case "t":
				c.screenDeviceLabels()
			case "", "0", "q":
				return
			default:
				warnC.Fprintln(c.out, "无效选项")
			}
			continue
		}

		// 方式 3 的状态灯 + 一键开关（macOS 实打实切；Linux 显示命令提示）
		isLoopback, _ := c.app.Plat.LocalDNSIsLoopback()
		if isLoopback {
			okC.Fprintln(c.out, "\n  本机 DNS: 127.0.0.1（停止 gateway 时自动恢复）")
		} else {
			dimC.Fprintln(c.out, "\n  本机 DNS: 默认")
		}
		fmt.Fprintln(c.out)
		titleC.Fprintln(c.out, "  ── 操作 ── L 切本机 DNS   R 恢复 DNS   T 标记设备   0 返回（或按 Q）")

		switch strings.ToLower(strings.TrimSpace(c.prompt("选择：> "))) {
		case "l":
			if err := c.app.Plat.SetLocalDNSToLoopback(); err != nil {
				if errors.Is(err, platform.ErrNotSupported) {
					warnC.Fprintln(c.out, "  当前系统不支持一键切换，请照上面命令手动改")
					c.pause()
				} else {
					badC.Fprintf(c.out, "  切换失败: %v\n", err)
				}
			} else {
				okC.Fprintln(c.out, "  ✓ 已把本机 DNS 切到 127.0.0.1（顺手关了系统 HTTP/SOCKS 代理）")
			}
		case "r":
			if err := c.app.Plat.RestoreLocalDNS(); err != nil {
				if errors.Is(err, platform.ErrNotSupported) {
					warnC.Fprintln(c.out, "  当前系统不支持一键恢复")
					c.pause()
				} else {
					badC.Fprintf(c.out, "  恢复失败: %v\n", err)
				}
			} else {
				okC.Fprintln(c.out, "  ✓ 已恢复系统默认 DNS")
			}
		case "t":
			c.screenDeviceLabels()
		case "", "0", "q":
			return
		default:
			warnC.Fprintln(c.out, "无效选项")
		}
	}
}
