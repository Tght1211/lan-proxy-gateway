//go:build darwin

package platform

import (
	"fmt"
	"os/exec"
	"strings"
)

func (p *impl) EnableIPForwarding() error {
	return exec.Command("sysctl", "-w", "net.inet.ip.forwarding=1").Run()
}

func (p *impl) DisableIPForwarding() error {
	return exec.Command("sysctl", "-w", "net.inet.ip.forwarding=0").Run()
}

func (p *impl) IsIPForwardingEnabled() (bool, error) {
	out, err := exec.Command("sysctl", "-n", "net.inet.ip.forwarding").Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "1", nil
}

func (p *impl) DisableFirewallInterference() error {
	// pfctl -d may fail if pf isn't loaded; that's fine
	exec.Command("pfctl", "-d").Run()
	return nil
}

func (p *impl) ClearFirewallRules() error {
	exec.Command("pfctl", "-d").Run()
	return nil
}

func (p *impl) DetectDefaultInterface() (string, error) {
	out, err := exec.Command("route", "-n", "get", "default").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "interface:") {
				iface := strings.TrimSpace(strings.TrimPrefix(line, "interface:"))
				if iface != "" {
					return iface, nil
				}
			}
		}
	}
	// Fallback: scan en0-en2 for active interface
	for _, name := range []string{"en0", "en1", "en2"} {
		out, err := exec.Command("ifconfig", name).Output()
		if err == nil && strings.Contains(string(out), "status: active") {
			return name, nil
		}
	}
	return "en0", nil
}

func (p *impl) DetectInterfaceIP(iface string) (string, error) {
	// Try ipconfig getifaddr first (macOS-specific, clean output)
	out, err := exec.Command("ipconfig", "getifaddr", iface).Output()
	if err == nil {
		ip := strings.TrimSpace(string(out))
		if ip != "" {
			return ip, nil
		}
	}
	// Fallback: parse ifconfig
	out, err = exec.Command("ifconfig", iface).Output()
	if err != nil {
		return "", fmt.Errorf("无法获取 %s 的 IP 地址", iface)
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "inet ") && !strings.Contains(line, "127.0.0.1") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[1], nil
			}
		}
	}
	return "", fmt.Errorf("无法获取 %s 的 IP 地址", iface)
}

func (p *impl) DetectGateway() (string, error) {
	out, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "gateway:") {
			gw := strings.TrimSpace(strings.TrimPrefix(line, "gateway:"))
			if gw != "" {
				return gw, nil
			}
		}
	}
	return "", fmt.Errorf("无法检测网关地址")
}

func (p *impl) DetectTUNInterface() (string, error) {
	out, err := exec.Command("ifconfig").Output()
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(out), "\n")
	for i, line := range lines {
		if strings.Contains(line, "inet 198.18") {
			// The interface name is on the line before or the current block header
			for j := i; j >= 0; j-- {
				if len(lines[j]) > 0 && lines[j][0] != '\t' && lines[j][0] != ' ' {
					name := strings.Split(lines[j], ":")[0]
					return name, nil
				}
			}
		}
	}
	return "", fmt.Errorf("未检测到 TUN 接口")
}
