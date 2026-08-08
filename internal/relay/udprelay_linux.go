//go:build linux

// UDP original-destination recovery on Linux using IP_RECVORIGDSTADDR.
//
// @author buchi
// @since 2026-08-08
package relay

import (
	"fmt"
	"net/netip"

	"golang.org/x/sys/unix"
)

const (
	ipRecvOrigDstAddr = 20 // IP_RECVORIGDSTADDR
	ipOrigDstAddr     = 20 // IP_ORIGDSTADDR (cmsg type)
)

// readFromRedirectedPlatform reads a UDP packet and recovers the original
// destination from the IP_ORIGDSTADDR control message set by netfilter.
func (r *UDPRelay) readFromRedirectedPlatform(buf []byte) (int, netip.AddrPort, netip.AddrPort, error) {
	r.enableOrigDstOnce()

	oob := make([]byte, 64)
	n, oobn, _, clientUDP, err := r.conn.ReadMsgUDP(buf, oob)
	if err != nil {
		return 0, netip.AddrPort{}, netip.AddrPort{}, err
	}
	clientAddr := clientUDP.AddrPort()

	origDst, err := parseOrigDst(oob[:oobn])
	if err != nil {
		r.logger.Debug("UDP IP_ORIGDSTADDR 解析失败", "client", clientAddr, "err", err)
		return n, clientAddr, netip.AddrPort{}, nil
	}
	return n, clientAddr, origDst, nil
}

func (r *UDPRelay) enableOrigDstOnce() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.origDstEnabled {
		return
	}
	rc, err := r.conn.SyscallConn()
	if err != nil {
		r.logger.Warn("获取 raw conn 失败", "err", err)
		return
	}
	_ = rc.Control(func(fd uintptr) {
		_ = unix.SetsockoptInt(int(fd), unix.SOL_IP, ipRecvOrigDstAddr, 1)
	})
	r.origDstEnabled = true
}

func parseOrigDst(oob []byte) (netip.AddrPort, error) {
	msgs, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		return netip.AddrPort{}, err
	}
	for _, msg := range msgs {
		if msg.Header.Level == unix.SOL_IP && msg.Header.Type == ipOrigDstAddr {
			if len(msg.Data) < 8 {
				continue
			}
			// struct sockaddr_in: family(2) + port(2) + addr(4)
			port := uint16(msg.Data[2])<<8 | uint16(msg.Data[3])
			ip := netip.AddrFrom4([4]byte{msg.Data[4], msg.Data[5], msg.Data[6], msg.Data[7]})
			return netip.AddrPortFrom(ip, port), nil
		}
	}
	return netip.AddrPort{}, fmt.Errorf("IP_ORIGDSTADDR 缺失")
}
