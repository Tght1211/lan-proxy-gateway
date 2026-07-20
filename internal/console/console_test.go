package console

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/fatih/color"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/gateway"
	"github.com/tght/lan-proxy-gateway/internal/platform"
)

type consoleTestPlatform struct{}

func (consoleTestPlatform) DetectNetwork() (platform.NetworkInfo, error) {
	return platform.NetworkInfo{Interface: "en0", IP: "192.168.12.100"}, nil
}
func (consoleTestPlatform) EnableIPForward() error            { return nil }
func (consoleTestPlatform) DisableIPForward() error           { return nil }
func (consoleTestPlatform) IPForwardEnabled() (bool, error)   { return true, nil }
func (consoleTestPlatform) IsAdmin() (bool, error)            { return true, nil }
func (consoleTestPlatform) InstallService(string) error       { return nil }
func (consoleTestPlatform) UninstallService() error           { return nil }
func (consoleTestPlatform) ServiceStatus() (string, error)    { return "", nil }
func (consoleTestPlatform) SetLocalDNSToLoopback() error      { return nil }
func (consoleTestPlatform) RestoreLocalDNS() error            { return nil }
func (consoleTestPlatform) LocalDNSIsLoopback() (bool, error) { return false, nil }

func newTestConsole(input string, out *bytes.Buffer) *consoleUI {
	return newConsole(&app.App{
		Cfg:     config.Default(),
		Gateway: gateway.New(),
		Plat:    consoleTestPlatform{},
	}, strings.NewReader(input), out)
}

func TestMainMenuIsFocusedOnGatewayAndSystemProxy(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = oldNoColor }()

	var out bytes.Buffer
	c := newTestConsole("0\n", &out)
	if err := c.main(context.Background()); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	for _, want := range []string{
		"2  设置代理",
		"3  查看设备参数",
		"4  查看最近日志",
		"Q  退出",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("menu missing %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "1  启动旁路由") && !strings.Contains(got, "1  停止旁路由") {
		t.Fatalf("menu missing lifecycle action:\n%s", got)
	}
	for _, removed := range []string{"首页", "设备列表", "DNS 设置", "活跃连接", "仪表盘", "节点", "订阅"} {
		if strings.Contains(got, removed) {
			t.Fatalf("menu still exposes removed feature %q:\n%s", removed, got)
		}
	}
}

func TestMainMenuQExits(t *testing.T) {
	c := newTestConsole("q\n", &bytes.Buffer{})
	if err := c.main(context.Background()); err != nil {
		t.Fatal(err)
	}
}
