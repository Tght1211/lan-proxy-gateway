// Package gateway owns the main feature: turning this host into a LAN gateway.
//
// It orchestrates IP forwarding (via internal/platform) and the firewall rule
// set (via internal/firewall) behind a small high-level API, and persists a
// runtime state file so a later `gateway stop` process only rolls back what
// we actually changed (issue #5 lesson: never slam ip_forward back to 0 if
// the user had it on before us).
package gateway

import (
	"fmt"

	"github.com/tght/lan-proxy-gateway/internal/firewall"
	"github.com/tght/lan-proxy-gateway/internal/platform"
)

// Gateway represents the LAN-gateway subsystem.
type Gateway struct {
	plat      platform.Platform
	fw        firewall.Manager
	info      platform.NetworkInfo
	statePath string // 可选；空字符串表示不写状态文件（测试）
}

// New creates a Gateway bound to the current platform and firewall backend.
func New() *Gateway {
	return &Gateway{plat: platform.Current(), fw: firewall.New()}
}

// newForTest injects fakes.
func newForTest(plat platform.Platform, fw firewall.Manager) *Gateway {
	return &Gateway{plat: plat, fw: fw}
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

// Enable turns on IP forwarding and applies the firewall rule set
// (idempotent full-sync; safe to re-run for live reconfiguration).
func (g *Gateway) Enable(fwCfg firewall.Config) error {
	if g.info.Interface == "" {
		if err := g.Detect(); err != nil {
			return fmt.Errorf("detect network: %w", err)
		}
	}
	if fwCfg.Iface == "" {
		fwCfg.Iface = g.info.Interface
	}
	if fwCfg.GatewayIP == "" {
		fwCfg.GatewayIP = g.info.IP
	}
	priorForward, _ := g.plat.IPForwardEnabled()
	existing, _ := readRuntimeState(g.statePath)

	if err := g.plat.EnableIPForward(); err != nil {
		return fmt.Errorf("enable IP forwarding: %w", err)
	}

	report, err := g.fw.Apply(fwCfg)
	if err != nil {
		return fmt.Errorf("apply firewall rules: %w", err)
	}

	state := runtimeState{
		Iface:              g.info.Interface,
		WeEnabledIPForward: existing.WeEnabledIPForward || !priorForward,
		WeEnabledPF:        existing.WeEnabledPF || report.WeEnabledPF,
		FirewallApplied:    true,
	}
	_ = writeRuntimeState(g.statePath, state)
	return nil
}

// Disable is the inverse of Enable, best-effort: remove our firewall rules,
// restore ip_forward only if we flipped it, disable pf only if we enabled it.
func (g *Gateway) Disable() error {
	state, _ := readRuntimeState(g.statePath)

	var fwErr error
	if state.FirewallApplied || state.Iface != "" {
		fwErr = g.fw.Remove()
	}

	var disableErr error
	if state.WeEnabledIPForward {
		disableErr = g.plat.DisableIPForward()
	}
	if state.WeEnabledPF {
		_ = firewall.DisablePF()
	}
	_ = removeRuntimeState(g.statePath)
	if fwErr != nil {
		return fwErr
	}
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
