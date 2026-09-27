package gateway

import (
	"fmt"

	"github.com/tght/lan-proxy-gateway/internal/firewall"
	"github.com/tght/lan-proxy-gateway/internal/hotspot"
	"github.com/tght/lan-proxy-gateway/internal/platform"
)

func (g *Gateway) discoverHotspot() hotspot.Status {
	if g.detectHotspot != nil {
		return g.detectHotspot()
	}
	return hotspot.Detect()
}

// Called under g.mu. macOS owns DHCP, NAT, IP forwarding and PF enablement.
func (g *Gateway) enableHotspot(cfg firewall.Config) error {
	s := g.discoverHotspot()
	s.Enabled = true
	g.hotspotStatus = s
	if !s.Available {
		// A missing hotspot is a waiting state, never permission to capture Ethernet.
		state, err := readRuntimeState(g.statePath)
		if err != nil {
			return err
		}
		if state.FirewallApplied {
			if err := g.fw.Remove(); err != nil {
				return err
			}
			state.FirewallApplied = false
			if err := writeRuntimeState(g.statePath, state); err != nil {
				return err
			}
		}
		return nil
	}
	fail := func(err error) error {
		// Stop steering clients into a partially configured service. Never
		// touch Internet Sharing's own anchors or global enablement.
		if cleanup := g.fw.Remove(); cleanup != nil {
			err = fmt.Errorf("%w；清理热点规则失败: %v", err, cleanup)
		}
		g.hotspotStatus.Stage = "apply_failed"
		g.hotspotStatus.Message = err.Error()
		return err
	}
	forwarding, err := g.plat.IPForwardEnabled()
	if err != nil || !forwarding {
		return fail(fmt.Errorf("互联网共享尚未准备好，请关闭后重新开启系统共享"))
	}
	cfg.Iface, cfg.GatewayIP = s.Interface, s.IP
	cfg.CaptureInterfaces = append([]string{s.Interface}, s.Members...)
	cfg.LANCIDRs = []string{s.CIDR}
	cfg.DNSLocalRedirect = true
	cfg.DNSHijack = true
	cfg.IPv6Block = true
	// Persist ownership before installing rules so failure cleanup is possible.
	state := runtimeState{Iface: s.Interface, PreserveGlobals: true, FirewallApplied: true}
	if err := writeRuntimeState(g.statePath, state); err != nil {
		return fail(err)
	}
	if _, err := g.fw.Apply(cfg); err != nil {
		return fail(err)
	}
	g.info = platform.NetworkInfo{Interface: s.Interface, IP: s.IP}
	g.hotspotStatus.Applied = true
	g.hotspotStatus.Message = "热点流量接管已开启，请在游戏机上测试连接。"
	return nil
}
