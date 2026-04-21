package gateway

import (
	"fmt"
	"strings"
)

// DeviceGuide 返回给设备接入者看的紧凑说明。
// mixedPort 是 mihomo 的 HTTP+SOCKS5 混合端口（方式 2 要填）。
func DeviceGuide(status Status, mixedPort int) string {
	ip := status.LocalIP
	if ip == "" {
		ip = "<本机局域网 IP>"
	}
	router := firstNonEmpty(status.Router, "<路由器 IP>")

	var b strings.Builder
	b.WriteString(fmt.Sprintf("  本机 IP %s  ·  路由器 %s  ·  代理端口 %d (HTTP+SOCKS5)\n\n",
		ip, router, mixedPort))

	b.WriteString(fmt.Sprintf("  📺 方式 1 · 改网关      Switch / PS5 / Apple TV / 智能电视\n"))
	b.WriteString(fmt.Sprintf("     网关 → %s    DNS → %s    掩码 → 255.255.255.0\n\n", ip, ip))

	b.WriteString(fmt.Sprintf("  📱 方式 2 · 填代理      iPhone / 电脑 App / 浏览器插件\n"))
	b.WriteString(fmt.Sprintf("     代理服务器 → %s    端口 → %d\n\n", ip, mixedPort))

	b.WriteString("  💻 方式 3 · 本机自己用  跑 gateway 这台电脑的浏览器 / App 也想走规则\n")
	b.WriteString("     把本机 DNS 改到 127.0.0.1，让本机查询走 mihomo\n")
	b.WriteString("     macOS：sudo networksetup -setdnsservers Wi-Fi 127.0.0.1\n\n")

	b.WriteString("  💡 方式 1 需 TUN+DNS 都开；方式 2/3 只要 gateway 在跑\n")
	b.WriteString("  💡 验证方式 3：dscacheutil -q host -a name ping0.cc 返回 198.18.x.x 说明生效\n")

	return b.String()
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
