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
func (consoleTestPlatform) EnableIPForward() error                   { return nil }
func (consoleTestPlatform) DisableIPForward() error                  { return nil }
func (consoleTestPlatform) IPForwardEnabled() (bool, error)          { return true, nil }
func (consoleTestPlatform) ConfigureNAT(string) error                { return nil }
func (consoleTestPlatform) UnconfigureNAT(string) error              { return nil }
func (consoleTestPlatform) PostStopCleanup() error                   { return nil }
func (consoleTestPlatform) ResolveMihomoPath(string) (string, error) { return "", nil }
func (consoleTestPlatform) IsAdmin() (bool, error)                   { return true, nil }
func (consoleTestPlatform) InstallService(string) error              { return nil }
func (consoleTestPlatform) UninstallService() error                  { return nil }
func (consoleTestPlatform) ServiceStatus() (string, error)           { return "", nil }
func (consoleTestPlatform) SetLocalDNSToLoopback() error             { return nil }
func (consoleTestPlatform) RestoreLocalDNS() error                   { return nil }
func (consoleTestPlatform) LocalDNSIsLoopback() (bool, error)        { return false, nil }

func TestScreenMenuQReturnsDashboard(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = oldNoColor }()

	var out bytes.Buffer
	c := newConsole(&app.App{
		Cfg:     config.Default(),
		Gateway: gateway.New(),
		Plat:    consoleTestPlatform{},
	}, strings.NewReader("q\n"), &out)

	if c.screenMenu(context.Background()) {
		t.Fatal("screenMenu(q) should return to dashboard, not exit console")
	}
	if !strings.Contains(out.String(), "Q  返回首页") {
		t.Fatalf("expected menu output, got: %s", out.String())
	}
}

func TestScreenMenuZeroReturnsDashboard(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = oldNoColor }()

	c := newConsole(&app.App{
		Cfg:     config.Default(),
		Gateway: gateway.New(),
		Plat:    consoleTestPlatform{},
	}, strings.NewReader("0\n"), &bytes.Buffer{})

	if c.screenMenu(context.Background()) {
		t.Fatal("screenMenu(0) should return to dashboard, not exit console")
	}
}
