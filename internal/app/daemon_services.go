package app

import (
	"context"
	"log/slog"
	"net"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/dns"
	"github.com/tght/lan-proxy-gateway/internal/relay"
)

// startServices launches relay, DNS and the loopback API for the current config.
func (a *App) startServices(ctx context.Context, logger *slog.Logger, origDST relay.OrigDSTResolver) (*daemonRuntime, error) {
	a.migrateLearnedGroups()
	rt := &daemonRuntime{
		logger:      logger,
		eventCh:     make(chan serviceEvent, 4),
		crashCounts: make(map[string]int),
	}
	rt.tracker = relay.NewTracker()
	if err := rt.tracker.EnableHistory(filepath.Join(a.Paths.Root, "usage-history.json")); err != nil {
		return nil, err
	}
	rt.tracker.StartSampling(ctx, 5*time.Second)
	go func() {
		tick := time.NewTicker(5 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				if err := rt.tracker.SaveHistory(); err != nil {
					logger.Error("保存流量历史失败", "err", err)
				}
				return
			case <-tick.C:
				if err := rt.tracker.SaveHistory(); err != nil {
					logger.Error("保存流量历史失败", "err", err)
				}
			}
		}
	}()

	rt.learner = newFallbackLearner(filepath.Join(a.Paths.Root, "fallback-learn.json"), logger)

	dialer, err := buildDialer(a.Cfg.Egress)
	if err != nil {
		return nil, err
	}
	rt.relay = relay.New(relay.Options{
		ListenAddr: net.JoinHostPort("", strconv.Itoa(a.Cfg.Runtime.RedirPort)),
		OrigDST:    origDST,
		Tracker:    rt.tracker,
		Dialer:     dialer,
		ViaProxy:   a.Cfg.Egress.Mode == config.EgressProxy,
		OnFallbackSuccess: func(host string) {
			srv, _, _ := rt.services()
			if srv != nil && !srv.EgressHealth().ProxyDown {
				rt.learner.Record(host)
			}
		},
		Logger: logger,
	})
	rt.relay.SetProxyFailAction(a.Cfg.ProxyFailure.Action)
	rt.applyRouting(a.Cfg)
	rt.startEgressMonitor(ctx, a, logger)
	if a.Cfg.DNS.Enabled {
		rt.dns = dns.New(dnsOptions(a.Cfg, a.Paths.FakeIPCacheFile, logger))
		rt.bindFakeIP()
		go func() {
			if err := rt.dns.ListenAndServe(ctx); err != nil && ctx.Err() == nil {
				rt.eventCh <- serviceEvent{name: "dns", err: err}
			}
		}()
	}
	// Start UDP relay for fake-IP range (game voice, video calls).
	if a.Cfg.Egress.Mode == config.EgressProxy && a.Cfg.DNS.FakeIP && rt.dns != nil {
		fakeRange := rt.dns.FakeIPRange()
		rt.udpRelay = relay.NewUDPRelay(relay.UDPRelayOptions{
			ListenAddr:   relay.FormatUDPListenAddr(a.Cfg.Runtime.UDPRedirPort),
			FakeIPRange:  &fakeRange,
			LookupFakeIP: rt.dns.LookupFakeIP,
			Resolve:      relay.NewUpstreamResolver(a.Cfg.DNS.Upstreams),
			Tracker:      rt.tracker,
			Logger:       logger,
		})
		rt.syncUDPRelayRouting(a.Cfg)
		go func() {
			if err := rt.udpRelay.ListenAndServe(ctx); err != nil && ctx.Err() == nil {
				rt.eventCh <- serviceEvent{name: "udp-relay", err: err}
			}
		}()
	}
	go func() {
		if err := rt.relay.ListenAndServe(ctx); err != nil && ctx.Err() == nil {
			rt.eventCh <- serviceEvent{name: "relay", err: err}
		}
	}()
	rt.api = newAPIServer(a, rt)
	go func() {
		if err := rt.api.ListenAndServe(ctx); err != nil && ctx.Err() == nil {
			rt.eventCh <- serviceEvent{name: "api", err: err}
		}
	}()
	go rt.watchConfig(ctx, a)
	return rt, nil
}

// serviceEvent signals the main loop when a sub-service goroutine exits.
type serviceEvent struct {
	name string
	err  error
}

// daemonRuntime bundles the long-running services for shutdown/reload.
type daemonRuntime struct {
	logger  *slog.Logger
	tracker *relay.Tracker
	learner *fallbackLearner
	eventCh chan serviceEvent // sub-service crash notifications

	// mu guards the pointers swapped by restartService and the crash counter
	// map, both read from API handler goroutines while the main loop writes.
	mu          sync.RWMutex
	relay       *relay.Server
	udpRelay    *relay.UDPRelay
	dns         *dns.Server
	api         *apiServer
	crashCounts map[string]int
}

// services returns a consistent snapshot of the swappable service pointers.
func (rt *daemonRuntime) services() (*relay.Server, *relay.UDPRelay, *dns.Server) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.relay, rt.udpRelay, rt.dns
}

// bumpCrash increments and returns the crash count for a service.
func (rt *daemonRuntime) bumpCrash(name string) int {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.crashCounts[name]++
	return rt.crashCounts[name]
}

// componentHealth returns the health of each core sub-service.
func (rt *daemonRuntime) componentHealth() []ComponentHealth {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	var out []ComponentHealth
	out = append(out, ComponentHealth{
		Name: "relay", Running: rt.relay != nil, Crashes: rt.crashCounts["relay"],
	})
	if rt.dns != nil {
		out = append(out, ComponentHealth{
			Name: "dns", Running: true, Crashes: rt.crashCounts["dns"],
		})
	}
	if rt.udpRelay != nil {
		out = append(out, ComponentHealth{
			Name: "udp-relay", Running: true, Crashes: rt.crashCounts["udp-relay"],
		})
	}
	out = append(out, ComponentHealth{
		Name: "api", Running: rt.api != nil, Crashes: rt.crashCounts["api"],
	})
	return out
}

func (rt *daemonRuntime) restartService(ctx context.Context, a *App, name string) {
	rt.logger.Info("正在重启子服务", "service", name)
	switch name {
	case "dns":
		if rt.dns != nil {
			rt.dns.Shutdown()
			newDNS := dns.New(dnsOptions(a.Cfg, a.Paths.FakeIPCacheFile, rt.logger))
			rt.mu.Lock()
			rt.dns = newDNS
			rt.mu.Unlock()
			rt.bindFakeIP()
			rt.bindUDPRelayFakeIP()
			go func() {
				if err := newDNS.ListenAndServe(ctx); err != nil && ctx.Err() == nil {
					rt.eventCh <- serviceEvent{name: "dns", err: err}
				}
			}()
			rt.logger.Info("DNS 服务已重启")
		}
	case "relay":
		if rt.relay != nil {
			origDST, err := relay.NewPlatformOrigDST()
			if err != nil {
				rt.logger.Error("重启 relay 失败: 无法初始化 OrigDST", "err", err)
				return
			}
			_ = rt.relay.Close()
			dialer, err := buildDialer(a.Cfg.Egress)
			if err != nil {
				rt.logger.Error("重启 relay 失败: 无法构建 dialer", "err", err)
				return
			}
			newRelay := relay.New(relay.Options{
				ListenAddr: net.JoinHostPort("", strconv.Itoa(a.Cfg.Runtime.RedirPort)),
				OrigDST:    origDST,
				Tracker:    rt.tracker,
				Dialer:     dialer,
				ViaProxy:   a.Cfg.Egress.Mode == config.EgressProxy,
				OnFallbackSuccess: func(host string) {
					srv, _, _ := rt.services()
					if srv != nil && !srv.EgressHealth().ProxyDown {
						rt.learner.Record(host)
					}
				},
				Logger: rt.logger,
			})
			newRelay.SetProxyFailAction(a.Cfg.ProxyFailure.Action)
			rt.mu.Lock()
			rt.relay = newRelay
			rt.mu.Unlock()
			rt.applyRouting(a.Cfg)
			rt.bindFakeIP()
			go func() {
				if err := newRelay.ListenAndServe(ctx); err != nil && ctx.Err() == nil {
					rt.eventCh <- serviceEvent{name: "relay", err: err}
				}
			}()
			rt.logger.Info("relay 服务已重启")
		}
	case "udp-relay":
		if rt.udpRelay != nil && rt.dns != nil {
			_ = rt.udpRelay.Close()
			fakeRange := rt.dns.FakeIPRange()
			newUDP := relay.NewUDPRelay(relay.UDPRelayOptions{
				ListenAddr:   relay.FormatUDPListenAddr(a.Cfg.Runtime.UDPRedirPort),
				FakeIPRange:  &fakeRange,
				LookupFakeIP: rt.dns.LookupFakeIP,
				Resolve:      relay.NewUpstreamResolver(a.Cfg.DNS.Upstreams),
				Tracker:      rt.tracker,
				Logger:       rt.logger,
			})
			rt.mu.Lock()
			rt.udpRelay = newUDP
			rt.mu.Unlock()
			rt.syncUDPRelayRouting(a.Cfg)
			go func() {
				if err := newUDP.ListenAndServe(ctx); err != nil && ctx.Err() == nil {
					rt.eventCh <- serviceEvent{name: "udp-relay", err: err}
				}
			}()
			rt.logger.Info("UDP relay 服务已重启")
		}
	case "api":
		if rt.api != nil {
			_ = rt.api.Close()
			newAPI := newAPIServer(a, rt)
			rt.mu.Lock()
			rt.api = newAPI
			rt.mu.Unlock()
			go func() {
				if err := newAPI.ListenAndServe(ctx); err != nil && ctx.Err() == nil {
					rt.eventCh <- serviceEvent{name: "api", err: err}
				}
			}()
			rt.logger.Info("状态 API 已重启")
		}
	}
}

func (rt *daemonRuntime) shutdown() {
	if rt.relay != nil {
		_ = rt.relay.Close()
	}
	if rt.udpRelay != nil {
		_ = rt.udpRelay.Close()
	}
	if rt.dns != nil {
		rt.dns.Shutdown()
	}
	if rt.api != nil {
		_ = rt.api.Close()
	}
}
