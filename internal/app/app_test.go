package app

import (
	"errors"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/platform"
)

type fakePlatform struct {
	loopback      bool
	restoreCalled int
	restoreErr    error
}

func (p *fakePlatform) DetectNetwork() (platform.NetworkInfo, error) {
	return platform.NetworkInfo{}, nil
}
func (p *fakePlatform) EnableIPForward() error                   { return nil }
func (p *fakePlatform) DisableIPForward() error                  { return nil }
func (p *fakePlatform) IPForwardEnabled() (bool, error)          { return false, nil }
func (p *fakePlatform) ConfigureNAT(string) error                { return nil }
func (p *fakePlatform) UnconfigureNAT(string) error              { return nil }
func (p *fakePlatform) PostStopCleanup() error                   { return nil }
func (p *fakePlatform) ResolveMihomoPath(string) (string, error) { return "", nil }
func (p *fakePlatform) IsAdmin() (bool, error)                   { return true, nil }
func (p *fakePlatform) InstallService(string) error              { return nil }
func (p *fakePlatform) UninstallService() error                  { return nil }
func (p *fakePlatform) ServiceStatus() (string, error)           { return "", nil }
func (p *fakePlatform) SetLocalDNSToLoopback() error             { return nil }
func (p *fakePlatform) RestoreLocalDNS() error {
	p.restoreCalled++
	return p.restoreErr
}
func (p *fakePlatform) LocalDNSIsLoopback() (bool, error) { return p.loopback, nil }

func TestStopRestoresLocalDNSWhenLoopback(t *testing.T) {
	plat := &fakePlatform{loopback: true}
	a := &App{Plat: plat}

	if err := a.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if plat.restoreCalled != 1 {
		t.Fatalf("RestoreLocalDNS calls = %d, want 1", plat.restoreCalled)
	}
}

func TestStopSkipsLocalDNSRestoreWhenNotLoopback(t *testing.T) {
	plat := &fakePlatform{loopback: false}
	a := &App{Plat: plat}

	if err := a.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if plat.restoreCalled != 0 {
		t.Fatalf("RestoreLocalDNS calls = %d, want 0", plat.restoreCalled)
	}
}

func TestStopReportsLocalDNSRestoreFailure(t *testing.T) {
	restoreErr := errors.New("boom")
	plat := &fakePlatform{loopback: true, restoreErr: restoreErr}
	a := &App{Plat: plat}

	if err := a.Stop(); !errors.Is(err, restoreErr) {
		t.Fatalf("Stop error = %v, want restore error", err)
	}
}

func TestStartLocalExternalProxySkipsGatewayEnable(t *testing.T) {
	cfg := config.Default()
	cfg.Source.Type = config.SourceTypeExternal
	cfg.Source.External.Server = "127.0.0.1"
	cfg.Source.External.Port = 6578
	cfg.Source.External.Kind = "socks5"
	cfg.Gateway.Enabled = true
	cfg.Gateway.TUN.Enabled = true
	cfg.Gateway.DNS.Enabled = true

	a := &App{Cfg: cfg}
	err := a.Start(nil)
	if err == nil || err.Error() != "mihomo 未找到，请先运行 `gateway install`" {
		t.Fatalf("Start error = %v, want missing mihomo after skipping gateway enable", err)
	}
}
