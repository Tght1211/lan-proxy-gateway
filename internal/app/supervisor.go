package app

import (
	"context"
	"sync"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/source"
)

// SourceHealth 是「代理源健康看板」，supervisor 写、UI 读。
// 当 Healthy=false 时，源健康探测失败。只有 FallbackActive=true 才意味着
// supervisor 已经通过 mihomo API 把 mode 强切到 direct。
type SourceHealth struct {
	Healthy        bool
	LastError      string
	FallbackActive bool // 是否因源异常被迫进入 direct
	OriginalMode   string
	CheckedAt      time.Time
	FailCount      int
}

type healthState struct {
	mu sync.RWMutex
	h  SourceHealth
}

func (s *healthState) snapshot() SourceHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.h
}

func (s *healthState) set(h SourceHealth) {
	s.mu.Lock()
	s.h = h
	s.mu.Unlock()
}

// Health 返回当前代理源健康状态快照，UI 层用于显示告警。
func (a *App) Health() SourceHealth {
	if a.health == nil {
		return SourceHealth{}
	}
	return a.health.snapshot()
}

// StartSupervisor 启一个后台 goroutine，周期性检查代理源。
// 普通订阅/文件源异常时自动切到 direct；本机单点代理只告警，不自动改 mode，
// 避免健康探测波动反过来干扰用户正在测试的本机代理链路。
// 重复调用是安全的（第二次会 no-op，通过 supervisorStarted 标记）。
func (a *App) StartSupervisor(ctx context.Context) {
	if a.health == nil {
		a.health = &healthState{}
	}
	a.supervisorOnce.Do(func() {
		go a.supervisorLoop(ctx)
	})
}

const (
	supervisorInterval = 30 * time.Second
	supervisorTimeout  = 5 * time.Second
	supervisorMaxFails = 2
)

func (a *App) supervisorLoop(ctx context.Context) {
	// 先做一次即时检测，别等 30 秒。
	a.checkSourceHealth(ctx)

	t := time.NewTicker(supervisorInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.checkSourceHealth(ctx)
		}
	}
}

// checkSourceHealth 执行一次 source.Test，并在必要时触发 fallback / restore。
// 普通源异常时 fallback 到 direct（通过 mihomo API）；恢复时切回用户原本的 mode。
// 注意：fallback 不修改 a.Cfg.Traffic.Mode（用户视角 mode 没变），只是运行时
// 临时覆盖，这样恢复时能无损还原。
func (a *App) checkSourceHealth(ctx context.Context) {
	if a.Engine == nil || !a.Engine.Running() {
		// mihomo 没跑，无从判断也无从 fallback，状态置空。
		a.health.set(SourceHealth{})
		return
	}
	// SourceTypeNone：用户主动选「全部直连」，没有源要测，视为永远健康。
	if a.Cfg.Source.Type == config.SourceTypeNone {
		a.health.set(SourceHealth{Healthy: true, CheckedAt: time.Now()})
		return
	}

	testCtx, cancel := context.WithTimeout(ctx, supervisorTimeout)
	defer cancel()
	err := source.TestWithOptions(testCtx, a.Cfg.Source, source.TestOptions{
		SubscriptionProxyURL: source.LocalMixedProxyURL(a.Cfg.Runtime.Ports.Mixed),
		ProxyTCPOnly:         config.UsesLocalExternalProxy(a.Cfg),
	})

	prev := a.health.snapshot()
	now := time.Now()

	if err != nil {
		errMsg := err.Error()
		failCount := prev.FailCount + 1
		if failCount < supervisorMaxFails {
			a.health.set(SourceHealth{
				Healthy:        false,
				LastError:      errMsg,
				FallbackActive: prev.FallbackActive,
				OriginalMode:   prev.OriginalMode,
				CheckedAt:      now,
				FailCount:      failCount,
			})
			return
		}
		if config.UsesLocalExternalProxy(a.Cfg) {
			a.health.set(SourceHealth{
				Healthy:        false,
				LastError:      errMsg,
				FallbackActive: false,
				OriginalMode:   prev.OriginalMode,
				CheckedAt:      now,
				FailCount:      failCount,
			})
			return
		}
		// 还没 fallback：切到 direct 保住 LAN 通网。
		if !prev.FallbackActive {
			apiCtx, cancelAPI := context.WithTimeout(ctx, supervisorTimeout)
			defer cancelAPI()
			originalMode := a.Cfg.Traffic.Mode
			if switchErr := a.Engine.API().SetMode(apiCtx, config.ModeDirect); switchErr == nil {
				a.health.set(SourceHealth{
					Healthy:        false,
					LastError:      errMsg,
					FallbackActive: true,
					OriginalMode:   originalMode,
					CheckedAt:      now,
					FailCount:      failCount,
				})
				return
			}
			// 切 mode 失败：依然记录源异常状态，但 FallbackActive 保持 false，
			// 下次 tick 会再试。
		}
		// 已经 fallback 了：只刷新错误信息和时间
		prev.Healthy = false
		prev.LastError = errMsg
		prev.CheckedAt = now
		prev.FailCount = failCount
		a.health.set(prev)
		return
	}

	// 源健康：如果之前 fallback 过，切回原 mode。
	if prev.FallbackActive {
		apiCtx, cancelAPI := context.WithTimeout(ctx, supervisorTimeout)
		defer cancelAPI()
		target := prev.OriginalMode
		if target == "" {
			target = a.Cfg.Traffic.Mode
		}
		_ = a.Engine.API().SetMode(apiCtx, target)
	}
	a.health.set(SourceHealth{
		Healthy:   true,
		CheckedAt: now,
	})
}
