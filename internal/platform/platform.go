// Package platform hides the OS differences behind a single interface.
//
// Each platform file (darwin/linux) implements Platform. Callers
// (gateway layer, engine, cobra commands) never touch OS specifics directly.
package platform

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
)

// ErrNotSupported is returned by platforms that haven't implemented a feature.
var ErrNotSupported = errors.New("not supported on this platform yet")

// NetworkInfo describes the LAN-facing interface of this host.
type NetworkInfo struct {
	Interface string // e.g. "en0", "eth0"
	IP        string // e.g. "192.168.1.100"
	Gateway   string // optional; router IP
}

// Platform is the OS-specific runtime facade.
// Firewall/NAT rules are owned by internal/firewall, not this interface.
type Platform interface {
	// Network
	DetectNetwork() (NetworkInfo, error)
	EnableIPForward() error
	DisableIPForward() error
	IPForwardEnabled() (bool, error)

	// Process plumbing
	IsAdmin() (bool, error)

	// Service lifecycle
	InstallService(binPath string) error
	UninstallService() error
	ServiceStatus() (string, error)

	// 本机系统 DNS：把跑 gateway 这台电脑的系统 DNS 切到 127.0.0.1，
	// 好让本机的域名查询也走 gateway 自己的 DNS 服务。
	// 返回错误或 ErrNotSupported。
	SetLocalDNSToLoopback() error
	RestoreLocalDNS() error
	LocalDNSIsLoopback() (bool, error) // 查看当前是否已经指向 127.0.0.1
}

// Current returns the Platform for the running OS.
// Concrete implementations are in platform_{darwin,linux}.go.
func Current() Platform {
	return current()
}

// OS returns the running GOOS string for human-readable messages.
func OS() string { return runtime.GOOS }

// commandExists returns true if `name` resolves on $PATH.
func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// run executes a command, returning the combined output for diagnostics.
func run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s %v: %w: %s", name, args, err, out)
	}
	return string(out), nil
}
