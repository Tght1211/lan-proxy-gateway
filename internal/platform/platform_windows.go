//go:build windows

package platform

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type windowsPlatform struct{}

func current() Platform { return windowsPlatform{} }

// DetectNetwork picks the LAN interface by scanning net.Interfaces(). It skips
// loopback, TUN/TAP virtual adapters (mihomo, wintun, tap-*), and the CGNAT
// range 198.18.0.0/15 that mihomo hands its own TUN — otherwise once mihomo is
// running the banner flips from the real LAN IP (e.g. 192.168.12.109) to the
// TUN IP (198.18.0.1), which is useless for the "set your other devices'
// gateway to this IP" instructions.
//
// This is still approximate — users can override via config.
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
		if isVirtualAdapterName(iface.Name) {
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
			if isTunIPv4(ipnet.IP) {
				continue
			}
			info.Interface = iface.Name
			info.IP = ipnet.IP.String()
			return info, nil
		}
	}
	return info, fmt.Errorf("unable to detect LAN interface")
}

// isVirtualAdapterName matches the common Windows TUN/TAP adapter names. The
// TUN created by mihomo is typically called "Mihomo"; wintun / tap-* show up
// in OpenVPN, Tailscale, Clash Verge, etc.
func isVirtualAdapterName(name string) bool {
	n := strings.ToLower(name)
	for _, needle := range []string{"mihomo", "wintun", "tailscale", "tap-", "tap "} {
		if strings.Contains(n, needle) {
			return true
		}
	}
	return false
}

// isTunIPv4 returns true for addresses mihomo typically assigns to its TUN:
// the CGNAT block 198.18.0.0/15 (RFC 6598 benchmarking / CGNAT).
func isTunIPv4(ip net.IP) bool {
	v4 := ip.To4()
	if v4 == nil {
		return false
	}
	// 198.18.0.0/15 covers 198.18.0.0 – 198.19.255.255
	return v4[0] == 198 && (v4[1] == 18 || v4[1] == 19)
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

// PostStopCleanup no-op on windows: mihomo's TUN driver (wintun) cleans up its
// own routes via the wintun lifecycle when the adapter is destroyed.
func (windowsPlatform) PostStopCleanup() error { return nil }

func (windowsPlatform) ResolveMihomoPath(preferred string) (string, error) {
	if preferred != "" {
		if _, err := os.Stat(preferred); err == nil {
			return preferred, nil
		}
	}
	// Check the places `gateway install` actually drops the binary. Keep these
	// in sync with defaultInstallDir() in cmd/install.go.
	for _, p := range windowsMihomoCandidates() {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, nil
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

func windowsMihomoCandidates() []string {
	var out []string
	if profile := os.Getenv("USERPROFILE"); profile != "" {
		out = append(out, filepath.Join(profile, "AppData", "Local", "lan-proxy-gateway", "bin", "mihomo.exe"))
	}
	if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
		out = append(out,
			filepath.Join(localApp, "lan-proxy-gateway", "bin", "mihomo.exe"),
			filepath.Join(localApp, "mihomo", "mihomo.exe"),
		)
	}
	if progFiles := os.Getenv("ProgramFiles"); progFiles != "" {
		out = append(out, filepath.Join(progFiles, "mihomo", "mihomo.exe"))
	}
	return out
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
		"/TR", fmt.Sprintf(`"%s" start --foreground`, binPath),
		"/SC", "ONSTART", "/RU", "SYSTEM", "/RL", "HIGHEST", "/F")
	return err
}

func (windowsPlatform) UninstallService() error {
	_, _ = run("schtasks", "/Delete", "/TN", "LanProxyGateway", "/F")
	return nil
}

// 本机 DNS 切换 Windows 用 netsh。为避免踩到多个适配器的不同配置，
// 这里先不实现；用户在 guide 里能看到手动命令。
func (windowsPlatform) SetLocalDNSToLoopback() error      { return ErrNotSupported }
func (windowsPlatform) RestoreLocalDNS() error            { return ErrNotSupported }
func (windowsPlatform) LocalDNSIsLoopback() (bool, error) { return false, nil }

func (windowsPlatform) ServiceStatus() (string, error) {
	out, err := exec.Command("schtasks", "/Query", "/TN", "LanProxyGateway").Output()
	if err != nil {
		return "未安装", nil
	}
	return strings.TrimSpace(string(out)), nil
}
