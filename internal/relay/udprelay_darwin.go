//go:build darwin

// UDP original-destination recovery on macOS using pf DIOCNATLOOK.
//
// @author buchi
// @since 2026-08-08
package relay

import (
	"fmt"
	"net"
	"net/netip"
	"unsafe"

	"golang.org/x/sys/unix"
)

// readFromRedirectedPlatform reads a UDP packet from the redirected listener
// and recovers the original destination via pf's NAT state table (DIOCNATLOOK).
func (r *UDPRelay) readFromRedirectedPlatform(buf []byte) (int, netip.AddrPort, netip.AddrPort, error) {
	n, clientUDP, err := r.conn.ReadFromUDP(buf)
	if err != nil {
		return 0, netip.AddrPort{}, netip.AddrPort{}, err
	}
	clientAddr := clientUDP.AddrPort()
	localAddr := r.conn.LocalAddr().(*net.UDPAddr).AddrPort()

	origDst, err := pfNatlookUDP(clientAddr, localAddr)
	if err != nil {
		r.logger.Debug("UDP DIOCNATLOOK 失败", "client", clientAddr, "err", err)
		return n, clientAddr, netip.AddrPort{}, nil
	}
	return n, clientAddr, origDst, nil
}

// pfNatlookUDP queries pf's NAT state to recover the original UDP destination.
// Opens /dev/pf per call — acceptable because this only runs on new session
// creation (once per unique client:port + fake_ip:port tuple, not per packet).
func pfNatlookUDP(src, dst netip.AddrPort) (netip.AddrPort, error) {
	fd, err := unix.Open(pfDevicePath, unix.O_RDWR, 0)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("打开 %s 失败: %w", pfDevicePath, err)
	}
	defer unix.Close(fd)

	nl := pfiocNatlook{Af: unix.AF_INET, Proto: unix.IPPROTO_UDP}
	udpPutAddrPort(&nl.Saddr, &nl.Sxport, src)
	udpPutAddrPort(&nl.Daddr, &nl.Dxport, dst)

	for _, dir := range []uint8{pfOut, pfIn} {
		nl.Direction = dir
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), dioCNATLOOK, uintptr(unsafe.Pointer(&nl)))
		if errno == 0 {
			return readAddr(&nl.Rdaddr, &nl.Rdxport)
		}
		if errno == unix.ENOENT {
			continue
		}
		return netip.AddrPort{}, fmt.Errorf("DIOCNATLOOK UDP: %w", errno)
	}
	return netip.AddrPort{}, fmt.Errorf("DIOCNATLOOK: pf 中无此 UDP 流的 rdr 状态")
}

// udpPutAddrPort writes an IPv4 AddrPort into pf natlook address fields.
func udpPutAddrPort(a *[16]byte, xport *uint32, ap netip.AddrPort) {
	for i := range a {
		a[i] = 0
	}
	ip4 := ap.Addr().As4()
	copy(a[:4], ip4[:])
	*xport = uint32(htons16(ap.Port()))
}
