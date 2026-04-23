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
  <array><string>%s</string><string>start</string><string>--foreground</string></array>
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
	// launchd 经常残留「上一次 bootstrap 半失败但 label 已注册」的状态，
	// 不先 bootout 的话再次 bootstrap 会返回 exit 5 / Input/output error。
	// bootout 在未注册时返回非零，吞掉。
	_, _ = run("launchctl", "bootout", "system/"+launchdLabel)
	if _, err := run("launchctl", "bootstrap", "system", launchdPlistPath()); err != nil {
		if strings.Contains(err.Error(), "already loaded") ||
			strings.Contains(err.Error(), "service already bootstrapped") {
			return nil
		}
		return fmt.Errorf("launchctl bootstrap: %w", err)
	}
	// 如果之前被加进 disabled 列表（launchctl disable），bootstrap 成功但 launchd 不会拉起。
	// enable 兜一手，未 disable 时是 no-op。
	_, _ = run("launchctl", "enable", "system/"+launchdLabel)
	return nil
}

func (darwinPlatform) UninstallService() error {
	_, _ = run("launchctl", "bootout", "system/"+launchdLabel)
	_ = os.Remove(launchdPlistPath())
	return nil
}

// --- 本机系统 DNS 切换（macOS）---
//
// 通过 networksetup 改**当前所有活跃服务**的 DNS。活跃服务指在 `networksetup
// -listnetworkserviceorder` 结果中、有设备名且不是 "disabled" 的那些
// （Wi-Fi / Ethernet 等）。用户如果连多个接口（比如笔记本插网线又开 Wi-Fi）
// 两个都会改到，保证一定能走到 mihomo。

func activeNetworkServices() ([]string, error) {
	out, err := exec.Command("networksetup", "-listallnetworkservices").Output()
	if err != nil {
		return nil, err
	}
	var services []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// 跳过头部提示 + 带星号（disabled）的服务
		if line == "" || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "An asterisk") {
			continue
		}
		services = append(services, line)
	}
	return services, nil
}

func (darwinPlatform) SetLocalDNSToLoopback() error {
	services, err := activeNetworkServices()
	if err != nil {
		return fmt.Errorf("列网络服务: %w", err)
	}
	if len(services) == 0 {
		return fmt.Errorf("没有活跃网络服务")
	}
	var firstErr error
	for _, svc := range services {
		if _, err := run("networksetup", "-setdnsservers", svc, "127.0.0.1"); err != nil && firstErr == nil {
			firstErr = err
		}
		// 方式 3（DNS 劫持）和系统 HTTP/SOCKS 代理并存时，系统代理会抢先把
		// 流量直送到代理端口，DNS fake-ip 这条路被架空。切 DNS 的同时把三类
		// 系统代理 state 关掉（只关 state，不动 server/port，用户下次自己再开）。
		for _, flag := range []string{"-setwebproxystate", "-setsecurewebproxystate", "-setsocksfirewallproxystate"} {
			_, _ = run("networksetup", flag, svc, "off")
		}
	}
	// 清 DNS 缓存立即生效（非致命，失败忽略）
	_, _ = run("dscacheutil", "-flushcache")
	_, _ = run("killall", "-HUP", "mDNSResponder")
	return firstErr
}

func (darwinPlatform) RestoreLocalDNS() error {
	services, err := activeNetworkServices()
	if err != nil {
		return fmt.Errorf("列网络服务: %w", err)
	}
	var firstErr error
	for _, svc := range services {
		// "empty" 让服务恢复成由 DHCP 提供的 DNS
		if _, err := run("networksetup", "-setdnsservers", svc, "empty"); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	_, _ = run("dscacheutil", "-flushcache")
	_, _ = run("killall", "-HUP", "mDNSResponder")
	return firstErr
}

func (darwinPlatform) LocalDNSIsLoopback() (bool, error) {
	services, err := activeNetworkServices()
	if err != nil {
		return false, err
	}
	for _, svc := range services {
		out, err := exec.Command("networksetup", "-getdnsservers", svc).Output()
		if err != nil {
			continue
		}
		if strings.Contains(string(out), "127.0.0.1") {
			return true, nil
		}
	}
	return false, nil
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
