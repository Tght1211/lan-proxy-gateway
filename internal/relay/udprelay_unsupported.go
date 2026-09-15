//go:build !darwin && !linux

package relay

import (
	"fmt"
	"net/netip"
)

func (r *UDPRelay) readFromRedirectedPlatform(buf []byte) (int, netip.AddrPort, netip.AddrPort, error) {
	return 0, netip.AddrPort{}, netip.AddrPort{}, fmt.Errorf("UDP relay: 当前平台不支持原始目标恢复")
}
