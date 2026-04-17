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
	var b strings.Builder
	b.WriteString("=== 设备接入指引 ===\n")
	b.WriteString(fmt.Sprintf("本机局域网 IP: %s\n", ip))
	b.WriteString(fmt.Sprintf("默认路由网关 : %s\n", firstNonEmpty(status.Router, "<路由器 IP>")))
	b.WriteString("\n在想接入的设备（Switch / PS5 / Apple TV / 手机）网络设置中：\n")
	b.WriteString(fmt.Sprintf("  • 网关 (Gateway)      改成 %s\n", ip))
	b.WriteString(fmt.Sprintf("  • DNS 服务器          改成 %s\n", ip))
	b.WriteString("  • 其他 (IP / 子网掩码)  保持不变\n")
	b.WriteString("\n保存后重连 Wi-Fi，设备流量就会经由本机网关出口。\n")
	b.WriteString("\n⚠ 需满足两个前提：\n")
	b.WriteString("  • 本机 TUN 必须【开】：TUN 负责劫持流量并按规则走代理。\n")
	b.WriteString("    关了 TUN 的话流量只会被普通路由转发，Switch/PS5 照样被墙。\n")
	b.WriteString("  • 本机 DNS 代理【开】时，设备 DNS 指向本机 IP 即可；\n")
	b.WriteString("    如果 DNS 代理关了（比如 Clash Verge 占了 53 端口），\n")
	b.WriteString("    设备 DNS 仍可指向本机 IP（由占用方回答），或改成 114.114.114.114。\n")
	b.WriteString("\n如果手机上 YouTube 能连但 Switch 连不上，先重启 Switch 再试。\n")
	return b.String()
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
