package gateway

import (
	"strings"
	"testing"
)

func TestDeviceGuideGatewayAndDNS(t *testing.T) {
	out := DeviceGuide(Status{
		LocalIP: "192.168.12.100",
		Router:  "192.168.12.1",
	})

	// Labels mirror the manual network fields shown by consoles and phones.
	for _, want := range []string{
		"参数",
		"192.168.12.100",
		"IP 地址设置      手动 / 静态",
		"IP 地址（设备自身，推荐）  192.168.12.112",
		"子网掩码         255.255.255.0",
		"前缀长度（Android）  24",
		"网关 / 路由器",
		"DNS 设置         手动",
		"首选 DNS         192.168.12.100",
		"备用 DNS         192.168.12.100",
		"代理              无 / 不使用",
		"私人 DNS（Android）  关闭 / 自动",
		"设备网络设置",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("guide missing %q:\n%s", want, out)
		}
	}
	for _, banned := range []string{"填代理", "端口=", "TUN", "mixed"} {
		if strings.Contains(out, banned) {
			t.Fatalf("guide must not mention %q in v4:\n%s", banned, out)
		}
	}
}

func TestRecommendedDeviceIPAvoidsGatewayAndRouter(t *testing.T) {
	if got := recommendedDeviceIP("192.168.12.112", "192.168.12.113"); got != "192.168.12.114" {
		t.Fatalf("recommended IP = %q, want 192.168.12.114", got)
	}
	if got := recommendedDeviceIP("not-an-ip", ""); got != "<同网段未占用 IP>" {
		t.Fatalf("fallback = %q", got)
	}
}
