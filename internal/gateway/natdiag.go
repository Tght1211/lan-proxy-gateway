package gateway

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"net"
	"net/netip"
	"strings"
	"time"
)

// NATDiag holds the results of a NAT environment diagnosis.
type NATDiag struct {
	NATType         string     `json:"nat_type"`      // "open" | "moderate" | "strict" | "unknown"
	ExternalIP      string     `json:"external_ip"`   // public IP seen by STUN server
	ExternalPort    int        `json:"external_port"` // mapped port
	DoubleNAT       bool       `json:"double_nat"`    // true if behind two NATs
	DoubleNATDetail string     `json:"double_nat_detail,omitempty"`
	UPnP            UPnPStatus `json:"upnp"`
	Warnings        []string   `json:"warnings,omitempty"`
}

// UPnPStatus reports UPnP/NAT-PMP availability.
type UPnPStatus struct {
	Available   bool   `json:"available"`
	DeviceName  string `json:"device_name,omitempty"`
	ServiceType string `json:"service_type,omitempty"`
}

// DiagnoseNAT performs NAT type detection, double-NAT check, and UPnP discovery.
func (g *Gateway) DiagnoseNAT(ctx context.Context) (*NATDiag, error) {
	info := g.Info()
	if info.Interface == "" {
		if err := g.Detect(); err != nil {
			return nil, err
		}
	}

	info = g.Info()
	diag := &NATDiag{NATType: "unknown"}

	// 1. STUN NAT type detection
	stunCtx, stunCancel := context.WithTimeout(ctx, 5*time.Second)
	defer stunCancel()
	stunResult := probeSTUN(stunCtx)
	diag.NATType = stunResult.natType
	diag.ExternalIP = stunResult.externalIP
	diag.ExternalPort = stunResult.externalPort

	// 2. Double NAT detection
	if info.Gateway != "" && diag.ExternalIP != "" {
		routerIP, err := netip.ParseAddr(info.Gateway)
		if err == nil && isPrivate(routerIP) {
			externalIP, err := netip.ParseAddr(diag.ExternalIP)
			if err == nil && !isPrivate(externalIP) {
				// Single NAT: private gateway, public external — normal
			} else if err == nil && isPrivate(externalIP) {
				diag.DoubleNAT = true
				diag.DoubleNATDetail = fmt.Sprintf(
					"本机网关 %s 是内网地址，STUN 检测到的外部 IP %s 也是内网地址，存在双重 NAT",
					info.Gateway, diag.ExternalIP)
				diag.Warnings = append(diag.Warnings,
					"检测到双重 NAT，可能导致游戏联机受限 (NAT Type 3/Strict)")
			}
		}
	}

	// 3. UPnP discovery
	upnpCtx, upnpCancel := context.WithTimeout(ctx, 3*time.Second)
	defer upnpCancel()
	diag.UPnP = discoverUPnP(upnpCtx)

	// 4. Generate warnings
	if diag.NATType == "strict" {
		diag.Warnings = append(diag.Warnings,
			"NAT 类型为 Strict，游戏联机可能受限 (PS5 NAT Type 3 / Switch NAT D)")
	}
	if diag.NATType == "moderate" {
		diag.Warnings = append(diag.Warnings,
			"NAT 类型为 Moderate，大部分游戏可联机但 P2P 匹配可能较慢")
	}
	if !diag.UPnP.Available {
		diag.Warnings = append(diag.Warnings,
			"未检测到 UPnP，建议在路由器中开启 UPnP 或手动配置端口映射")
	}

	return diag, nil
}

// STUN probe (simplified RFC 5389 Binding Request)

var stunServers = []string{
	"stun.l.google.com:19302",
	"stun.cloudflare.com:3478",
	"stun.miwifi.com:3478",
}

type stunProbeResult struct {
	natType      string
	externalIP   string
	externalPort int
}

func probeSTUN(ctx context.Context) stunProbeResult {
	result := stunProbeResult{natType: "unknown"}

	for _, server := range stunServers {
		r, err := singleSTUNProbe(ctx, server)
		if err != nil {
			continue
		}
		result = r
		break
	}

	if result.externalIP == "" {
		result.natType = "strict"
		return result
	}

	// Determine NAT type by checking if our local port matches the external port.
	// This is a simplified check — full NAT typing needs two STUN servers.
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return result
	}
	localPort := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()

	// Probe with a second server using the same local port to see if mapping is consistent.
	r2, err := singleSTUNProbeFromPort(ctx, stunServers[len(stunServers)-1], localPort)
	if err != nil {
		// Can't determine precisely; assume moderate
		if result.externalPort != 0 {
			result.natType = "moderate"
		}
		return result
	}

	if r2.externalPort == result.externalPort && r2.externalIP == result.externalIP {
		result.natType = "open"
	} else if r2.externalIP == result.externalIP {
		result.natType = "moderate"
	} else {
		result.natType = "strict"
	}

	return result
}

func singleSTUNProbe(ctx context.Context, server string) (stunProbeResult, error) {
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return stunProbeResult{}, err
	}
	defer conn.Close()
	return doSTUNExchange(ctx, conn, server)
}

func singleSTUNProbeFromPort(ctx context.Context, server string, localPort int) (stunProbeResult, error) {
	conn, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", localPort))
	if err != nil {
		return stunProbeResult{}, err
	}
	defer conn.Close()
	return doSTUNExchange(ctx, conn, server)
}

func doSTUNExchange(ctx context.Context, conn net.PacketConn, server string) (stunProbeResult, error) {
	addr, err := net.ResolveUDPAddr("udp4", server)
	if err != nil {
		return stunProbeResult{}, err
	}

	// Build STUN Binding Request (RFC 5389)
	txID := make([]byte, 12)
	binary.BigEndian.PutUint32(txID[0:4], rand.Uint32())
	binary.BigEndian.PutUint32(txID[4:8], rand.Uint32())
	binary.BigEndian.PutUint32(txID[8:12], rand.Uint32())

	req := make([]byte, 20)
	binary.BigEndian.PutUint16(req[0:2], 0x0001)     // Binding Request
	binary.BigEndian.PutUint16(req[2:4], 0)          // Length
	binary.BigEndian.PutUint32(req[4:8], 0x2112A442) // Magic Cookie
	copy(req[8:20], txID)

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(3 * time.Second)
	}
	_ = conn.SetDeadline(deadline)

	if _, err := conn.WriteTo(req, addr); err != nil {
		return stunProbeResult{}, err
	}

	buf := make([]byte, 1024)
	n, _, err := conn.ReadFrom(buf)
	if err != nil {
		return stunProbeResult{}, err
	}

	return parseSTUNResponse(buf[:n])
}

func parseSTUNResponse(data []byte) (stunProbeResult, error) {
	if len(data) < 20 {
		return stunProbeResult{}, fmt.Errorf("STUN response too short")
	}
	msgType := binary.BigEndian.Uint16(data[0:2])
	if msgType != 0x0101 { // Binding Response
		return stunProbeResult{}, fmt.Errorf("not a STUN Binding Response: 0x%04x", msgType)
	}
	msgLen := binary.BigEndian.Uint16(data[2:4])
	if int(msgLen)+20 > len(data) {
		return stunProbeResult{}, fmt.Errorf("STUN message truncated")
	}
	magicCookie := binary.BigEndian.Uint32(data[4:8])

	// Parse attributes
	attrs := data[20 : 20+msgLen]
	for len(attrs) >= 4 {
		attrType := binary.BigEndian.Uint16(attrs[0:2])
		attrLen := binary.BigEndian.Uint16(attrs[2:4])
		if int(attrLen)+4 > len(attrs) {
			break
		}
		attrVal := attrs[4 : 4+attrLen]

		switch attrType {
		case 0x0020: // XOR-MAPPED-ADDRESS
			if ip, port, err := parseXORMappedAddr(attrVal, magicCookie); err == nil {
				return stunProbeResult{externalIP: ip, externalPort: port}, nil
			}
		case 0x0001: // MAPPED-ADDRESS
			if ip, port, err := parseMappedAddr(attrVal); err == nil {
				return stunProbeResult{externalIP: ip, externalPort: port}, nil
			}
		}

		// Advance to next attribute (4-byte aligned). The padded length must be
		// re-bounds-checked: a truncated packet whose padding runs past the end
		// would otherwise panic on the slice below.
		padded := int(attrLen)
		if padded%4 != 0 {
			padded += 4 - padded%4
		}
		if 4+padded > len(attrs) {
			break
		}
		attrs = attrs[4+padded:]
	}

	return stunProbeResult{}, fmt.Errorf("no mapped address in STUN response")
}

func parseXORMappedAddr(data []byte, magic uint32) (string, int, error) {
	if len(data) < 8 {
		return "", 0, fmt.Errorf("too short")
	}
	family := data[1]
	port := int(binary.BigEndian.Uint16(data[2:4])) ^ int(magic>>16)
	if family == 0x01 { // IPv4
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, binary.BigEndian.Uint32(data[4:8])^magic)
		return ip.String(), port, nil
	}
	return "", 0, fmt.Errorf("unsupported family %d", family)
}

func parseMappedAddr(data []byte) (string, int, error) {
	if len(data) < 8 {
		return "", 0, fmt.Errorf("too short")
	}
	family := data[1]
	port := int(binary.BigEndian.Uint16(data[2:4]))
	if family == 0x01 { // IPv4
		ip := net.IPv4(data[4], data[5], data[6], data[7])
		return ip.String(), port, nil
	}
	return "", 0, fmt.Errorf("unsupported family %d", family)
}

// UPnP SSDP discovery

func discoverUPnP(ctx context.Context) UPnPStatus {
	ssdpAddr := "239.255.255.250:1900"
	searchTargets := []string{
		"urn:schemas-upnp-org:device:InternetGatewayDevice:1",
		"urn:schemas-upnp-org:device:InternetGatewayDevice:2",
	}

	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return UPnPStatus{}
	}
	defer conn.Close()

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(3 * time.Second)
	}
	_ = conn.SetDeadline(deadline)

	addr, err := net.ResolveUDPAddr("udp4", ssdpAddr)
	if err != nil {
		return UPnPStatus{}
	}

	for _, st := range searchTargets {
		msg := fmt.Sprintf(
			"M-SEARCH * HTTP/1.1\r\nHOST: %s\r\nMAN: \"ssdp:discover\"\r\nMX: 2\r\nST: %s\r\n\r\n",
			ssdpAddr, st)
		_, _ = conn.WriteTo([]byte(msg), addr)
	}

	buf := make([]byte, 2048)
	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			break
		}
		resp := string(buf[:n])
		if strings.Contains(resp, "InternetGatewayDevice") || strings.Contains(resp, "WANIPConnection") {
			status := UPnPStatus{Available: true}
			for _, line := range strings.Split(resp, "\r\n") {
				lower := strings.ToLower(line)
				if strings.HasPrefix(lower, "server:") {
					status.DeviceName = strings.TrimSpace(line[7:])
				}
				if strings.HasPrefix(lower, "st:") {
					status.ServiceType = strings.TrimSpace(line[3:])
				}
			}
			return status
		}
	}

	return UPnPStatus{}
}

func isPrivate(ip netip.Addr) bool {
	return ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLoopback()
}
