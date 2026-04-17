//go:build linux

package platform

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
)

type linuxPlatform struct{}

func current() Platform { return linuxPlatform{} }

func (linuxPlatform) DetectNetwork() (NetworkInfo, error) {
	info := NetworkInfo{}
	out, err := exec.Command("ip", "route", "show", "default").Output()
	if err != nil {
		return info, fmt.Errorf("ip route: %w", err)
	}
	fields := strings.Fields(string(out))
	for i, f := range fields {
		if f == "dev" && i+1 < len(fields) {
			info.Interface = fields[i+1]
		}
		if f == "via" && i+1 < len(fields) {
			info.Gateway = fields[i+1]
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
		return "", err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
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

func (linuxPlatform) EnableIPForward() error {
	return os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0o644)
}

func (linuxPlatform) DisableIPForward() error {
	return os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("0"), 0o644)
}

func (linuxPlatform) IPForwardEnabled() (bool, error) {
	data, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(data)) == "1", nil
}

// ConfigureNAT adds an iptables MASQUERADE rule idempotently.
func (linuxPlatform) ConfigureNAT(iface string) error {
	if iface == "" {
		return fmt.Errorf("empty interface name")
	}
	if !commandExists("iptables") {
		return fmt.Errorf("iptables 未安装")
	}
	// Check if rule already exists.
	check := exec.Command("iptables", "-t", "nat", "-C", "POSTROUTING", "-o", iface, "-j", "MASQUERADE")
	if err := check.Run(); err == nil {
		return nil // already present
	}
	_, err := run("iptables", "-t", "nat", "-A", "POSTROUTING", "-o", iface, "-j", "MASQUERADE")
	return err
}

func (linuxPlatform) UnconfigureNAT(iface string) error {
	if iface == "" {
		return nil
	}
	_, _ = run("iptables", "-t", "nat", "-D", "POSTROUTING", "-o", iface, "-j", "MASQUERADE")
	return nil
}

func (linuxPlatform) ResolveMihomoPath(preferred string) (string, error) {
	if preferred != "" {
		if _, err := os.Stat(preferred); err == nil {
			return preferred, nil
		}
	}
	for _, p := range []string{"/usr/local/bin/mihomo", "/usr/bin/mihomo"} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if p, err := exec.LookPath("mihomo"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("未找到 mihomo，请先运行 `gateway install`")
}

func (linuxPlatform) IsAdmin() (bool, error) {
	return os.Geteuid() == 0, nil
}

const systemdUnit = "lan-proxy-gateway.service"
const systemdPath = "/etc/systemd/system/lan-proxy-gateway.service"

func (linuxPlatform) InstallService(binPath string) error {
	if binPath == "" {
		var err error
		binPath, err = exec.LookPath("gateway")
		if err != nil {
			return fmt.Errorf("locate gateway binary: %w", err)
		}
	}
	unit := fmt.Sprintf(`[Unit]
Description=LAN Proxy Gateway
After=network.target

[Service]
Type=simple
ExecStart=%s start
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`, binPath)
	if err := os.WriteFile(systemdPath, []byte(unit), 0o644); err != nil {
		return err
	}
	if _, err := run("systemctl", "daemon-reload"); err != nil {
		return err
	}
	_, err := run("systemctl", "enable", systemdUnit)
	return err
}

func (linuxPlatform) UninstallService() error {
	_, _ = run("systemctl", "disable", "--now", systemdUnit)
	_ = os.Remove(systemdPath)
	_, _ = run("systemctl", "daemon-reload")
	return nil
}

func (linuxPlatform) ServiceStatus() (string, error) {
	out, _ := exec.Command("systemctl", "is-active", systemdUnit).Output()
	s := strings.TrimSpace(string(out))
	if s == "" {
		return "未安装", nil
	}
	return s, nil
}
