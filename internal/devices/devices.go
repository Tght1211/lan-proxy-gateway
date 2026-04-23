// Package devices 把 LAN 设备 IP 翻译成人读的名字。
//
// 策略（按优先级）：
//  1. 用户手动标签（GatewayConfig.DeviceLabels）—— 最权威
//  2. 反向 DNS (PTR) —— 家用路由器+苹果设备通常会宣告 hostname
//  3. 查不到 → 空字符串（调用方落回显示纯 IP）
//
// 反向 DNS 走异步 + TTL 缓存：仪表盘每 2 秒刷新，不能每次都阻塞 200ms×N
// 个设备。同 IP 成功缓存 10 分钟，失败缓存 1 分钟（避免对不会响应 PTR 的
// 设备反复查）。
package devices

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"
)

// Resolver 是线程安全的 IP → 设备名解析器。零值不可用，请用 NewResolver。
type Resolver struct {
	labels  map[string]string // 用户手动标签（从 config 注入）
	mu      sync.RWMutex
	cache   map[string]cacheEntry // IP → PTR 结果
	pending map[string]bool       // 正在后台查的 IP，避免重复 goroutine
}

type cacheEntry struct {
	name    string
	expires time.Time
}

const (
	ttlHit  = 10 * time.Minute
	ttlMiss = 1 * time.Minute
	lookupTimeout = 250 * time.Millisecond
)

// NewResolver 初始化解析器。labels 可以是 nil（等于「没手动标签」）。
func NewResolver(labels map[string]string) *Resolver {
	return &Resolver{
		labels:  labels,
		cache:   make(map[string]cacheEntry),
		pending: make(map[string]bool),
	}
}

// SetLabels 热更新手动标签表（菜单里改了标签后调一下）。
func (r *Resolver) SetLabels(labels map[string]string) {
	r.mu.Lock()
	r.labels = labels
	r.mu.Unlock()
}

// LookupName 非阻塞：有缓存返缓存，没有就启后台 PTR 并先返空。仪表盘下一轮
// 刷新时命名就会出现，不影响首次渲染速度。
func (r *Resolver) LookupName(ip string) string {
	if ip == "" {
		return ""
	}
	r.mu.RLock()
	if r.labels != nil {
		if name, ok := r.labels[ip]; ok && name != "" {
			r.mu.RUnlock()
			return name
		}
	}
	if ent, ok := r.cache[ip]; ok && time.Now().Before(ent.expires) {
		r.mu.RUnlock()
		return ent.name
	}
	r.mu.RUnlock()

	r.mu.Lock()
	if r.pending[ip] {
		r.mu.Unlock()
		return ""
	}
	r.pending[ip] = true
	r.mu.Unlock()

	go r.lookupAsync(ip)
	return ""
}

func (r *Resolver) lookupAsync(ip string) {
	ctx, cancel := context.WithTimeout(context.Background(), lookupTimeout)
	defer cancel()
	var resolver net.Resolver
	names, err := resolver.LookupAddr(ctx, ip)
	name := ""
	ttl := ttlMiss
	if err == nil && len(names) > 0 {
		name = cleanHostname(names[0])
		if name != "" {
			ttl = ttlHit
		}
	}
	r.mu.Lock()
	r.cache[ip] = cacheEntry{name: name, expires: time.Now().Add(ttl)}
	delete(r.pending, ip)
	r.mu.Unlock()
}

// cleanHostname 把 PTR 结果修一修：去掉末尾 "."、常见后缀 ".local"、".lan"、
// ".home"，只留最前面的 label（「iPhone.local.」→「iPhone」）。
func cleanHostname(h string) string {
	h = strings.TrimSuffix(h, ".")
	if i := strings.IndexByte(h, '.'); i > 0 {
		h = h[:i]
	}
	return h
}
