package app

import (
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestDNSPortFallbackPersistsAndPreservesOwner(t *testing.T) {
	for _, network := range []string{"tcp", "udp"} {
		t.Run(network, func(t *testing.T) {
			var port int
			if network == "tcp" {
				ln, err := net.Listen("tcp", ":0")
				if err != nil {
					t.Fatal(err)
				}
				defer ln.Close()
				port = ln.Addr().(*net.TCPAddr).Port
			} else {
				ln, err := net.ListenPacket("udp", ":0")
				if err != nil {
					t.Fatal(err)
				}
				defer ln.Close()
				port = ln.LocalAddr().(*net.UDPAddr).Port
			}
			cfg := config.Default()
			cfg.DNS.Port = port
			a := &App{Cfg: cfg, Paths: config.Paths{ConfigFile: filepath.Join(t.TempDir(), "gateway.yaml")}}
			if err := a.prepareDNSPort(slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
				t.Fatal(err)
			}
			if a.Cfg.DNS.Port == port || dnsPortAvailable(port) {
				t.Fatal("conflicting listener must remain untouched")
			}
			data, err := os.ReadFile(a.Paths.ConfigFile)
			if err != nil {
				t.Fatal(err)
			}
			saved, err := config.Parse(data)
			if err != nil || saved.DNS.Port != a.Cfg.DNS.Port {
				t.Fatalf("fallback must survive reload: %v", err)
			}
			if !firewallConfig(saved).DNSLocalRedirect {
				t.Fatal("fallback needs standard port mapping")
			}
		})
	}
}
