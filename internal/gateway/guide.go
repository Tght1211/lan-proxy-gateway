package gateway

import (
	"fmt"
	"net/netip"
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
	deviceIP := recommendedDeviceIP(status.LocalIP, status.Router)

	var b strings.Builder
	b.WriteString("  参数\n")
	b.WriteString(fmt.Sprintf("    本机 IP     %s\n", ip))
	b.WriteString(fmt.Sprintf("    路由器      %s\n\n", router))

	b.WriteString("  设备网络设置（照设备页面逐项填写）\n")
	b.WriteString("    IP 地址设置      手动 / 静态\n")
	b.WriteString("    IP 地址（设备自身，推荐）  " + deviceIP + "（确认未占用）\n")
	b.WriteString("    子网掩码         255.255.255.0\n")
	b.WriteString("    前缀长度（Android）  24\n")
	b.WriteString("    网关 / 路由器    " + ip + "\n")
	b.WriteString("    DNS 设置         手动\n")
	b.WriteString("    首选 DNS         " + ip + "\n")
	b.WriteString("    备用 DNS         " + ip + "（设备不允许重复时留空）\n")
	b.WriteString("    代理              无 / 不使用\n")
	b.WriteString("    私人 DNS（Android）  关闭 / 自动\n\n")
	b.WriteString("  说明\n")
	b.WriteString("    · 只有“IP 地址”是这台设备自己的地址；网关和两个 DNS 都填本机 IP\n")
	b.WriteString("    · 设备改完后立即可用：流量经本机转发，出口默认直连本机网络\n")
	b.WriteString("    · 启用系统代理后（菜单 → 2），局域网所有 TCP 复用同一代理\n")
	b.WriteString("    · 推荐 IP 只是同网段建议值，请先确认未占用并在路由器中固定\n")
	return b.String()
}

func recommendedDeviceIP(localIP, routerIP string) string {
	local, err := netip.ParseAddr(strings.TrimSpace(localIP))
	if err != nil || !local.Is4() {
		return "<同网段未占用 IP>"
	}
	router, _ := netip.ParseAddr(strings.TrimSpace(routerIP))
	octets := local.As4()
	for last := 112; last < 255; last++ {
		candidate := netip.AddrFrom4([4]byte{octets[0], octets[1], octets[2], byte(last)})
		if candidate != local && candidate != router {
			return candidate.String()
		}
	}
	return "<同网段未占用 IP>"
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
