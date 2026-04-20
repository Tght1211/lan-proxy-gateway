package gateway

import (
	"fmt"
	"strings"
)

// DeviceGuide returns multi-line instructions that a user can paste onto their
// phone/Switch/PS5 to point that device at this gateway.
func DeviceGuide(status Status) string {
	ip := status.LocalIP
	if ip == "" {
		ip = "<本机局域网 IP>"
	}
	router := firstNonEmpty(status.Router, "<路由器 IP>")

	var b strings.Builder
	b.WriteString("=== 设备接入指引 ===\n")
	b.WriteString(fmt.Sprintf("  本机局域网 IP : %s\n", ip))
	b.WriteString(fmt.Sprintf("  路由器 IP     : %s\n", router))
	b.WriteString("\n在设备（Switch / PS5 / Apple TV / 手机）的网络设置里填：\n")
	b.WriteString(fmt.Sprintf("  网关 (Gateway)  →  %s\n", ip))
	b.WriteString(fmt.Sprintf("  DNS 服务器       →  %s\n", ip))
	b.WriteString("  子网掩码          →  255.255.255.0        （99% 家庭网就这个值；与路由器一致即可）\n")
	b.WriteString("  IP 地址           →  原值保留；或固定一个同网段地址（如 192.168.x.200）\n")
	b.WriteString("\n保存并重连 Wi-Fi 即生效。\n")
	b.WriteString("\n前提：本机 TUN 要开（才会劫持流量进代理），DNS 代理要开（设备 DNS 指向本机才能解析）。\n")
	return b.String()
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
