package gateway

import (
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/platform"
)

// fakePlatform records which Platform methods were invoked. Lets us assert
// that Gateway.Disable() actually calls PostStopCleanup() (issue #5 wiring).
type fakePlatform struct {
	calls []string
	// 让 DisableIPForward 可以注入错误，验证 PostStopCleanup 仍然被调用
	disableForwardErr error
}

func (f *fakePlatform) DetectNetwork() (platform.NetworkInfo, error) {
	return platform.NetworkInfo{Interface: "eth0", IP: "10.0.0.1"}, nil
}
func (f *fakePlatform) EnableIPForward() error {
	f.calls = append(f.calls, "EnableIPForward")
	return nil
}
func (f *fakePlatform) DisableIPForward() error {
	f.calls = append(f.calls, "DisableIPForward")
	return f.disableForwardErr
}
func (f *fakePlatform) IPForwardEnabled() (bool, error) { return true, nil }
func (f *fakePlatform) ConfigureNAT(iface string) error {
	f.calls = append(f.calls, "ConfigureNAT")
	return nil
}
func (f *fakePlatform) UnconfigureNAT(iface string) error {
	f.calls = append(f.calls, "UnconfigureNAT")
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

// 关键回归断言 (issue #5)：Gateway.Disable() 必须调用 PostStopCleanup，
// 顺序还要在 UnconfigureNAT / DisableIPForward 之后 —— 这样 mihomo 已经死、
// NAT 已经解、最后才扫剩余 ip rule。乱序会让清理逻辑试图删 mihomo 还活着
// 时仍在用的 rule，被拒。
func TestGatewayDisable_CallsPostStopCleanupAfterNATAndForward(t *testing.T) {
	fp := &fakePlatform{}
	g := &Gateway{plat: fp, info: platform.NetworkInfo{Interface: "eth0"}}
	if err := g.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	want := []string{"UnconfigureNAT", "DisableIPForward", "PostStopCleanup"}
	if !equalStringSlices(fp.calls, want) {
		t.Fatalf("call order wrong:\n got = %v\nwant = %v", fp.calls, want)
	}
}

// PostStopCleanup 的失败不该阻止 Disable 返回 —— 这是 best-effort 清理，
// 主路径要保留 DisableIPForward 的错误（如果 sysctl 写入失败）作为返回值。
func TestGatewayDisable_PostStopCleanupRunsEvenWhenInterfaceMissing(t *testing.T) {
	fp := &fakePlatform{}
	// info.Interface == "" → UnconfigureNAT 被跳过，但 PostStopCleanup 仍要跑
	g := &Gateway{plat: fp, info: platform.NetworkInfo{}}
	_ = g.Disable()
	hasCleanup := false
	for _, c := range fp.calls {
		if c == "PostStopCleanup" {
			hasCleanup = true
		}
		if c == "UnconfigureNAT" {
			t.Fatalf("UnconfigureNAT must be skipped when Interface is empty")
		}
	}
	if !hasCleanup {
		t.Fatalf("PostStopCleanup must run even without an interface; got calls: %v", fp.calls)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
