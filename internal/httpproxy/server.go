// Package httpproxy provides the explicit LAN HTTP/HTTPS proxy listener.
package httpproxy

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Credentials struct{ Username, Password string }
type DialFunc func(context.Context, string, string) (net.Conn, error)

type Server struct {
	http    *http.Server
	dial    DialFunc
	auth    atomic.Pointer[Credentials]
	mu      sync.Mutex
	tunnels map[net.Conn]struct{}
	closed  bool
	running atomic.Bool
}

func New(dial DialFunc, credentials Credentials) *Server {
	s := &Server{dial: dial, tunnels: make(map[net.Conn]struct{})}
	s.SetCredentials(credentials)
	s.http = &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 64 << 10}
	return s
}
func (s *Server) SetCredentials(c Credentials) { s.auth.Store(&c) }
func (s *Server) Running() bool                { return s.running.Load() }
func (s *Server) Serve(ln net.Listener) error {
	s.running.Store(true)
	defer s.running.Store(false)
	err := s.http.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
func (s *Server) Close() error {
	s.mu.Lock()
	s.closed = true
	for conn := range s.tunnels {
		_ = conn.Close()
	}
	s.mu.Unlock()
	return s.http.Close()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	src, _, err := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(src)
	if err != nil || ip == nil || !(ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()) {
		http.Error(w, "LAN clients only", http.StatusForbidden)
		return
	}
	if isPACDownload(r) {
		s.servePAC(w, r)
		return
	}
	c := s.auth.Load()
	if c.Username != "" {
		scheme, token, _ := strings.Cut(r.Header.Get("Proxy-Authorization"), " ")
		supplied, err := base64.StdEncoding.DecodeString(token)
		want := sha256.Sum256([]byte(c.Username + ":" + c.Password))
		got := sha256.Sum256(supplied)
		if !strings.EqualFold(scheme, "Basic") || err != nil || subtle.ConstantTimeCompare(want[:], got[:]) != 1 {
			w.Header().Set("Proxy-Authenticate", `Basic realm="LAN Proxy Gateway", charset="UTF-8"`)
			http.Error(w, "Proxy authentication required", http.StatusProxyAuthRequired)
			return
		}
	}
	if r.Method == http.MethodConnect {
		s.connect(w, r, src)
		return
	}
	if r.URL.Scheme != "http" || r.URL.Host == "" || r.URL.User != nil {
		http.Error(w, "Expected an absolute HTTP URL or HTTPS CONNECT", http.StatusBadRequest)
		return
	}
	out := r.Clone(r.Context())
	out.RequestURI = ""
	out.Header = r.Header.Clone()
	stripHopHeaders(out.Header)
	out.Close = true
	// Never inherit environment proxies or share pooled connections across clients.
	transport := &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) { return s.dial(ctx, src, addr) }, DisableKeepAlives: true, ResponseHeaderTimeout: 30 * time.Second}
	defer transport.CloseIdleConnections()
	resp, err := transport.RoundTrip(out)
	if err != nil {
		http.Error(w, "Upstream connection failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	stripHopHeaders(resp.Header)
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	// Flush headers and each forwarded chunk. Without this, small SSE/chunked
	// responses remain in net/http's buffer until the origin closes.
	controller := http.NewResponseController(w)
	_ = controller.Flush()
	if _, err := io.Copy(flushingWriter{writer: w, controller: controller}, resp.Body); err != nil {
		// Do not turn a truncated upstream stream into a successful chunked EOF.
		panic(http.ErrAbortHandler)
	}
}

func (s *Server) connect(w http.ResponseWriter, r *http.Request, src string) {
	host, port, err := net.SplitHostPort(r.Host)
	n, portErr := strconv.Atoi(port)
	if err != nil || host == "" || portErr != nil || n < 1 || n > 65535 {
		http.Error(w, "CONNECT requires host:port", http.StatusBadRequest)
		return
	}
	upstream, err := s.dial(r.Context(), src, r.Host)
	if err != nil {
		http.Error(w, "Upstream connection failed", http.StatusBadGateway)
		return
	}
	defer upstream.Close()
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Tunneling unavailable", http.StatusInternalServerError)
		return
	}
	client, buffered, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer client.Close()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.tunnels[client] = struct{}{}
	s.tunnels[upstream] = struct{}{}
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.tunnels, client); delete(s.tunnels, upstream); s.mu.Unlock() }()
	if _, err := buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	if err := buffered.Flush(); err != nil {
		return
	}
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upstream, buffered); closeWrite(upstream); done <- struct{}{} }()
	go func() { _, _ = io.Copy(client, upstream); closeWrite(client); done <- struct{}{} }()
	<-done
	timer := time.NewTimer(2 * time.Minute)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		_ = client.Close()
		_ = upstream.Close()
		<-done
	}
}
func closeWrite(c net.Conn) {
	if half, ok := c.(interface{ CloseWrite() error }); ok {
		_ = half.CloseWrite()
	}
}

type flushingWriter struct {
	writer     io.Writer
	controller *http.ResponseController
}

func (w flushingWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if err == nil {
		err = w.controller.Flush()
	}
	return n, err
}

func stripHopHeaders(h http.Header) {
	for _, value := range h.Values("Connection") {
		for _, name := range strings.Split(value, ",") {
			h.Del(strings.TrimSpace(name))
		}
	}
	for _, name := range []string{"Connection", "Proxy-Connection", "Proxy-Authorization", "Proxy-Authenticate", "Keep-Alive", "TE", "Trailer", "Transfer-Encoding", "Upgrade"} {
		h.Del(name)
	}
}
