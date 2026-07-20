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

	// v4：改网关 + 改 DNS 指向本机是唯一接入方式，不再出现代理端口话术。
	for _, want := range []string{
		"参数",
		"192.168.12.100",
		"网关 / 路由器",
		"DNS",
		"设备接入",
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
