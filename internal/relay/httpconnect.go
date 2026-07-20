package relay

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"time"
)

// NewHTTPConnectDialer dials targets through an HTTP proxy using the CONNECT
// method. Domain names are passed to the proxy unresolved.
func NewHTTPConnectDialer(addr, username, password string, timeout time.Duration) Dialer {
	return &httpConnectDialer{
		addr:     addr,
		username: username,
		password: password,
		timeout:  timeout,
		forward:  &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second},
	}
}

type httpConnectDialer struct {
	addr     string
	username string
	password string
	timeout  time.Duration
	forward  *net.Dialer
}

func (d *httpConnectDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" {
		return nil, fmt.Errorf("http: 不支持的网络类型 %s", network)
	}
	conn, err := d.forward.DialContext(ctx, "tcp", d.addr)
	if err != nil {
		return nil, fmt.Errorf("http: 连接代理 %s 失败: %w", d.addr, err)
	}
	timeout := d.timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))

	req := &http.Request{
		Method: http.MethodConnect,
		Host:   addr,
		Header: http.Header{
			"User-Agent":       []string{"lan-proxy-gateway"},
			"Proxy-Connection": []string{"Keep-Alive"},
		},
	}
	if d.username != "" || d.password != "" {
		cred := base64.StdEncoding.EncodeToString([]byte(d.username + ":" + d.password))
		req.Header.Set("Proxy-Authorization", "Basic "+cred)
	}
	// Request.Write would turn CONNECT into an absolute-URI form; write it manually.
	if _, err := fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n", addr, addr); err != nil {
		conn.Close()
		return nil, fmt.Errorf("http: 发送 CONNECT 失败: %w", err)
	}
	if err := req.Header.Write(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("http: 发送 CONNECT 头失败: %w", err)
	}
	if _, err := fmt.Fprintf(conn, "\r\n"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("http: 发送 CONNECT 结束符失败: %w", err)
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("http: 读取 CONNECT 响应失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		conn.Close()
		return nil, fmt.Errorf("http: 代理拒绝 CONNECT: %s", resp.Status)
	}
	_ = conn.SetDeadline(time.Time{})
	// The bufio reader may have already buffered bytes belonging to the
	// tunneled stream — they must be drained before the raw conn.
	if br.Buffered() > 0 {
		return &bufferedConn{Conn: conn, r: br}, nil
	}
	return conn, nil
}

// bufferedConn serves bytes buffered in r before reading from the conn.
type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.r.Read(p)
}
