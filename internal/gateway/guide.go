package gateway

import (
	"fmt"
	"strings"
)

// DeviceGuide 返回给设备接入者看的紧凑说明。
// v4 只有一种接入方式：把设备的 网关 + DNS 都指向本机 IP。
func DeviceGuide(status Status) string {
	ip := status.LocalIP
	if ip == "" {
		ip = "<本机局域网 IP>"
	}
	router := firstNonEmpty(status.Router, "<路由器 IP>")

	var b strings.Builder
	b.WriteString("  参数\n")
	b.WriteString(fmt.Sprintf("    本机 IP     %s\n", ip))
	b.WriteString(fmt.Sprintf("    路由器      %s\n\n", router))

	b.WriteString("  设备接入（任选一台设备，改它的网络设置）\n")
	b.WriteString("    网关 / 路由器   " + ip + "\n")
	b.WriteString("    DNS            " + ip + "\n")
	b.WriteString("    子网掩码        255.255.255.0（保持原值即可）\n\n")
	b.WriteString("  说明\n")
	b.WriteString("    · 设备改完后立即可用：流量经本机转发，出口默认直连本机网络\n")
	b.WriteString("    · 启用系统代理后（菜单 → 2），局域网所有 TCP 复用同一代理\n")
	b.WriteString("    · 建议保持设备 IP 为静态或在路由器里固定分配，避免改漂移\n")
	return b.String()
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
