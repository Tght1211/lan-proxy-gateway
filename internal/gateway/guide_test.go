package gateway

import (
	"strings"
	"testing"
)

func TestDeviceGuideIsCompactTable(t *testing.T) {
	out := DeviceGuide(Status{
		LocalIP: "192.168.12.100",
		Router:  "192.168.12.1",
	}, 17890)

	for _, want := range []string{
		"参数",
		"接入方式",
		"改网关",
		"填代理",
		"主机=192.168.12.100",
		"端口=17890",
		"停止 gateway 会自动恢复本机 DNS",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("guide missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "验证方式 3") {
		t.Fatalf("guide should stay compact, got old verbose hint:\n%s", out)
	}
}
