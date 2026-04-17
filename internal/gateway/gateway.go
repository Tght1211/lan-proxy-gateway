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
	plat platform.Platform
	info platform.NetworkInfo
}

// New creates a Gateway bound to the current platform.
func New() *Gateway {
	return &Gateway{plat: platform.Current()}
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
func (g *Gateway) Enable() error {
	if g.info.Interface == "" {
		if err := g.Detect(); err != nil {
			return fmt.Errorf("detect network: %w", err)
		}
	}
	if err := g.plat.EnableIPForward(); err != nil {
		return fmt.Errorf("enable IP forwarding: %w", err)
	}
	if err := g.plat.ConfigureNAT(g.info.Interface); err != nil {
		return fmt.Errorf("configure NAT: %w", err)
	}
	return nil
}

// Disable is the inverse of Enable, best-effort.
func (g *Gateway) Disable() error {
	if g.info.Interface != "" {
		_ = g.plat.UnconfigureNAT(g.info.Interface)
	}
	return g.plat.DisableIPForward()
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
