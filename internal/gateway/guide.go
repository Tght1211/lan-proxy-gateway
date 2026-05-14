package gateway

import (
	"fmt"
	"runtime"
	"strings"
)

// DeviceGuide 返回给设备接入者看的紧凑说明。
// mixedPort 是 mihomo 的 HTTP+SOCKS5 混合端口（方式 2 要填）。
//
// 平台差异很大，尤其 Windows：这台机只能做 HTTP/SOCKS 代理，没法做透明
// 网关，因为 ConfigureNAT 在 Windows 上是 no-op（家用版 Windows 没 RRAS，
// ICS 强制 192.168.137/24 不好用）。指引里必须明说，不然用户设了"网关"
// 发现连不上网会一脸懵。
func DeviceGuide(status Status, mixedPort int) string {
	ip := status.LocalIP
	if ip == "" {
		ip = "<本机局域网 IP>"
	}
	router := firstNonEmpty(status.Router, "<路由器 IP>")

	var b strings.Builder
	b.WriteString("  参数\n")
	b.WriteString(fmt.Sprintf("    本机 IP     %s\n", ip))
	b.WriteString(fmt.Sprintf("    路由器      %s\n", router))
	b.WriteString(fmt.Sprintf("    代理端口    %d (HTTP+SOCKS5)\n\n", mixedPort))

	if runtime.GOOS == "windows" {
		b.WriteString("  接入方式\n")
		b.WriteString("    方式        适合                  填写\n")
		b.WriteString(fmt.Sprintf("    填代理      手机 / 电脑 / 浏览器    主机=%s  端口=%d  类型=HTTP\n", ip, mixedPort))
		b.WriteString("    本机使用    当前 Windows           TUN 开启后自动生效\n\n")
		b.WriteString("  Windows 不支持改网关；游戏机 / 电视请用 macOS、Linux 或软路由。\n")
		return b.String()
	}

	b.WriteString("  接入方式\n")
	b.WriteString("    方式        适合                    填写\n")
	b.WriteString(fmt.Sprintf("    改网关      游戏机 / 电视            网关=%s  DNS=%s  掩码=255.255.255.0\n", ip, ip))
	b.WriteString(fmt.Sprintf("    填代理      手机 / 电脑 / 浏览器      主机=%s  端口=%d  类型=HTTP\n", ip, mixedPort))
	b.WriteString("    本机使用    当前电脑                 TUN 开启即可；需要时按 L 切 DNS\n\n")
	b.WriteString("  备注: 停止 gateway 会自动恢复本机 DNS。\n")

	return b.String()
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
