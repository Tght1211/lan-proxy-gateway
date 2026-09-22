package app

import (
	"context"
	"log/slog"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/relay"
)

// applyConfig re-reads the config file and live-applies deltas.
// Everything is idempotent: relay dialer swap, dns toggles, firewall re-sync.
func (rt *daemonRuntime) applyConfig(a *App, cfg *config.Config) {
	a.setCfg(cfg)
	if dialer, err := buildDialer(cfg.Egress); err == nil {
		rt.relay.SetDialer(dialer, cfg.Egress.Mode == config.EgressProxy)
	}
	rt.relay.SetProxyFailAction(cfg.ProxyFailure.Action)
	rt.applyRouting(cfg)
	if rt.dns != nil {
		rt.dns.SetUpstreams(cfg.DNS.Upstreams)
		rt.dns.SetFakeIPEnabled(cfg.Egress.Mode == config.EgressProxy && cfg.DNS.FakeIP)
	}
	rt.bindFakeIP()
	rt.bindUDPRelayFakeIP()
	if cfg.Gateway.Enabled {
		if err := a.Gateway.Enable(firewallConfig(cfg)); err != nil {
			rt.logger.Warn("防火墙规则热应用失败", "err", err)
		}
	}
	rt.logger.Info("配置已热应用", "egress", cfg.Egress.Mode)
}

func (rt *daemonRuntime) applyRouting(cfg *config.Config) {
	direct, proxy, rules := buildRoutingArgs(cfg)
	rt.relay.SetRouting(cfg.Egress.Mode, direct, proxy, rules)
	if rt.udpRelay != nil {
		rt.udpRelay.SetRouting(relay.BuildRoutingPolicy(cfg.Egress.Mode, direct, proxy, rules))
	}
	rt.setEgressProbe(cfg, proxy)
}

// syncUDPRelayRouting pushes the current routing policy to the UDP relay.
// Called once at startup after udpRelay is created; hot-reload goes through applyRouting.
func (rt *daemonRuntime) syncUDPRelayRouting(cfg *config.Config) {
	if rt.udpRelay == nil {
		return
	}
	direct, proxy, rules := buildRoutingArgs(cfg)
	rt.udpRelay.SetRouting(relay.BuildRoutingPolicy(cfg.Egress.Mode, direct, proxy, rules))
}

func buildRoutingArgs(cfg *config.Config) (relay.Dialer, relay.Dialer, []relay.RouteRule) {
	direct, _ := buildDialer(config.EgressConfig{Mode: config.EgressDirect})
	var proxy relay.Dialer
	if cfg.Egress.Mode == config.EgressProxy {
		proxy, _ = buildDialer(cfg.Egress)
	}
	rules := make([]relay.RouteRule, 0, len(cfg.Routing.Rules))
	for _, rule := range cfg.Routing.Rules {
		rules = append(rules, relay.RouteRule{Type: rule.Type, Value: rule.Value, Action: rule.Action})
	}
	return direct, proxy, rules
}

// setEgressProbe arms the global outage monitor with the upstream proxy
// address + dialer so it can probe the port and run per-host recovery.
func (rt *daemonRuntime) setEgressProbe(cfg *config.Config, proxy relay.Dialer) {
	if cfg.Egress.Mode != config.EgressProxy {
		rt.relay.SetEgressProbe("", nil)
		return
	}
	addr := net.JoinHostPort(cfg.Egress.Proxy.Host, strconv.Itoa(cfg.Egress.Proxy.Port))
	rt.relay.SetEgressProbe(addr, proxy)
}

// startEgressMonitor launches the port-probe and per-host recovery loop once;
// on proxy-recovery success it removes the learned direct rule for that host.
func (rt *daemonRuntime) startEgressMonitor(ctx context.Context, a *App, logger *slog.Logger) {
	rt.relay.StartEgressMonitor(ctx, func(host string) {
		if removed, err := a.RemoveLearnedDirectRule(host); err != nil {
			logger.Warn("移除学习规则失败", "host", host, "err", err)
		} else if removed {
			logger.Info("代理恢复探测成功，已移除自动学习直连规则", "host", host)
		}
	})
}

// bindFakeIP wires the relay's fake-ip lookup to the DNS server's pool.
func (rt *daemonRuntime) bindFakeIP() {
	if rt.dns == nil || rt.relay == nil {
		return
	}
	prefix := rt.dns.FakeIPRange()
	rt.relay.SetFakeIP(&prefix, rt.dns.LookupFakeIP)
	rt.relay.SetRealIPLookup(rt.dns.LookupRealIP)
}

// bindUDPRelayFakeIP keeps the UDP relay's fake-ip lookup in sync with DNS.
func (rt *daemonRuntime) bindUDPRelayFakeIP() {
	if rt.dns == nil || rt.udpRelay == nil {
		return
	}
	prefix := rt.dns.FakeIPRange()
	rt.udpRelay.SetFakeIP(&prefix, rt.dns.LookupFakeIP)
}

// watchConfig polls the config file mtime and applies changes (belt; the
// /api/reload poke is the suspenders).
func (rt *daemonRuntime) watchConfig(ctx context.Context, a *App) {
	var lastMod time.Time
	if fi, err := os.Stat(a.Paths.ConfigFile); err == nil {
		lastMod = fi.ModTime()
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fi, err := os.Stat(a.Paths.ConfigFile)
			if err != nil || !fi.ModTime().After(lastMod) {
				continue
			}
			lastMod = fi.ModTime()
			cfg, err := config.LoadFrom(a.Paths.ConfigFile)
			if err != nil {
				rt.logger.Warn("配置文件变更但解析失败", "err", err)
				continue
			}
			rt.applyConfig(a, cfg)
		}
	}
}
