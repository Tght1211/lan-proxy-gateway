//go:build linux

package relay

import (
	"fmt"
	"net"
	"net/netip"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	soOriginalDst = 80 // SO_ORIGINAL_DST
	solIP         = 0  // SOL_IP
)

type linuxOrigDST struct{}

// NewPlatformOrigDST returns the Linux resolver based on SO_ORIGINAL_DST,
// which reads the pre-REDIRECT destination saved by netfilter NAT.
func NewPlatformOrigDST() (OrigDSTResolver, error) {
	return linuxOrigDST{}, nil
}

func (linuxOrigDST) Resolve(c *net.TCPConn) (netip.AddrPort, error) {
	rc, err := c.SyscallConn()
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("获取 raw conn 失败: %w", err)
	}
	var raw unix.RawSockaddrInet4
	size := uint32(unsafe.Sizeof(raw))
	var sockErr error
	if err := rc.Control(func(fd uintptr) {
		_, _, errno := unix.Syscall6(
			unix.SYS_GETSOCKOPT,
			fd,
			solIP,
			soOriginalDst,
			uintptr(unsafe.Pointer(&raw)),
			uintptr(unsafe.Pointer(&size)),
			0,
		)
		if errno != 0 {
			sockErr = errno
		}
	}); err != nil {
		return netip.AddrPort{}, fmt.Errorf("SO_ORIGINAL_DST 失败: %w", err)
	}
	if sockErr != nil {
		return netip.AddrPort{}, fmt.Errorf("SO_ORIGINAL_DST 失败(连接未经过 REDIRECT?): %w", sockErr)
	}
	ip := netip.AddrFrom4(raw.Addr)
	// raw.Port is stored big-endian; convert byte-wise for portability.
	p := (*[2]byte)(unsafe.Pointer(&raw.Port))
	port := uint16(p[0])<<8 | uint16(p[1])
	return netip.AddrPortFrom(ip, port), nil
}
