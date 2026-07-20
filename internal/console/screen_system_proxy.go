package console

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/systemproxy"
)

func (c *consoleUI) screenSystemProxy(ctx context.Context) {
	c.banner("设置代理")
	status, err := systemproxy.Get()
	if errors.Is(err, systemproxy.ErrNotSupported) {
		fmt.Fprintf(c.out, "  当前: %s\n", configuredEgress(c.app.Cfg.Egress))
	} else if err != nil {
		badC.Fprintf(c.out, "  读取失败: %v\n", err)
	} else {
		fmt.Fprintf(c.out, "  当前: %s\n", status.Summary())
	}
	fmt.Fprintln(c.out, "\n  1  SOCKS5")
	fmt.Fprintln(c.out, "  2  HTTP + HTTPS")
	fmt.Fprintln(c.out, "  3  直连（关闭代理）")
	fmt.Fprintln(c.out, "  0  返回")
	titleC.Fprint(c.out, "\n选择：> ")

	switch strings.ToLower(c.readLine()) {
	case "1":
		c.configureSystemProxy(ctx, systemproxy.ModeSOCKS5)
	case "2":
		c.configureSystemProxy(ctx, systemproxy.ModeHTTP)
	case "3":
		if !c.canConfigureSystemProxy() {
			c.pause()
			return
		}
		if err := systemproxy.Disable(); err != nil && !errors.Is(err, systemproxy.ErrNotSupported) {
			badC.Fprintf(c.out, "  关闭失败: %v\n", err)
		} else if err := c.app.SetEgress(ctx, config.EgressConfig{Mode: config.EgressDirect}, false); err != nil {
			badC.Fprintf(c.out, "  代理已关闭，但旁路由切回直连失败: %v\n", err)
		} else {
			okC.Fprintln(c.out, "  已切换为直连")
		}
		c.pause()
	case "", "0", "q":
		return
	default:
		warnC.Fprintln(c.out, "  无效选项")
		c.pause()
	}
}

func (c *consoleUI) configureSystemProxy(ctx context.Context, mode systemproxy.Mode) {
	if !c.canConfigureSystemProxy() {
		return
	}
	proxy := c.app.Cfg.Egress.Proxy
	host := strings.TrimSpace(c.ask("  代理地址", nonEmpty(proxy.Host, "127.0.0.1")))
	defaultPort := proxy.Port
	if defaultPort == 0 {
		defaultPort = 7897
	}
	port, err := strconv.Atoi(c.ask("  代理端口", strconv.Itoa(defaultPort)))
	if err != nil {
		warnC.Fprintln(c.out, "  端口无效")
		return
	}
	previous := c.app.Cfg.Egress
	next := config.EgressConfig{
		Mode:  config.EgressProxy,
		Proxy: config.ProxyConfig{Type: string(mode), Host: host, Port: port},
	}
	dimC.Fprintln(c.out, "  正在测试代理连通性…")
	if err := c.app.SetEgress(ctx, next, true); err != nil {
		badC.Fprintf(c.out, "  设置失败: %v\n", err)
	} else if err := systemproxy.Enable(mode, host, port); err != nil && !errors.Is(err, systemproxy.ErrNotSupported) {
		_ = c.app.SetEgress(ctx, previous, false)
		badC.Fprintf(c.out, "  设置失败: %v\n", err)
	} else {
		if errors.Is(err, systemproxy.ErrNotSupported) {
			okC.Fprintf(c.out, "  ✓ 旁路由出口已启用: %s %s:%d\n", mode, host, port)
		} else {
			okC.Fprintf(c.out, "  ✓ 系统代理和旁路由出口已启用: %s %s:%d\n", mode, host, port)
		}
	}
}

func (c *consoleUI) canConfigureSystemProxy() bool {
	if runtime.GOOS != "darwin" {
		return true
	}
	admin, _ := c.app.Plat.IsAdmin()
	if admin {
		return true
	}
	warnC.Fprintln(c.out, "  修改 macOS 系统代理需要管理员权限，请运行: sudo gateway")
	return false
}
