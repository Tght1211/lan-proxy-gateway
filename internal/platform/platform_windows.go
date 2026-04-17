//go:build windows

package platform

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
)

type windowsPlatform struct{}

func current() Platform { return windowsPlatform{} }

// DetectNetwork picks the first IPv4 interface that isn't loopback as the LAN interface.
// This is approximate; users can override via config.
func (windowsPlatform) DetectNetwork() (NetworkInfo, error) {
	info := NetworkInfo{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return info, err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP.To4() == nil || ipnet.IP.IsLoopback() {
				continue
			}
			info.Interface = iface.Name
			info.IP = ipnet.IP.String()
			return info, nil
		}
	}
	return info, fmt.Errorf("unable to detect LAN interface")
}

// EnableIPForward: on Windows, this is done via `netsh` and the Routing/Remote Access
// service. We just set IPEnableRouter in the registry (HKLM). This works without
// RRAS on Server editions; on client editions the ICS service is the alternative.
func (windowsPlatform) EnableIPForward() error {
	_, err := run("reg", "add",
		`HKLM\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters`,
		"/v", "IPEnableRouter", "/t", "REG_DWORD", "/d", "1", "/f")
	return err
}

func (windowsPlatform) DisableIPForward() error {
	_, err := run("reg", "add",
		`HKLM\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters`,
		"/v", "IPEnableRouter", "/t", "REG_DWORD", "/d", "0", "/f")
	return err
}

func (windowsPlatform) IPForwardEnabled() (bool, error) {
	out, err := exec.Command("reg", "query",
		`HKLM\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters`,
		"/v", "IPEnableRouter").Output()
	if err != nil {
		return false, nil
	}
	return strings.Contains(string(out), "0x1"), nil
}

// ConfigureNAT: Windows needs ICS or netsh portproxy rules. For a v1 implementation
// we rely on mihomo TUN mode (which handles its own routing) and skip NAT setup.
func (windowsPlatform) ConfigureNAT(iface string) error   { return nil }
func (windowsPlatform) UnconfigureNAT(iface string) error { return nil }

func (windowsPlatform) ResolveMihomoPath(preferred string) (string, error) {
	if preferred != "" {
		if _, err := os.Stat(preferred); err == nil {
			return preferred, nil
		}
	}
	if p, err := exec.LookPath("mihomo.exe"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("mihomo"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("未找到 mihomo.exe，请先运行 `gateway install`")
}

// IsAdmin: try to open the MASTER service hive for write; if it succeeds we are admin.
func (windowsPlatform) IsAdmin() (bool, error) {
	// Simpler heuristic: `net session` returns 0 for admins, nonzero otherwise.
	err := exec.Command("net", "session").Run()
	return err == nil, nil
}

// InstallService creates a scheduled task (Windows SCM integration is heavy for CLI tools).
func (windowsPlatform) InstallService(binPath string) error {
	if binPath == "" {
		p, err := exec.LookPath("gateway.exe")
		if err != nil {
			return fmt.Errorf("locate gateway.exe: %w", err)
		}
		binPath = p
	}
	_, err := run("schtasks", "/Create", "/TN", "LanProxyGateway",
		"/TR", fmt.Sprintf(`"%s" start`, binPath),
		"/SC", "ONSTART", "/RU", "SYSTEM", "/RL", "HIGHEST", "/F")
	return err
}

func (windowsPlatform) UninstallService() error {
	_, _ = run("schtasks", "/Delete", "/TN", "LanProxyGateway", "/F")
	return nil
}

func (windowsPlatform) ServiceStatus() (string, error) {
	out, err := exec.Command("schtasks", "/Query", "/TN", "LanProxyGateway").Output()
	if err != nil {
		return "未安装", nil
	}
	return strings.TrimSpace(string(out)), nil
}
