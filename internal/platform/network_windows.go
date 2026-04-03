//go:build windows

package platform

import (
	"fmt"
	"net"
	"os/exec"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func (p *impl) EnableIPForwarding() error {
	return exec.Command("netsh", "int", "ipv4", "set", "global", "forwarding=enabled").Run()
}

func (p *impl) DisableIPForwarding() error {
	return exec.Command("netsh", "int", "ipv4", "set", "global", "forwarding=disabled").Run()
}

func (p *impl) IsIPForwardingEnabled() (bool, error) {
	// Use the registry to check IP forwarding — avoids parsing localized netsh output
	// which varies between English ("Enabled") and Chinese ("已启用") and can't be
	// reliably compared against UTF-8 strings when netsh outputs GBK on Chinese Windows.
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Services\Tcpip\Parameters`,
		registry.QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()
	val, _, err := k.GetIntegerValue("IPEnableRouter")
	if err != nil {
		// Key absent means forwarding is disabled
		return false, nil
	}
	return val == 1, nil
}

func (p *impl) DisableFirewallInterference() error {
	return nil
}

func (p *impl) ClearFirewallRules() error {
	return nil
}

func (p *impl) DetectDefaultInterface() (string, error) {
	// Use Go's net package for reliable cross-locale detection
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil && !ipnet.IP.IsLoopback() {
				return iface.Name, nil
			}
		}
	}
	return "", fmt.Errorf("无法检测默认网络接口")
}

func (p *impl) DetectInterfaceIP(iface string) (string, error) {
	netIface, err := net.InterfaceByName(iface)
	if err != nil {
		return "", fmt.Errorf("无法获取 %s 的 IP 地址: %w", iface, err)
	}
	addrs, err := netIface.Addrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
			return ipnet.IP.String(), nil
		}
	}
	return "", fmt.Errorf("无法获取 %s 的 IP 地址", iface)
}

func (p *impl) DetectGateway() (string, error) {
	out, err := exec.Command("cmd", "/C", "route", "print", "0.0.0.0").Output()
	if err != nil {
		return "", err
	}
	// Parse the routing table output for the default gateway
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "0.0.0.0" {
			return fields[2], nil
		}
	}
	return "", fmt.Errorf("无法检测网关地址")
}

func (p *impl) DetectTUNInterface() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if strings.HasPrefix(addr.String(), "198.18.") {
				return iface.Name, nil
			}
		}
	}
	return "", fmt.Errorf("未检测到 TUN 接口")
}
