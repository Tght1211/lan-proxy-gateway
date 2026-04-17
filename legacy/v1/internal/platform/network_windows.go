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
	if route, err := currentWindowsDefaultRoute(); err == nil {
		if iface, err := detectWindowsInterfaceByIP(route.InterfaceIP); err == nil {
			return iface, nil
		}
	}

	// Fallback: pick the first active IPv4 interface if route parsing fails.
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
	route, err := currentWindowsDefaultRoute()
	if err == nil {
		return route.Gateway, nil
	}
	return "", err
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

func currentWindowsDefaultRoute() (windowsDefaultRoute, error) {
	outputs := [][]string{
		{"route", "print", "-4", "0.0.0.0"},
		{"cmd", "/C", "route", "print", "0.0.0.0"},
	}
	for _, args := range outputs {
		out, err := exec.Command(args[0], args[1:]...).Output()
		if err != nil {
			continue
		}
		if route, ok := parseWindowsDefaultRoute(string(out)); ok {
			return route, nil
		}
	}
	return windowsDefaultRoute{}, fmt.Errorf("无法检测默认路由")
}

func detectWindowsInterfaceByIP(targetIP string) (string, error) {
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
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil && ipnet.IP.String() == targetIP {
				return iface.Name, nil
			}
		}
	}
	return "", fmt.Errorf("无法根据 %s 匹配网络接口", targetIP)
}
