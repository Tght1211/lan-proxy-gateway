package gateway

import (
	"path/filepath"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/firewall"
	"github.com/tght/lan-proxy-gateway/internal/platform"
)

// fakePlatform records which Platform methods were invoked and how ip_forward
// flips, so we can assert issue #5 fix behavior.
type fakePlatform struct {
	calls     []string
	forwardOn bool
}

func (f *fakePlatform) DetectNetwork() (platform.NetworkInfo, error) {
	return platform.NetworkInfo{Interface: "eth0", IP: "10.0.0.1"}, nil
}
func (f *fakePlatform) EnableIPForward() error {
	f.calls = append(f.calls, "EnableIPForward")
	f.forwardOn = true
	return nil
}
func (f *fakePlatform) DisableIPForward() error {
	f.calls = append(f.calls, "DisableIPForward")
	f.forwardOn = false
	return nil
}
func (f *fakePlatform) IPForwardEnabled() (bool, error)   { return f.forwardOn, nil }
func (f *fakePlatform) IsAdmin() (bool, error)            { return true, nil }
func (f *fakePlatform) InstallService(string) error       { return nil }
func (f *fakePlatform) UninstallService() error           { return nil }
func (f *fakePlatform) ServiceStatus() (string, error)    { return "active", nil }
func (f *fakePlatform) SetLocalDNSToLoopback() error      { return nil }
func (f *fakePlatform) RestoreLocalDNS() error            { return nil }
func (f *fakePlatform) LocalDNSIsLoopback() (bool, error) { return false, nil }

// fakeFirewall records Apply/Remove calls.
type fakeFirewall struct {
	calls   []string
	applied firewall.Config
	report  firewall.Report
}

func (f *fakeFirewall) Apply(c firewall.Config) (firewall.Report, error) {
	f.calls = append(f.calls, "Apply")
	f.applied = c
	return f.report, nil
}
func (f *fakeFirewall) Remove() error {
	f.calls = append(f.calls, "Remove")
	return nil
}

func newGateway(t *testing.T, fp *fakePlatform, fw *fakeFirewall) *Gateway {
	t.Helper()
	dir := t.TempDir()
	g := newForTest(fp, fw)
	g.SetStatePath(filepath.Join(dir, "runtime.state"))
	return g
}

var testFwCfg = firewall.Config{RedirPort: 17892, DNSPort: 53, TCPRedirect: true, DNSHijack: true}

func contains(calls []string, want string) bool {
	for _, c := range calls {
		if c == want {
			return true
		}
	}
	return false
}

// issue #5: start 前 ip_forward 已经是 1（docker / 用户 sysctl 早就打开）。
// stop 必须保留 ip_forward=1，不能打回 0。
func TestDisable_PreservesIPForward_WhenAlreadyOnBeforeEnable(t *testing.T) {
	fp := &fakePlatform{forwardOn: true}
	fw := &fakeFirewall{}
	g := newGateway(t, fp, fw)

	if err := g.Enable(testFwCfg); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if err := g.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if contains(fp.calls, "DisableIPForward") {
		t.Fatalf("DisableIPForward must NOT be called when ip_forward was already on; got %v", fp.calls)
	}
	if !fp.forwardOn {
		t.Fatalf("ip_forward should still be on after stop")
	}
	if !contains(fw.calls, "Remove") {
		t.Fatalf("firewall Remove must run; got %v", fw.calls)
	}
}

// 互补场景：start 前 ip_forward 是 0，是我们开的；stop 时要回退到 0。
func TestDisable_RevertsIPForward_WhenWeTurnedItOn(t *testing.T) {
	fp := &fakePlatform{forwardOn: false}
	fw := &fakeFirewall{}
	g := newGateway(t, fp, fw)

	if err := g.Enable(testFwCfg); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if err := g.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if !contains(fp.calls, "DisableIPForward") {
		t.Fatalf("DisableIPForward must be called when we turned ip_forward on; got %v", fp.calls)
	}
	if fp.forwardOn {
		t.Fatalf("ip_forward should be off after stop")
	}
}

// Enable fills iface/gateway IP from detection into the firewall config.
func TestEnable_FillsFirewallConfigFromDetect(t *testing.T) {
	fp := &fakePlatform{}
	fw := &fakeFirewall{}
	g := newGateway(t, fp, fw)
	if err := g.Enable(firewall.Config{RedirPort: 17892}); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if fw.applied.Iface != "eth0" || fw.applied.GatewayIP != "10.0.0.1" {
		t.Fatalf("firewall cfg = %+v", fw.applied)
	}
}

// 多次 Enable() 幂等：第二次 Enable 不能把 WeEnabledIPForward 清掉。
func TestEnable_Idempotent_KeepsWeChangedFlag(t *testing.T) {
	fp := &fakePlatform{forwardOn: false}
	fw := &fakeFirewall{}
	g := newGateway(t, fp, fw)

	if err := g.Enable(testFwCfg); err != nil {
		t.Fatalf("first Enable: %v", err)
	}
	if err := g.Enable(testFwCfg); err != nil {
		t.Fatalf("second Enable: %v", err)
	}
	if err := g.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if !contains(fp.calls, "DisableIPForward") {
		t.Fatalf("DisableIPForward must be called after re-Enable kept the flag; got %v", fp.calls)
	}
}

// state 文件不存在时 Disable 是安全的 no-op（不动 ip_forward，不报错）。
func TestDisable_NoStateIsSafeNoop(t *testing.T) {
	fp := &fakePlatform{forwardOn: true}
	fw := &fakeFirewall{}
	g := newGateway(t, fp, fw)

	if err := g.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if contains(fp.calls, "DisableIPForward") {
		t.Fatalf("DisableIPForward must NOT run when no state file; got %v", fp.calls)
	}
	if contains(fw.calls, "Remove") {
		t.Fatalf("Remove must NOT run when nothing was applied; got %v", fw.calls)
	}
}
