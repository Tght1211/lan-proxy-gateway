package gateway

import (
	"bufio"
	"fmt"
	"net"
	"net/netip"
	"os/exec"
	"runtime"
	"strings"
)

// LANDevice represents a device found on the LAN via ARP table inspection.
type LANDevice struct {
	IP        string `json:"ip"`
	MAC       string `json:"mac"`
	Interface string `json:"interface,omitempty"`
}

// SubnetInfo describes the gateway's LAN subnet.
type SubnetInfo struct {
	CIDR       string `json:"cidr"`       // e.g. "192.168.1.0/24"
	PrefixLen  int    `json:"prefix_len"` // e.g. 24
	Netmask    string `json:"netmask"`    // e.g. "255.255.255.0"
	GatewayIP  string `json:"gateway_ip"`
	LocalIP    string `json:"local_ip"`
	Broadcast  string `json:"broadcast,omitempty"`
	UsableFrom string `json:"usable_from"` // first host address
	UsableTo   string `json:"usable_to"`   // last host address
}

// DeviceOnboarding bundles all device-onboarding helper info for the API.
type DeviceOnboarding struct {
	Subnet       SubnetInfo  `json:"subnet"`
	OnlineDevs   []LANDevice `json:"online_devices"`
	RecommendIPs []string    `json:"recommend_ips"` // safe unoccupied IPs
}

// DeviceOnboardingInfo gathers subnet info, scans for online devices, and
// recommends available IPs for new device onboarding.
func (g *Gateway) DeviceOnboardingInfo() (*DeviceOnboarding, error) {
	info := g.Info()
	if info.Interface == "" {
		if err := g.Detect(); err != nil {
			return nil, err
		}
	}

	info = g.Info()
	subnet, err := detectSubnet(info.Interface, info.IP)
	if err != nil {
		return nil, fmt.Errorf("detect subnet: %w", err)
	}
	subnet.GatewayIP = info.Gateway
	subnet.LocalIP = info.IP

	devs := scanARPTable(info.Interface)

	occupied := make(map[string]bool)
	for _, d := range devs {
		occupied[d.IP] = true
	}
	occupied[info.IP] = true
	if info.Gateway != "" {
		occupied[info.Gateway] = true
	}

	recommends := recommendIPs(info.IP, occupied, 5)

	return &DeviceOnboarding{
		Subnet:       *subnet,
		OnlineDevs:   devs,
		RecommendIPs: recommends,
	}, nil
}

// CheckIPConflict pings and ARP-probes the given IP to check occupancy.
func CheckIPConflict(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil || !addr.Is4() {
		return false
	}
	// Quick ping check (1 second timeout).
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("ping", "-c", "1", "-W", "1000", ip)
	} else {
		cmd = exec.Command("ping", "-c", "1", "-W", "1", ip)
	}
	if err := cmd.Run(); err == nil {
		return true // responded — conflict
	}
	return false
}

// detectSubnet reads the interface's IPv4 address + prefix from the OS.
func detectSubnet(iface, fallbackIP string) (*SubnetInfo, error) {
	netIf, err := net.InterfaceByName(iface)
	if err != nil {
		return fallbackSubnet(fallbackIP), nil
	}
	addrs, err := netIf.Addrs()
	if err != nil || len(addrs) == 0 {
		return fallbackSubnet(fallbackIP), nil
	}
	for _, a := range addrs {
		cidrStr := a.String()
		prefix, err := netip.ParsePrefix(cidrStr)
		if err != nil || !prefix.Addr().Is4() {
			continue
		}
		bits := prefix.Bits()
		network := prefix.Masked()
		mask := prefixToNetmask(bits)

		first, last := hostRange(network)

		return &SubnetInfo{
			CIDR:       network.String(),
			PrefixLen:  bits,
			Netmask:    mask,
			UsableFrom: first,
			UsableTo:   last,
		}, nil
	}
	return fallbackSubnet(fallbackIP), nil
}

func fallbackSubnet(ip string) *SubnetInfo {
	addr, err := netip.ParseAddr(ip)
	if err != nil || !addr.Is4() {
		return &SubnetInfo{PrefixLen: 24, Netmask: "255.255.255.0"}
	}
	o := addr.As4()
	network := netip.PrefixFrom(netip.AddrFrom4([4]byte{o[0], o[1], o[2], 0}), 24)
	first, last := hostRange(network)
	return &SubnetInfo{
		CIDR:       network.String(),
		PrefixLen:  24,
		Netmask:    "255.255.255.0",
		UsableFrom: first,
		UsableTo:   last,
	}
}

func prefixToNetmask(bits int) string {
	mask := net.CIDRMask(bits, 32)
	return fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
}

func hostRange(prefix netip.Prefix) (first, last string) {
	addr := prefix.Addr().As4()
	bits := prefix.Bits()
	hostBits := 32 - bits
	if hostBits <= 1 {
		return prefix.Addr().String(), prefix.Addr().String()
	}
	// First host: network + 1
	f := [4]byte{addr[0], addr[1], addr[2], addr[3]}
	f[3] |= 1
	// Last host: broadcast - 1
	l := [4]byte{addr[0], addr[1], addr[2], addr[3]}
	hostMax := uint32(1<<hostBits) - 1
	base := uint32(l[0])<<24 | uint32(l[1])<<16 | uint32(l[2])<<8 | uint32(l[3])
	bcast := base | hostMax
	bcast-- // last usable
	l[0] = byte(bcast >> 24)
	l[1] = byte(bcast >> 16)
	l[2] = byte(bcast >> 8)
	l[3] = byte(bcast)
	return netip.AddrFrom4(f).String(), netip.AddrFrom4(l).String()
}

// scanARPTable reads the OS ARP table and returns known devices.
func scanARPTable(iface string) []LANDevice {
	var devs []LANDevice
	if runtime.GOOS == "darwin" {
		devs = scanARPDarwin(iface)
	} else {
		devs = scanARPLinux(iface)
	}
	return devs
}

func scanARPDarwin(iface string) []LANDevice {
	out, err := exec.Command("arp", "-a", "-i", iface).Output()
	if err != nil {
		return nil
	}
	var devs []LANDevice
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		// Format: hostname (IP) at MAC on iface ifscope [ethernet]
		parts := strings.Fields(line)
		if len(parts) < 4 || parts[3] == "(incomplete)" {
			continue
		}
		ip := strings.Trim(parts[1], "()")
		mac := parts[3]
		if _, err := netip.ParseAddr(ip); err != nil {
			continue
		}
		if !isValidMAC(mac) {
			continue
		}
		devs = append(devs, LANDevice{IP: ip, MAC: mac, Interface: iface})
	}
	return devs
}

func scanARPLinux(iface string) []LANDevice {
	out, err := exec.Command("ip", "neigh", "show", "dev", iface).Output()
	if err != nil {
		// fallback to /proc/net/arp
		return scanARPProc(iface)
	}
	var devs []LANDevice
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		// Format: IP lladdr MAC STATE
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		ip := fields[0]
		state := fields[len(fields)-1]
		if state == "FAILED" {
			continue
		}
		mac := ""
		for i, f := range fields {
			if f == "lladdr" && i+1 < len(fields) {
				mac = fields[i+1]
				break
			}
		}
		if mac == "" || !isValidMAC(mac) {
			continue
		}
		devs = append(devs, LANDevice{IP: ip, MAC: mac, Interface: iface})
	}
	return devs
}

func scanARPProc(iface string) []LANDevice {
	out, err := exec.Command("cat", "/proc/net/arp").Output()
	if err != nil {
		return nil
	}
	var devs []LANDevice
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue // skip header
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 6 || fields[5] != iface {
			continue
		}
		if fields[2] == "0x0" { // incomplete
			continue
		}
		devs = append(devs, LANDevice{IP: fields[0], MAC: fields[3], Interface: iface})
	}
	return devs
}

func isValidMAC(mac string) bool {
	_, err := net.ParseMAC(mac)
	return err == nil
}

// recommendIPs suggests up to `count` unoccupied IPs in the same /24.
func recommendIPs(localIP string, occupied map[string]bool, count int) []string {
	addr, err := netip.ParseAddr(localIP)
	if err != nil || !addr.Is4() {
		return nil
	}
	o := addr.As4()
	var result []string
	// Prefer high range (200-254) to avoid common DHCP pool conflicts.
	for _, last := range highRangeOrder() {
		candidate := netip.AddrFrom4([4]byte{o[0], o[1], o[2], byte(last)})
		if !occupied[candidate.String()] {
			result = append(result, candidate.String())
			if len(result) >= count {
				break
			}
		}
	}
	return result
}

func highRangeOrder() []int {
	var order []int
	for i := 200; i <= 254; i++ {
		order = append(order, i)
	}
	for i := 100; i < 200; i++ {
		order = append(order, i)
	}
	return order
}
