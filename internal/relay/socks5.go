package relay

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/netip"
	"time"
)

// NewSOCKS5Dialer dials targets through a SOCKS5 proxy (RFC 1928, optional
// username/password auth per RFC 1929). Domain names are passed to the proxy
// unresolved (ATYP=DOMAIN) so the proxy resolves remotely.
func NewSOCKS5Dialer(addr, username, password string, timeout time.Duration) Dialer {
	return &socks5Dialer{
		addr:     addr,
		username: username,
		password: password,
		timeout:  timeout,
		forward:  &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second},
	}
}

type socks5Dialer struct {
	addr     string
	username string
	password string
	timeout  time.Duration
	forward  *net.Dialer
}

func (d *socks5Dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" {
		return nil, fmt.Errorf("socks5: 不支持的网络类型 %s", network)
	}
	conn, err := d.forward.DialContext(ctx, "tcp", d.addr)
	if err != nil {
		return nil, fmt.Errorf("socks5: 连接代理 %s 失败: %w", d.addr, err)
	}
	timeout := d.timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	if err := d.handshake(conn, addr, timeout); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func (d *socks5Dialer) handshake(conn net.Conn, addr string, timeout time.Duration) error {
	_ = conn.SetDeadline(time.Now().Add(timeout))
	defer conn.SetDeadline(time.Time{})

	// greeting: VER=5, NMETHODS, METHODS (no-auth, and user/pass when configured)
	methods := []byte{0x00}
	if d.username != "" || d.password != "" {
		methods = append(methods, 0x02)
	}
	greeting := append([]byte{0x05, byte(len(methods))}, methods...)
	if _, err := conn.Write(greeting); err != nil {
		return fmt.Errorf("socks5: 发送握手失败: %w", err)
	}

	var resp [2]byte
	if _, err := io.ReadFull(conn, resp[:]); err != nil {
		return fmt.Errorf("socks5: 读取握手响应失败: %w", err)
	}
	if resp[0] != 0x05 {
		return fmt.Errorf("socks5: 代理返回了不支持的版本 %d", resp[0])
	}
	switch resp[1] {
	case 0x00: // no auth
	case 0x02: // username/password (RFC 1929)
		if err := d.auth(conn); err != nil {
			return err
		}
	case 0xff:
		return fmt.Errorf("socks5: 代理不接受任何认证方式")
	default:
		return fmt.Errorf("socks5: 代理要求不支持的认证方式 0x%02x", resp[1])
	}

	// CONNECT request: VER, CMD=1, RSV, ATYP, DST.ADDR, DST.PORT
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("socks5: 目标地址 %q 不合法: %w", addr, err)
	}
	port, err := net.LookupPort("tcp", portStr)
	if err != nil || port < 0 || port > 65535 {
		return fmt.Errorf("socks5: 目标端口 %q 不合法", portStr)
	}

	req := []byte{0x05, 0x01, 0x00}
	if ip, err := netip.ParseAddr(host); err == nil {
		if ip.Is4() {
			b := ip.As4()
			req = append(req, 0x01, b[0], b[1], b[2], b[3])
		} else {
			b := ip.As16()
			req = append(req, 0x04)
			req = append(req, b[:]...)
		}
	} else {
		if len(host) > 255 {
			return fmt.Errorf("socks5: 域名过长: %q", host)
		}
		req = append(req, 0x03, byte(len(host)))
		req = append(req, host...)
	}
	var portBuf [2]byte
	binary.BigEndian.PutUint16(portBuf[:], uint16(port))
	req = append(req, portBuf[:]...)

	if _, err := conn.Write(req); err != nil {
		return fmt.Errorf("socks5: 发送 CONNECT 失败: %w", err)
	}

	// reply: VER, REP, RSV, ATYP, BND.ADDR, BND.PORT
	var head [4]byte
	if _, err := io.ReadFull(conn, head[:]); err != nil {
		return fmt.Errorf("socks5: 读取 CONNECT 响应失败: %w", err)
	}
	if head[0] != 0x05 {
		return fmt.Errorf("socks5: 响应版本异常 %d", head[0])
	}
	if head[1] != 0x00 {
		return fmt.Errorf("socks5: 连接目标失败: %s", socks5RepError(head[1]))
	}
	var skip int
	switch head[3] {
	case 0x01:
		skip = 4
	case 0x03:
		var ln [1]byte
		if _, err := io.ReadFull(conn, ln[:]); err != nil {
			return fmt.Errorf("socks5: 读取响应地址失败: %w", err)
		}
		skip = int(ln[0])
	case 0x04:
		skip = 16
	default:
		return fmt.Errorf("socks5: 响应地址类型异常 0x%02x", head[3])
	}
	if _, err := io.ReadFull(conn, make([]byte, skip+2)); err != nil {
		return fmt.Errorf("socks5: 读取响应剩余部分失败: %w", err)
	}
	return nil
}

func (d *socks5Dialer) auth(conn net.Conn) error {
	if len(d.username) > 255 || len(d.password) > 255 {
		return fmt.Errorf("socks5: 用户名或密码过长")
	}
	req := []byte{0x01, byte(len(d.username))}
	req = append(req, d.username...)
	req = append(req, byte(len(d.password)))
	req = append(req, d.password...)
	if _, err := conn.Write(req); err != nil {
		return fmt.Errorf("socks5: 发送认证信息失败: %w", err)
	}
	var resp [2]byte
	if _, err := io.ReadFull(conn, resp[:]); err != nil {
		return fmt.Errorf("socks5: 读取认证响应失败: %w", err)
	}
	if resp[1] != 0x00 {
		return fmt.Errorf("socks5: 用户名或密码错误")
	}
	return nil
}

func socks5RepError(rep byte) string {
	switch rep {
	case 0x01:
		return "代理服务器故障"
	case 0x02:
		return "规则禁止"
	case 0x03:
		return "网络不可达"
	case 0x04:
		return "主机不可达"
	case 0x05:
		return "目标拒绝连接"
	case 0x06:
		return "TTL 过期"
	case 0x07:
		return "不支持的命令"
	case 0x08:
		return "不支持的地址类型"
	default:
		return fmt.Sprintf("未知错误 0x%02x", rep)
	}
}
