package console

import (
	"context"
	"fmt"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/systemproxy"
)

// onboard creates the minimal gateway config. An existing macOS system proxy
// is reused as the LAN egress so users maintain only one proxy endpoint.
func (c *consoleUI) onboard(_ context.Context) error {
	c.banner("欢迎使用 lan-proxy-gateway · 首次配置")

	if err := c.app.Gateway.Detect(); err == nil {
		info := c.app.Gateway.Info()
		fmt.Fprintf(c.out, "  默认接口  %s\n", info.Interface)
		fmt.Fprintf(c.out, "  本机 IP   %s\n", info.IP)
		fmt.Fprintf(c.out, "  路由器 IP %s\n\n", info.Gateway)
	}

	c.app.Cfg.Egress = config.EgressConfig{Mode: config.EgressDirect}
	if status, err := systemproxy.Get(); err == nil {
		for _, service := range status.Services {
			if !service.Enabled {
				continue
			}
			c.app.Cfg.Egress = config.EgressConfig{
				Mode: config.EgressProxy,
				Proxy: config.ProxyConfig{
					Type: string(service.Mode), Host: service.Host, Port: service.Port,
				},
			}
			okC.Fprintf(c.out, "  ✓ 已复用系统代理作为旁路由出口: %s %s:%d\n", service.Mode, service.Host, service.Port)
			break
		}
	}

	fmt.Fprintln(c.out, "  ✓ 代理模式保留域名给代理软件，不劫持设备的其他 DNS")
	fmt.Fprintln(c.out, "  ✓ 代理规则和节点由系统代理对应的软件处理")

	if err := c.app.Save(); err != nil {
		badC.Fprintf(c.out, "保存配置失败: %v\n", err)
		return err
	}
	okC.Fprintln(c.out, "\n配置已保存到 "+c.app.Paths.ConfigFile)
	dimC.Fprintln(c.out, "可在控制面板的“系统代理 / LAN 出口”中启用或关闭代理。")
	return nil
}
