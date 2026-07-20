//go:build darwin

package relay

import (
	"fmt"
	"net"
	"net/netip"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

// macOS pf `rdr` rewrites the destination of redirected connections, so the
// original target must be recovered from pf's NAT state table via the
// DIOCNATLOOK ioctl on /dev/pf.
//
// Modern XNU layout (bsd/net/pfvar.h, verified against xnu-12377):
//
//	struct pfioc_natlook {
//	    struct pf_addr        saddr, daddr, rsaddr, rdaddr;  // 16 bytes each
//	    union pf_state_xport  sxport, dxport, rsxport, rdxport; // 4 bytes each
//	    sa_family_t           af;          // u8
//	    u_int8_t              proto;
//	    u_int8_t              proto_variant;
//	    u_int8_t              direction;
//	};                                       // total 84 bytes
//
// The ioctl number encodes the struct size (0xC0544417); a wrong size makes
// the kernel return EOPNOTSUPP. Older XNU used a 520-byte sockaddr_storage
// layout — this resolver targets the modern one and fails loudly otherwise.
const (
	pfDevicePath = "/dev/pf"
	pfIn         = 1
	pfOut        = 2

	iocInOut     = 0xC0000000
	iocParmMask  = 0x1fff
	natlookGroup = 'D'
	natlookCmd   = 23
)

type pfiocNatlook struct {
	Saddr, Daddr, Rsaddr, Rdaddr       [16]byte
	Sxport, Dxport, Rsxport, Rdxport   uint32
	Af, Proto, ProtoVariant, Direction uint8
}

var dioCNATLOOK = computeNatlookIOC()

func computeNatlookIOC() uintptr {
	size := uintptr(unsafe.Sizeof(pfiocNatlook{}))
	return iocInOut | (size&iocParmMask)<<16 | uintptr(natlookGroup)<<8 | natlookCmd
}

// PFNatlook resolves original destinations via the pf NAT table.
// The /dev/pf fd is opened once and reused for the process lifetime.
type PFNatlook struct {
	fd int
	mu sync.Mutex
}

// NewPlatformOrigDST opens /dev/pf and returns the macOS resolver.
// Requires root (the daemon runs elevated).
func NewPlatformOrigDST() (OrigDSTResolver, error) {
	fd, err := unix.Open(pfDevicePath, unix.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("打开 %s 失败(需要 root): %w", pfDevicePath, err)
	}
	return &PFNatlook{fd: fd}, nil
}

func (r *PFNatlook) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fd >= 0 {
		err := unix.Close(r.fd)
		r.fd = -1
		return err
	}
	return nil
}

func (r *PFNatlook) Resolve(c *net.TCPConn) (netip.AddrPort, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fd < 0 {
		return netip.AddrPort{}, fmt.Errorf("pf natlook 已关闭")
	}
	nl := pfiocNatlook{Af: unix.AF_INET, Proto: unix.IPPROTO_TCP}
	if err := putAddr(&nl.Saddr, &nl.Sxport, c.RemoteAddr()); err != nil {
		return netip.AddrPort{}, err
	}
	if err := putAddr(&nl.Daddr, &nl.Dxport, c.LocalAddr()); err != nil {
		return netip.AddrPort{}, err
	}
	// rdr lookups should use PF_OUT; some macOS versions disagree — fall back
	// to PF_IN on ENOENT before giving up.
	for _, dir := range []uint8{pfOut, pfIn} {
		nl.Direction = dir
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(r.fd), dioCNATLOOK, uintptr(unsafe.Pointer(&nl)))
		if errno == 0 {
			return readAddr(&nl.Rdaddr, &nl.Rdxport)
		}
		if errno == unix.ENOENT {
			continue
		}
		// EOPNOTSUPP would mean the struct layout doesn't match this kernel.
		return netip.AddrPort{}, fmt.Errorf("DIOCNATLOOK: %w", errno)
	}
	return netip.AddrPort{}, fmt.Errorf("DIOCNATLOOK: pf 中无此连接的 rdr 状态(连接未经过 rdr?)")
}

// putAddr writes an IPv4 address (network order) into a pf_addr and the port
// into a pf_state_xport. pf copies ports verbatim from packet headers, so
// xport holds them in NETWORK byte order (unlike Go's host-order TCPAddr.Port).
func putAddr(a *[16]byte, xport *uint32, addr net.Addr) error {
	ta, ok := addr.(*net.TCPAddr)
	if !ok {
		return fmt.Errorf("地址类型异常 %T", addr)
	}
	ip4 := ta.IP.To4()
	if ip4 == nil {
		return fmt.Errorf("仅支持 IPv4: %s", ta.IP)
	}
	for i := range a {
		a[i] = 0
	}
	copy(a[:4], ip4)
	*xport = uint32(htons16(uint16(ta.Port)))
	return nil
}

func readAddr(a *[16]byte, xport *uint32) (netip.AddrPort, error) {
	var ip [4]byte
	copy(ip[:], a[:4])
	port := ntohs16(uint16(*xport & 0xffff))
	if port == 0 {
		return netip.AddrPort{}, fmt.Errorf("pf 返回了空端口")
	}
	return netip.AddrPortFrom(netip.AddrFrom4(ip), port), nil
}

func htons16(v uint16) uint16 { return v>>8 | v<<8 }
func ntohs16(v uint16) uint16 { return htons16(v) }
