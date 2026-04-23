package app

import (
	"context"
	"sync"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/source"
)

// SourceHealth 是「代理源健康看板」，supervisor 写、UI 读。
// 当 Healthy=false 且 FallbackActive=true 时，意味着我们已经通过 mihomo API
// 把 mode 强切到 direct（LAN 设备能继续上网，但走的是直连），用户在主菜单
// 应该能看到醒目告警。
type SourceHealth struct {
	Healthy        bool
	LastError      string
	FallbackActive bool // 是否因源异常被迫进入 direct
	OriginalMode   string
	CheckedAt      time.Time
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

// StartSupervisor 启一个后台 goroutine，周期性检查代理源；
// 代理源挂了自动切到 direct，恢复自动切回。
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
// 源异常时 fallback 到 direct（通过 mihomo API）；恢复时切回用户原本的 mode。
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
	})

	prev := a.health.snapshot()
	now := time.Now()

	if err != nil {
		errMsg := err.Error()
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
