package app

import (
	"fmt"
	"log/slog"
	"net"
	"strconv"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func dnsPortAvailable(port int) bool {
	addr := net.JoinHostPort("", strconv.Itoa(port))
	tcp, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	defer tcp.Close()
	udp, err := net.ListenPacket("udp", addr)
	if err != nil {
		return false
	}
	defer udp.Close()
	return true
}

// Persist the selected port so later config reloads keep DNS and firewall in sync.
func (a *App) prepareDNSPort(logger *slog.Logger) error {
	if !a.Cfg.DNS.Enabled || dnsPortAvailable(a.Cfg.DNS.Port) {
		return nil
	}
	if !a.Cfg.Gateway.Enabled {
		return fmt.Errorf("DNS 端口 %d 被占用；网关转发未启用，无法自动映射备用端口", a.Cfg.DNS.Port)
	}
	previous := a.Cfg.DNS.Port
	for port := config.DefaultDNSPort; port < config.DefaultDNSPort+100; port++ {
		if port == a.Cfg.Runtime.APIPort || port == a.Cfg.Runtime.RedirPort || port == a.Cfg.Runtime.UDPRedirPort || !dnsPortAvailable(port) {
			continue
		}
		a.Cfg.DNS.Port = port
		if err := config.Save(a.Cfg, a.Paths.ConfigFile); err != nil {
			a.Cfg.DNS.Port = previous
			return fmt.Errorf("保存备用 DNS 端口失败: %w", err)
		}
		logger.Info("DNS 端口冲突，已自动切换；局域网设备仍使用端口 53", "previous_port", previous, "dns_port", port)
		return nil
	}
	return fmt.Errorf("未找到可用的 DNS 备用端口（%d-%d）", config.DefaultDNSPort, config.DefaultDNSPort+99)
}
