//go:build linux

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func (p *impl) EnableIPForwarding() error {
	return os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644)
}

func (p *impl) DisableIPForwarding() error {
	return os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("0"), 0644)
}

func (p *impl) IsIPForwardingEnabled() (bool, error) {
	data, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(data)) == "1", nil
}

func (p *impl) DisableFirewallInterference() error {
	// No-op by default on Linux; TUN mode handles routing
	return nil
}

func (p *impl) ClearFirewallRules() error {
	return nil
}

func (p *impl) DetectDefaultInterface() (string, error) {
	// ip route show default → "default via 192.168.1.1 dev eth0 ..."
	out, err := exec.Command("ip", "route", "show", "default").Output()
	if err != nil {
		return "", err
	}
	for _, field := range strings.Fields(string(out)) {
		// The interface name follows "dev"
		if field == "dev" {
			continue
		}
	}
	// Parse properly
	fields := strings.Fields(string(out))
	for i, f := range fields {
		if f == "dev" && i+1 < len(fields) {
			return fields[i+1], nil
		}
	}
	return "", fmt.Errorf("无法检测默认网络接口")
}

func (p *impl) DetectInterfaceIP(iface string) (string, error) {
	// ip addr show dev eth0 → parse "inet 192.168.1.100/24"
	out, err := exec.Command("ip", "addr", "show", "dev", iface).Output()
	if err != nil {
		return "", fmt.Errorf("无法获取 %s 的 IP 地址: %w", iface, err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "inet ") && !strings.Contains(line, "127.0.0.1") {
			// "inet 192.168.1.100/24 brd ..."
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				ip := strings.Split(fields[1], "/")[0]
				return ip, nil
			}
		}
	}
	return "", fmt.Errorf("无法获取 %s 的 IP 地址", iface)
}

func (p *impl) DetectGateway() (string, error) {
	out, err := exec.Command("ip", "route", "show", "default").Output()
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(out))
	for i, f := range fields {
		if f == "via" && i+1 < len(fields) {
			return fields[i+1], nil
		}
	}
	return "", fmt.Errorf("无法检测网关地址")
}

func (p *impl) DetectTUNInterface() (string, error) {
	out, err := exec.Command("ip", "addr").Output()
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(out), "\n")
	for i, line := range lines {
		if strings.Contains(line, "198.18.") {
			// Walk backward to find the interface line (starts with a number)
			for j := i; j >= 0; j-- {
				if len(lines[j]) > 0 && lines[j][0] >= '0' && lines[j][0] <= '9' {
					// "3: utun0: <...>"
					parts := strings.SplitN(lines[j], ":", 3)
					if len(parts) >= 2 {
						return strings.TrimSpace(parts[1]), nil
					}
				}
			}
		}
	}
	return "", fmt.Errorf("未检测到 TUN 接口")
}
