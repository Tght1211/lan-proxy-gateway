package console

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/systemproxy"
)

func (c *consoleUI) main(ctx context.Context) error {
	for {
		c.drawMainMenu()
		switch strings.ToLower(c.readLine()) {
		case "1":
			c.screenLifecycle(ctx)
		case "2":
			c.screenSystemProxy(ctx)
		case "3":
			c.screenGateway()
		case "4":
			c.screenLogs()
		case "", "0", "q", "quit", "exit":
			return nil
		default:
			warnC.Fprintln(c.out, "请输入 1-4，或 0 退出")
			c.pause()
		}
	}
}

func (c *consoleUI) drawMainMenu() {
	c.clearScreen()
	c.banner("lan-proxy-gateway")
	status := c.app.Status()
	if status.Running {
		okC.Fprint(c.out, "  ● 旁路由运行中")
	} else {
		dimC.Fprint(c.out, "  ○ 旁路由未启动")
	}
	ip := status.Gateway.LocalIP
	if ip == "" {
		ip = "未检测"
	}
	fmt.Fprintf(c.out, "    本机 IP %s\n", ip)
	if proxy, err := systemproxy.Get(); err == nil {
		fmt.Fprintf(c.out, "  系统代理 / LAN 出口: %s\n", proxy.Summary())
	} else if errors.Is(err, systemproxy.ErrNotSupported) {
		fmt.Fprintf(c.out, "  LAN 出口: %s\n", configuredEgress(c.app.Cfg.Egress))
	} else {
		warnC.Fprintf(c.out, "  代理状态读取失败: %v\n", err)
	}

	fmt.Fprintln(c.out)
	if status.Running {
		fmt.Fprintln(c.out, "  1  停止旁路由")
	} else {
		fmt.Fprintln(c.out, "  1  启动旁路由")
	}
	fmt.Fprintln(c.out, "  2  设置代理")
	fmt.Fprintln(c.out, "  3  查看设备参数")
	fmt.Fprintln(c.out, "  4  查看最近日志")
	fmt.Fprintln(c.out, "  Q  退出")
	titleC.Fprint(c.out, "\n选择：> ")
}

func configuredEgress(egress config.EgressConfig) string {
	if egress.Mode != config.EgressProxy {
		return "直连"
	}
	return fmt.Sprintf("%s %s:%d", egress.Proxy.Type, egress.Proxy.Host, egress.Proxy.Port)
}
