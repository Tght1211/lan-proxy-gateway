package app

import (
	"context"
	"fmt"
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
// Runs from API/watch goroutines, so service pointers go through services().
func (rt *daemonRuntime) applyConfig(a *App, cfg *config.Config) error {
	old := a.getCfg()
	if old.Gateway.AccessMode != cfg.Gateway.AccessMode || old.Gateway.Enabled != cfg.Gateway.Enabled {
		return fmt.Errorf("接入模式已保存，请重启核心后生效")
	}

	if err := rt.syncHTTPProxy(cfg); err != nil {
		rt.logger.Error("HTTP 代理配置应用失败，保留原配置", "err", err)
		return err
	}
	a.setCfg(cfg)
	relaySrv, _, dnsSrv := rt.services()
	if dialer, err := buildDialer(cfg.Egress); err == nil {
		relaySrv.SetDialer(dialer, cfg.Egress.Mode == config.EgressProxy)
	}
	relaySrv.SetProxyFailAction(cfg.ProxyFailure.Action)
	rt.applyRouting(cfg)
	if dnsSrv != nil {
		dnsSrv.SetUpstreams(cfg.DNS.Upstreams)
		dnsSrv.SetFakeIPEnabled(cfg.Egress.Mode == config.EgressProxy && cfg.DNS.FakeIP)
	}
	rt.bindFakeIP()
	rt.bindUDPRelayFakeIP()
	if cfg.Gateway.Enabled {
		if err := a.Gateway.Enable(firewallConfig(cfg)); err != nil {
			rt.logger.Warn("防火墙规则热应用失败", "err", err)
		}
	}
	rt.logger.Info("配置已热应用", "egress", cfg.Egress.Mode)
	return nil
}

func (rt *daemonRuntime) applyRouting(cfg *config.Config) {
	relaySrv, udpRelay, _ := rt.services()
	direct, proxy, rules := buildRoutingArgs(cfg)
	relaySrv.SetRouting(cfg.Egress.Mode, direct, proxy, rules)
	if udpRelay != nil {
		udpRelay.SetRouting(relay.BuildRoutingPolicy(cfg.Egress.Mode, direct, proxy, rules))
	}
	rt.setEgressProbe(cfg, proxy)
}

// syncUDPRelayRouting pushes the current routing policy to the UDP relay.
// Called once at startup after udpRelay is created; hot-reload goes through applyRouting.
func (rt *daemonRuntime) syncUDPRelayRouting(cfg *config.Config) {
	_, udpRelay, _ := rt.services()
	if udpRelay == nil {
		return
	}
	direct, proxy, rules := buildRoutingArgs(cfg)
	udpRelay.SetRouting(relay.BuildRoutingPolicy(cfg.Egress.Mode, direct, proxy, rules))
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
	relaySrv, _, _ := rt.services()
	if cfg.Egress.Mode != config.EgressProxy {
		relaySrv.SetEgressProbe("", nil)
		return
	}
	addr := net.JoinHostPort(cfg.Egress.Proxy.Host, strconv.Itoa(cfg.Egress.Proxy.Port))
	relaySrv.SetEgressProbe(addr, proxy)
}

// Keep outage recovery monitoring, but user-confirmed rules remain until undone.
func (rt *daemonRuntime) startEgressMonitor(ctx context.Context, a *App, logger *slog.Logger) {
	rt.relay.StartEgressMonitor(ctx, nil)
}

// bindFakeIP wires the relay's fake-ip lookup to the DNS server's pool.
func (rt *daemonRuntime) bindFakeIP() {
	relaySrv, _, dnsSrv := rt.services()
	if dnsSrv == nil || relaySrv == nil {
		return
	}
	prefix := dnsSrv.FakeIPRange()
	relaySrv.SetFakeIP(&prefix, dnsSrv.LookupFakeIP)
	relaySrv.SetRealIPLookup(dnsSrv.LookupRealIP)
}

// bindUDPRelayFakeIP keeps the UDP relay's fake-ip lookup in sync with DNS.
func (rt *daemonRuntime) bindUDPRelayFakeIP() {
	_, udpRelay, dnsSrv := rt.services()
	if dnsSrv == nil || udpRelay == nil {
		return
	}
	prefix := dnsSrv.FakeIPRange()
	udpRelay.SetFakeIP(&prefix, dnsSrv.LookupFakeIP)
}

// watchConfig polls the config file mtime and applies changes (belt; the
// /api/reload poke is the suspenders).
func (rt *daemonRuntime) watchConfig(ctx context.Context, a *App) {
	var lastMod time.Time
	var lastHotspotSync time.Time
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
			// Reconcile only hotspot ingress after Sharing restarts, including
			// a changed bridge address. Never fall back to the default LAN.
			live := a.getCfg()
			if live.Gateway.Enabled && live.Gateway.AccessMode == "hotspot" && time.Since(lastHotspotSync) >= 10*time.Second {
				lastHotspotSync = time.Now()
				if err := a.Gateway.Enable(firewallConfig(live)); err != nil {
					rt.logger.Warn("热点接管等待恢复", "err", err)
				}
			}
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
