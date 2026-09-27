// Package hotspot discovers an existing macOS Internet Sharing network.
// It never creates networks, changes routes/DNS, or runs a DHCP server.
package hotspot

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

type Status struct {
	Stage             string `json:"stage,omitempty"`
	SharingConfigured bool   `json:"sharing_configured"`
	WiFiPowerKnown    bool   `json:"wifi_power_known"`
	WiFiPowered       bool   `json:"wifi_powered"`

	Supported bool     `json:"supported"`
	Available bool     `json:"available"`
	Enabled   bool     `json:"enabled"`
	Applied   bool     `json:"applied"`
	Message   string   `json:"message"`
	Interface string   `json:"interface"`
	IP        string   `json:"ip"`
	CIDR      string   `json:"cidr"`
	Uplink    string   `json:"uplink"`
	Members   []string `json:"members,omitempty"`
}

// Inspect is pure so discovery can be checked against real ifconfig fixtures.
func Inspect(route, hardware, interfaces string, sharing bool) Status {
	s := Status{Supported: true}
	for _, line := range strings.Split(route, "\n") {
		if key, value, ok := strings.Cut(strings.TrimSpace(line), ":"); ok && key == "interface" {
			s.Uplink = strings.TrimSpace(value)
		}
	}
	wifi := map[string]bool{}
	wired := map[string]bool{}
	port := ""
	for _, line := range strings.Split(hardware, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Hardware Port:") {
			port = strings.TrimSpace(strings.TrimPrefix(line, "Hardware Port:"))
		}
		if strings.HasPrefix(line, "Device:") {
			device := strings.TrimSpace(strings.TrimPrefix(line, "Device:"))
			wifi[device] = port == "Wi-Fi" || port == "AirPort"
			wired[device] = strings.Contains(port, "Ethernet") || strings.Contains(port, "LAN")
		}
	}
	if !wired[s.Uplink] {
		s.Stage = "wired_required"
		s.Message = "请让 Mac 通过网线上网，再开启互联网共享；当前默认出口不是可识别的以太网接口。"
		return s
	}
	if !sharing {
		s.Stage = "sharing_off"
		s.Message = "请在系统设置 → 通用 → 共享中开启「互联网共享」：从以太网共享给 Wi-Fi。"
		return s
	}
	type iface struct {
		name    string
		up      bool
		prefix  netip.Prefix
		members []string
	}
	var all []*iface
	var current *iface
	for _, line := range strings.Split(interfaces, "\n") {
		if line != "" && line[0] != ' ' && line[0] != '\t' {
			name, _, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			current = &iface{name: name, up: strings.Contains(line, "<UP,")}
			all = append(all, current)
		}
		if current == nil {
			continue
		}
		f := strings.Fields(line)
		if len(f) >= 2 && f[0] == "member:" {
			current.members = append(current.members, f[1])
		}
		if len(f) >= 4 && f[0] == "inet" && f[2] == "netmask" {
			ip, err := netip.ParseAddr(f[1])
			if err != nil || !ip.Is4() {
				continue
			}
			mask, err := strconv.ParseUint(strings.TrimPrefix(f[3], "0x"), 16, 32)
			if err != nil {
				continue
			}
			ones, bits := net.IPv4Mask(byte(mask>>24), byte(mask>>16), byte(mask>>8), byte(mask)).Size()
			if bits == 32 && ones > 0 && ones < 31 {
				current.prefix = netip.PrefixFrom(ip, ones)
			}
		}
	}
	var candidates []*iface
	for _, n := range all {
		if !strings.HasPrefix(n.name, "bridge") || !n.up || !n.prefix.IsValid() || !n.prefix.Addr().IsPrivate() {
			continue
		}
		wireless := false
		for _, m := range n.members {
			if wifi[m] || isAP(m) {
				wireless = true
			}
		}
		if wireless {
			candidates = append(candidates, n)
		}
	}
	if len(candidates) == 0 {
		s.Stage = "waiting_network"
		s.Message = "共享服务尚未建立 Wi-Fi 网络，请确认共享给 Wi-Fi，并等待几秒后重试。"
		return s
	}
	if len(candidates) != 1 {
		s.Stage = "ambiguous_network"
		s.Message = "尚未找到唯一的 Wi-Fi 共享网络，请确认共享目标是 Wi-Fi，并等待几秒后重试。"
		return s
	}
	n := candidates[0]
	for _, member := range n.members {
		if member == s.Uplink || (!wifi[member] && !isAP(member)) {
			s.Stage = "mixed_network"
			s.Message = "共享网络还包含其他有线接口，请在系统互联网共享中仅选择 Wi-Fi。"
			return s
		}
	}
	for _, other := range all {
		if other.name == n.name || !other.up || !other.prefix.IsValid() {
			continue
		}
		member := false
		for _, m := range n.members {
			if m == other.name {
				member = true
			}
		}
		if !member && n.prefix.Overlaps(other.prefix) {
			s.Stage = "overlap"
			s.Message = fmt.Sprintf("共享网络与 %s 地址范围重叠，请先调整系统共享网络。", other.name)
			return s
		}
	}
	s.Stage = "ready"
	s.Available = true
	s.Interface, s.IP, s.CIDR, s.Members = n.name, n.prefix.Addr().String(), n.prefix.Masked().String(), n.members
	s.Message = "已检测到 Wi-Fi 共享，可以连接游戏机。"
	return s
}

func isAP(name string) bool {
	if !strings.HasPrefix(name, "ap") {
		return false
	}
	_, err := strconv.ParseUint(strings.TrimPrefix(name, "ap"), 10, 16)
	return err == nil
}

// Readiness combines saved configuration, process state and live network
// evidence. A saved switch or a daemon name alone never makes a hotspot ready.
type Readiness struct {
	SharingConfigured bool
	SharingRunning    bool
	WiFiPowerKnown    bool
	WiFiPowered       bool
}

func InspectReadiness(route, hardware, interfaces string, state Readiness) Status {
	s := Inspect(route, hardware, interfaces, state.SharingConfigured || state.SharingRunning)
	s.SharingConfigured = state.SharingConfigured
	s.WiFiPowerKnown, s.WiFiPowered = state.WiFiPowerKnown, state.WiFiPowered
	// Preserve concrete conflict errors and live bridge evidence. macOS can
	// report the client Wi-Fi radio off while a separate AP interface is active.
	if s.Stage != "sharing_off" && s.Stage != "waiting_network" {
		return s
	}
	if state.WiFiPowerKnown && !state.WiFiPowered {
		s.Stage = "wifi_off"
		s.Message = "Mac 的 Wi-Fi 已关闭，热点尚未启动。请先打开 Mac 的 Wi-Fi，再确认互联网共享已开启；Mac 仍可通过网线上网。"
	} else if state.SharingConfigured && !state.SharingRunning {
		s.Stage = "configured_not_running"
		s.Message = "共享配置已保存，但热点尚未启动。请在系统设置依次点击「好」「完成」，确认互联网共享开关已打开；仍无热点时，关闭共享后重新开启。"
	}
	return s
}

func wifiDevice(hardware string) string {
	port := ""
	for _, line := range strings.Split(hardware, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Hardware Port:") {
			port = strings.TrimSpace(strings.TrimPrefix(line, "Hardware Port:"))
		}
		if strings.HasPrefix(line, "Device:") && (port == "Wi-Fi" || port == "AirPort") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Device:"))
		}
	}
	return ""
}

func wifiPower(output string) (known, powered bool) {
	_, value, ok := strings.Cut(strings.TrimSpace(output), ":")
	if !ok {
		return false, false
	}
	switch strings.TrimSpace(value) {
	case "On":
		return true, true
	case "Off":
		return true, false
	default:
		return false, false
	}
}
