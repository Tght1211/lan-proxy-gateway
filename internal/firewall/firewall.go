// Package firewall owns the OS-level rules that steer LAN traffic into the
// gateway: iptables on Linux, pf anchors on macOS. Apply is a full-sync
// operation — it converges the OS to exactly the desired rule set, tagged
// with a comment so teardown is precise.
package firewall

import "errors"

// CommentTag marks every rule we install (iptables -m comment, pf comment),
// so discovery and teardown never touch foreign rules.
const CommentTag = "lan-proxy-gateway"

// ErrNotSupported is returned on platforms without a firewall backend.
var ErrNotSupported = errors.New("当前平台不支持防火墙规则管理")

// Config is the desired rule set.
type Config struct {
	Iface          string // LAN interface, e.g. "eth0" / "en0"
	GatewayIP      string // this host's LAN IP (excluded from redirect on macOS)
	RedirPort      int    // transparent TCP relay port
	UDPRedirPort   int    // UDP fake-IP relay port (0 = disabled)
	FakeIPRange    string // fake-IP prefix for UDP redirect, e.g. "198.18.0.0/16"
	DNSPort        int    // DNS server port
	TCPRedirect    bool   // REDIRECT all LAN TCP into the relay
	UDPFakeIPRedir bool   // REDIRECT LAN UDP destined for fake-IP range into UDP relay
	DNSHijack      bool   // REDIRECT any LAN UDP/TCP :53 into our DNS
	QUICBlock      bool   // REJECT LAN UDP/443 so clients fall back to TCP
}

// Report tells the caller what Apply changed about global state, for
// persistence in runtime.state (issue-#5-style safe rollback).
type Report struct {
	WeEnabledPF bool // macOS: pf was disabled and we turned it on
}

// Manager installs and removes the rule set.
type Manager interface {
	// Apply converges the OS to cfg (idempotent; adds missing, removes stale ours).
	Apply(cfg Config) (Report, error)
	// Remove deletes every rule tagged ours (idempotent, safe after crashes).
	Remove() error
}

// New returns the platform firewall manager.
func New() Manager { return newPlatformManager() }
