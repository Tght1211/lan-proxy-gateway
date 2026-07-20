package app

import (
	"context"
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
func (p *fakePlatform) EnableIPForward() error          { return nil }
func (p *fakePlatform) DisableIPForward() error         { return nil }
func (p *fakePlatform) IPForwardEnabled() (bool, error) { return false, nil }
func (p *fakePlatform) IsAdmin() (bool, error)          { return true, nil }
func (p *fakePlatform) InstallService(string) error     { return nil }
func (p *fakePlatform) UninstallService() error         { return nil }
func (p *fakePlatform) ServiceStatus() (string, error)  { return "", nil }
func (p *fakePlatform) SetLocalDNSToLoopback() error    { return nil }
func (p *fakePlatform) RestoreLocalDNS() error {
	p.restoreCalled++
	return p.restoreErr
}
func (p *fakePlatform) LocalDNSIsLoopback() (bool, error) { return p.loopback, nil }

func TestStopRestoresLocalDNSWhenLoopback(t *testing.T) {
	plat := &fakePlatform{loopback: true}
	a := &App{Plat: plat, Cfg: config.Default()}

	if err := a.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if plat.restoreCalled != 1 {
		t.Fatalf("RestoreLocalDNS calls = %d, want 1", plat.restoreCalled)
	}
}

func TestStopSkipsLocalDNSRestoreWhenNotLoopback(t *testing.T) {
	plat := &fakePlatform{loopback: false}
	a := &App{Plat: plat, Cfg: config.Default()}

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
	a := &App{Plat: plat, Cfg: config.Default()}

	if err := a.Stop(); !errors.Is(err, restoreErr) {
		t.Fatalf("Stop error = %v, want restore error", err)
	}
}

// ---------- buildDialer / firewallConfig / dnsOptions ----------

func TestBuildDialerKinds(t *testing.T) {
	d, err := buildDialer(config.EgressConfig{Mode: config.EgressDirect})
	if err != nil || d == nil {
		t.Fatalf("direct dialer: %v", err)
	}
	for _, typ := range []string{config.ProxyTypeSOCKS5, config.ProxyTypeHTTP} {
		d, err := buildDialer(config.EgressConfig{
			Mode:  config.EgressProxy,
			Proxy: config.ProxyConfig{Type: typ, Host: "127.0.0.1", Port: 7897},
		})
		if err != nil || d == nil {
			t.Fatalf("%s dialer: %v", typ, err)
		}
	}
	if _, err := buildDialer(config.EgressConfig{
		Mode:  config.EgressProxy,
		Proxy: config.ProxyConfig{Type: "vless", Host: "h", Port: 1},
	}); err == nil {
		t.Fatal("want error for unsupported proxy type")
	}
}

func TestFirewallConfigModes(t *testing.T) {
	cfg := config.Default()
	cfg.Egress.Mode = config.EgressDirect
	fw := firewallConfig(cfg)
	if !fw.TCPRedirect || fw.QUICBlock {
		t.Fatalf("direct mode must redirect TCP without blocking QUIC: %+v", fw)
	}
	if fw.DNSHijack {
		t.Fatalf("DNS hijack must be off by default: %+v", fw)
	}
	cfg.Egress.Mode = config.EgressProxy
	fw = firewallConfig(cfg)
	if !fw.TCPRedirect || !fw.QUICBlock || fw.DNSHijack {
		t.Fatalf("proxy mode must redirect TCP and block QUIC without DNS hijack by default: %+v", fw)
	}
	cfg.DNS.Hijack = true
	fw = firewallConfig(cfg)
	if !fw.DNSHijack {
		t.Fatalf("explicit DNS hijack must be honored: %+v", fw)
	}
}

func TestDNSOptionsFakeIPOnlyInProxyMode(t *testing.T) {
	cfg := config.Default()
	opts := dnsOptions(cfg, nil)
	if opts.FakeIPEnabled {
		t.Fatal("direct mode must not fake-ip")
	}
	cfg.Egress.Mode = config.EgressProxy
	opts = dnsOptions(cfg, nil)
	if opts.FakeIPEnabled {
		t.Fatal("proxy mode must not fake-ip by default")
	}
	cfg.DNS.FakeIP = true
	opts = dnsOptions(cfg, nil)
	if !opts.FakeIPEnabled {
		t.Fatal("explicit fake_ip=true must be honored in proxy mode")
	}
}

func TestSetEgressValidates(t *testing.T) {
	a := &App{Cfg: config.Default(), Paths: config.Paths{ConfigFile: t.TempDir() + "/gateway.yaml"}}
	err := a.SetEgress(context.Background(), config.EgressConfig{
		Mode:  config.EgressProxy,
		Proxy: config.ProxyConfig{Type: "bogus", Host: "127.0.0.1", Port: 7897},
	}, false)
	if err == nil {
		t.Fatal("want validation error")
	}
	// direct mode saves without probing
	if err := a.SetEgress(context.Background(), config.EgressConfig{Mode: config.EgressDirect}, false); err != nil {
		t.Fatalf("direct SetEgress: %v", err)
	}
	if a.Cfg.Egress.Mode != config.EgressDirect {
		t.Fatalf("mode = %q", a.Cfg.Egress.Mode)
	}
	if _, err := config.LoadFrom(a.Paths.ConfigFile); err != nil {
		t.Fatalf("config should have been saved: %v", err)
	}
}
