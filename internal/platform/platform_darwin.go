//go:build darwin

package platform

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type darwinPlatform struct{}

func current() Platform { return darwinPlatform{} }

// DetectNetwork uses `route get default` + `ifconfig` to find the LAN interface.
func (darwinPlatform) DetectNetwork() (NetworkInfo, error) {
	info := NetworkInfo{}

	// Find default route interface and gateway.
	out, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return info, fmt.Errorf("route get default: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "interface:"):
			info.Interface = strings.TrimSpace(strings.TrimPrefix(line, "interface:"))
		case strings.HasPrefix(line, "gateway:"):
			info.Gateway = strings.TrimSpace(strings.TrimPrefix(line, "gateway:"))
		}
	}
	if info.Interface == "" {
		return info, fmt.Errorf("unable to detect default interface")
	}
	ip, err := ifaceIPv4(info.Interface)
	if err != nil {
		return info, err
	}
	info.IP = ip
	return info, nil
}

func ifaceIPv4(name string) (string, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return "", fmt.Errorf("interface %s: %w", name, err)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", fmt.Errorf("addrs %s: %w", name, err)
	}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.To4() == nil {
			continue
		}
		return ipnet.IP.String(), nil
	}
	return "", fmt.Errorf("no IPv4 address on %s", name)
}

func (darwinPlatform) EnableIPForward() error {
	_, err := run("sysctl", "-w", "net.inet.ip.forwarding=1")
	return err
}

func (darwinPlatform) DisableIPForward() error {
	_, err := run("sysctl", "-w", "net.inet.ip.forwarding=0")
	return err
}

func (darwinPlatform) IPForwardEnabled() (bool, error) {
	out, err := run("sysctl", "-n", "net.inet.ip.forwarding")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "1", nil
}

// ConfigureNAT writes a pf anchor and loads it. For the typical mihomo TUN
// setup we actually don't need NAT (TUN takes care of egress), but we keep
// this available for non-TUN modes and for parity with Linux.
func (darwinPlatform) ConfigureNAT(iface string) error {
	if iface == "" {
		return fmt.Errorf("empty interface name")
	}
	// Idempotent no-op for now: mihomo TUN handles NAT via utun interface.
	// Future: write /etc/pf.anchors/lan-proxy-gateway with:
	//   nat on <iface> from any to any -> (<iface>)
	// and `pfctl -e -f`.
	return nil
}

func (darwinPlatform) UnconfigureNAT(iface string) error { return nil }

func (darwinPlatform) ResolveMihomoPath(preferred string) (string, error) {
	if preferred != "" {
		if _, err := os.Stat(preferred); err == nil {
			return preferred, nil
		}
	}
	candidates := []string{
		"/usr/local/bin/mihomo",
		"/opt/homebrew/bin/mihomo",
		"/usr/local/bin/clash.meta",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if p, err := exec.LookPath("mihomo"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("clash-meta"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("未找到 mihomo，请先运行 `gateway install`")
}

func (darwinPlatform) IsAdmin() (bool, error) {
	return os.Geteuid() == 0, nil
}

// --- launchd service helpers ---

const launchdLabel = "com.lan-proxy-gateway"

func launchdPlistPath() string {
	return filepath.Join("/Library/LaunchDaemons", launchdLabel+".plist")
}

func (darwinPlatform) InstallService(binPath string) error {
	if binPath == "" {
		var err error
		binPath, err = exec.LookPath("gateway")
		if err != nil {
			return fmt.Errorf("locate gateway binary: %w", err)
		}
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>%s</string>
  <key>ProgramArguments</key>
  <array><string>%s</string><string>start</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>/var/log/lan-proxy-gateway.out.log</string>
  <key>StandardErrorPath</key><string>/var/log/lan-proxy-gateway.err.log</string>
</dict>
</plist>
`, launchdLabel, binPath)
	if err := os.WriteFile(launchdPlistPath(), []byte(plist), 0o644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}
	if _, err := run("launchctl", "bootstrap", "system", launchdPlistPath()); err != nil {
		// Ignore "already bootstrapped" errors idempotently.
		if !strings.Contains(err.Error(), "already loaded") {
			return err
		}
	}
	return nil
}

func (darwinPlatform) UninstallService() error {
	_, _ = run("launchctl", "bootout", "system/"+launchdLabel)
	_ = os.Remove(launchdPlistPath())
	return nil
}

func (darwinPlatform) ServiceStatus() (string, error) {
	out, err := exec.Command("launchctl", "print", "system/"+launchdLabel).CombinedOutput()
	if err != nil {
		return "未安装", nil
	}
	if strings.Contains(string(out), "state = running") {
		return "运行中", nil
	}
	return "已加载", nil
}
