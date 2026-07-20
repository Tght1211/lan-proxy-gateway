package console

import (
	"fmt"

	"github.com/tght/lan-proxy-gateway/internal/gateway"
)

func (c *consoleUI) screenGateway() {
	c.banner("设备接入参数")
	_ = c.app.Gateway.Detect()
	fmt.Fprint(c.out, gateway.DeviceGuide(c.app.Status().Gateway))
	dimC.Fprintln(c.out, "\n  DNS2 留空；若设备强制要求填写，则与 DNS1 填相同地址。")
	dimC.Fprintln(c.out, "  Android/ColorOS 请关闭私人 DNS。")
	c.pause()
}
