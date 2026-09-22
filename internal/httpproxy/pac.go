package httpproxy

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// servePAC is a public configuration download on this same listener, not a
// forwarded request. It deliberately contains no credentials or routing copy.
func (s *Server) servePAC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
		port = "80"
	}
	n, e := strconv.Atoi(port)
	valid := host != "" && e == nil && n > 0 && n < 65536
	if net.ParseIP(host) == nil {
		for _, c := range host {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '-') {
				valid = false
			}
		}
	}
	if !valid {
		http.Error(w, "Invalid proxy address", http.StatusBadRequest)
		return
	}
	endpoint := net.JoinHostPort(host, port)
	proxy, _ := json.Marshal("PROXY " + endpoint)
	// Route decisions stay in the gateway. No silent DIRECT fallback when the
	// proxy is down: PAC and manual clients should use the same network path.
	script := "function FindProxyForURL(url, host) {\n" +
		"  if (host === 'localhost' || host === '127.0.0.1' || host === '::1' || host === '[::1]') return 'DIRECT';\n" +
		"  return " + string(proxy) + ";\n}\n"
	w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(len(script)))
	if r.Method == http.MethodGet {
		_, _ = w.Write([]byte(script))
	}
}

func isPACDownload(r *http.Request) bool {
	return r.Method != http.MethodConnect && !r.URL.IsAbs() && r.URL.Host == "" && strings.EqualFold(r.URL.Path, "/proxy.pac")
}
