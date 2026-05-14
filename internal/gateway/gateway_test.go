package gateway

import (
	"path/filepath"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/platform"
)

// fakePlatform records which Platform methods were invoked and how ip_forward
// flips, so we can assert issue #5 fix behavior.
type fakePlatform struct {
	calls []string
	// 让 DisableIPForward 可以注入错误，验证 PostStopCleanup 仍然被调用
	disableForwardErr error
	// 模拟当前 ip_forward 的状态：start 前 false 表示用户原本是 0；true 表示原本就是 1
	// （docker / systemd-sysctl 已经打开）
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
	return f.disableForwardErr
}
func (f *fakePlatform) IPForwardEnabled() (bool, error) { return f.forwardOn, nil }
func (f *fakePlatform) ConfigureNAT(iface string) error {
	f.calls = append(f.calls, "ConfigureNAT:"+iface)
	return nil
}
func (f *fakePlatform) UnconfigureNAT(iface string) error {
	f.calls = append(f.calls, "UnconfigureNAT:"+iface)
	return nil
}
func (f *fakePlatform) PostStopCleanup() error {
	f.calls = append(f.calls, "PostStopCleanup")
	return nil
}
func (f *fakePlatform) ResolveMihomoPath(string) (string, error) { return "/bin/mihomo", nil }
func (f *fakePlatform) IsAdmin() (bool, error)                   { return true, nil }
func (f *fakePlatform) InstallService(string) error              { return nil }
func (f *fakePlatform) UninstallService() error                  { return nil }
func (f *fakePlatform) ServiceStatus() (string, error)           { return "active", nil }
func (f *fakePlatform) SetLocalDNSToLoopback() error             { return nil }
func (f *fakePlatform) RestoreLocalDNS() error                   { return nil }
func (f *fakePlatform) LocalDNSIsLoopback() (bool, error)        { return false, nil }

func newGateway(t *testing.T, fp *fakePlatform) *Gateway {
	t.Helper()
	dir := t.TempDir()
	g := &Gateway{plat: fp}
	g.SetStatePath(filepath.Join(dir, "runtime.state"))
	return g
}

func contains(calls []string, want string) bool {
	for _, c := range calls {
		if c == want {
			return true
		}
	}
	return false
}

// issue #5 (主因): start 前 ip_forward 已经是 1（docker / 用户 sysctl 早就打开）。
// stop 必须保留 ip_forward=1，不能打回 0；否则 docker 的 LAN 访问立刻断。
func TestDisable_PreservesIPForward_WhenAlreadyOnBeforeEnable(t *testing.T) {
	fp := &fakePlatform{forwardOn: true} // 用户/docker 早就开了
	g := newGateway(t, fp)

	if err := g.Enable(); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if err := g.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if contains(fp.calls, "DisableIPForward") {
		t.Fatalf("DisableIPForward must NOT be called when ip_forward was already on; got %v", fp.calls)
	}
	if !fp.forwardOn {
		t.Fatalf("ip_forward should still be on after stop, but is off")
	}
}

// 互补场景：start 前 ip_forward 是 0，是我们开的；stop 时要回退到 0。
func TestDisable_RevertsIPForward_WhenWeTurnedItOn(t *testing.T) {
	fp := &fakePlatform{forwardOn: false}
	g := newGateway(t, fp)

	if err := g.Enable(); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if err := g.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if !contains(fp.calls, "DisableIPForward") {
		t.Fatalf("DisableIPForward must be called when we turned ip_forward on; got %v", fp.calls)
	}
	if fp.forwardOn {
		t.Fatalf("ip_forward should be off after stop, but is still on")
	}
}

// issue #5 (副因): `gateway stop` 是独立进程，内存里 info.Interface 是空的，
// 但 state 文件里写过 NATInterface。Disable 必须用 state 里的 iface 反删 MASQUERADE。
func TestDisable_UsesNATInterfaceFromState_OnFreshProcess(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "runtime.state")

	// 第一个进程：Enable 写下 state
	fp1 := &fakePlatform{forwardOn: false}
	g1 := &Gateway{plat: fp1}
	g1.SetStatePath(statePath)
	if err := g1.Enable(); err != nil {
		t.Fatalf("Enable: %v", err)
	}

	// 模拟新进程：另起一个 Gateway，没经过 Enable，info 是空的
	fp2 := &fakePlatform{forwardOn: true}
	g2 := &Gateway{plat: fp2}
	g2.SetStatePath(statePath)
	if err := g2.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if !contains(fp2.calls, "UnconfigureNAT:eth0") {
		t.Fatalf("UnconfigureNAT must run with iface from state on fresh process; got %v", fp2.calls)
	}
}

// 关键回归断言 (issue #5)：Gateway.Disable() 必须调用 PostStopCleanup。
// PostStopCleanup 是 best-effort 但必须跑。
func TestGatewayDisable_AlwaysCallsPostStopCleanup(t *testing.T) {
	fp := &fakePlatform{forwardOn: false}
	g := newGateway(t, fp)
	if err := g.Enable(); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if err := g.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if !contains(fp.calls, "PostStopCleanup") {
		t.Fatalf("PostStopCleanup must run; got %v", fp.calls)
	}
}

// 即使 state 文件不存在（用户在旧版 gateway 跑过、没写过 state，然后升级），
// Disable 也必须降级到 Detect()，把 MASQUERADE 删掉。
func TestDisable_FallsBackToDetect_WhenNoState(t *testing.T) {
	fp := &fakePlatform{forwardOn: true} // 假设别人开的，我们不要乱关
	g := newGateway(t, fp)               // state 文件不存在

	if err := g.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if !contains(fp.calls, "UnconfigureNAT:eth0") {
		t.Fatalf("UnconfigureNAT must run via Detect fallback; got %v", fp.calls)
	}
	// 没 state 表示我们不知道是不是自己开的 → 安全起见不动 ip_forward
	if contains(fp.calls, "DisableIPForward") {
		t.Fatalf("DisableIPForward must NOT run when no state file; got %v", fp.calls)
	}
}

// 多次 Enable() 幂等：第二次 Enable 不能因为 ip_forward 已经是 1 就把
// WeEnabledIPForward 清掉 —— 否则 stop 会保留 ip_forward 不回退，破坏第一次的语义。
func TestEnable_Idempotent_KeepsWeChangedFlag(t *testing.T) {
	fp := &fakePlatform{forwardOn: false}
	g := newGateway(t, fp)

	if err := g.Enable(); err != nil {
		t.Fatalf("first Enable: %v", err)
	}
	// 第二次 Enable，此时 forwardOn 已经是 true
	if err := g.Enable(); err != nil {
		t.Fatalf("second Enable: %v", err)
	}
	if err := g.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if !contains(fp.calls, "DisableIPForward") {
		t.Fatalf("DisableIPForward must be called after re-Enable kept the flag; got %v", fp.calls)
	}
}
