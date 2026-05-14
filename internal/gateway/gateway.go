// Package gateway owns the main feature: turning this host into a LAN gateway.
//
// It wraps the OS-specific bits (IP forwarding, NAT, interface detection) behind
// a small high-level API so callers just say "Enable()" / "Disable()" without
// needing to know about sysctl/iptables/netsh.
package gateway

import (
	"fmt"

	"github.com/tght/lan-proxy-gateway/internal/platform"
)

// Gateway represents the LAN-gateway subsystem.
type Gateway struct {
	plat      platform.Platform
	info      platform.NetworkInfo
	statePath string // 可选；空字符串表示不写状态文件（测试或老调用方）
}

// New creates a Gateway bound to the current platform.
func New() *Gateway {
	return &Gateway{plat: platform.Current()}
}

// SetStatePath 让 app 层把 runtime.state 的位置交给 gateway。
// 状态文件用来记录我们这次 Enable() 改了什么，stop 时只回滚自己改过的部分。
func (g *Gateway) SetStatePath(path string) {
	g.statePath = path
}

// Info returns cached network info; populated by Detect().
func (g *Gateway) Info() platform.NetworkInfo { return g.info }

// Detect populates the network info (default interface, IP, router gateway).
func (g *Gateway) Detect() error {
	info, err := g.plat.DetectNetwork()
	if err != nil {
		return err
	}
	g.info = info
	return nil
}

// Enable turns on IP forwarding and, where applicable, NAT rules.
//
// 状态机：Enable() 记录两件事到 runtime.state：
//  1. NATInterface — 我们 ConfigureNAT 用的 iface，Disable() 用来反删
//  2. WeEnabledIPForward — 仅当我们把 ip_forward 从 0 改成 1 时为 true
//
// 这样多次 Enable() 也是幂等的：如果已经记录过 WeEnabledIPForward=true，
// 重新 Enable 不会因为当前值已经是 1 就把这个 flag 清掉。
func (g *Gateway) Enable() error {
	if g.info.Interface == "" {
		if err := g.Detect(); err != nil {
			return fmt.Errorf("detect network: %w", err)
		}
	}
	priorForward, _ := g.plat.IPForwardEnabled()
	existing, _ := readRuntimeState(g.statePath)

	if err := g.plat.EnableIPForward(); err != nil {
		return fmt.Errorf("enable IP forwarding: %w", err)
	}
	if err := g.plat.ConfigureNAT(g.info.Interface); err != nil {
		return fmt.Errorf("configure NAT: %w", err)
	}

	state := runtimeState{
		NATInterface:       g.info.Interface,
		WeEnabledIPForward: existing.WeEnabledIPForward || !priorForward,
	}
	_ = writeRuntimeState(g.statePath, state)
	return nil
}

// Disable is the inverse of Enable, best-effort.
//
// Order matters:
//  1. UnconfigureNAT — remove the MASQUERADE rule we added in Enable
//  2. DisableIPForward — only if state says we were the ones who flipped it
//  3. PostStopCleanup — scrub leftover TUN strict-route ip rules from
//     mihomo (Linux only, no-op elsewhere). Issue #5: mihomo killed
//     by SIGKILL leaves `pref 9000+ from all unreachable` rules behind
//     which can break Docker DNAT for non-port-preserving port mappings.
//
// issue #5 关键修复：之前 Disable() 一律 DisableIPForward() 把 ip_forward 打回 0，
// 把那些 gateway start 之前就已经在用 ip_forward=1 的 docker 用户撞断了
// （局域网访问 docker 暴露的端口立刻超时）。现在只有 state 文件里记着"是我们打开的"
// 才回退，原本就是 1 的保持 1 不动。
//
// 另一个修复：原本 Disable() 在 g.info.Interface=="" 时直接跳过 UnconfigureNAT。
// 但 `gateway stop` 是独立进程，info 一直是空的 → MASQUERADE 永远删不掉，留尾。
// 现在优先用 state 里的 NATInterface，再 fallback 到 Detect()。
func (g *Gateway) Disable() error {
	state, _ := readRuntimeState(g.statePath)

	iface := state.NATInterface
	if iface == "" && g.info.Interface != "" {
		iface = g.info.Interface
	}
	if iface == "" {
		// 最后一招：现场 Detect 一次（默认路由还在的话能拿到 iface）。
		if err := g.Detect(); err == nil {
			iface = g.info.Interface
		}
	}
	if iface != "" {
		_ = g.plat.UnconfigureNAT(iface)
	}

	var disableErr error
	if state.WeEnabledIPForward {
		disableErr = g.plat.DisableIPForward()
	}
	_ = g.plat.PostStopCleanup()
	_ = removeRuntimeState(g.statePath)
	return disableErr
}

// Status reports whether IP forwarding is currently active.
type Status struct {
	IPForward bool
	Interface string
	LocalIP   string
	Router    string
}

// Status returns the live status.
func (g *Gateway) Status() (Status, error) {
	if g.info.Interface == "" {
		_ = g.Detect()
	}
	on, err := g.plat.IPForwardEnabled()
	if err != nil {
		return Status{}, err
	}
	return Status{
		IPForward: on,
		Interface: g.info.Interface,
		LocalIP:   g.info.IP,
		Router:    g.info.Gateway,
	}, nil
}
