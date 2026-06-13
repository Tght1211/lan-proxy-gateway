// internal/console/health.go
package console

import (
	"fmt"
	"sync"
	"time"
)

// healthBar 是定长环形缓冲，每分钟记一次测速结果，渲染成 PAST→NOW 的健康柱。
//
// record 由后台 ticker goroutine 调用，render* 由主 dashboard 循环调用，
// 二者并发访问同一份状态，故全程持 mu 锁（go test -race 已固化回归）。
type healthBar struct {
	mu     sync.Mutex
	ok     []bool // 每格是否健康
	used   []bool // 该格是否已有记录
	size   int
	n      int
	lastAt time.Time // 最近一次 record 的时刻，用于倒计时与探测同源
}

func newHealthBar(size int) *healthBar {
	return &healthBar{ok: make([]bool, size), used: make([]bool, size), size: size}
}

func (h *healthBar) record(healthy bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	i := h.n % h.size
	h.ok[i] = healthy
	h.used[i] = true
	h.n++
	h.lastAt = time.Now()
}

// secsToNext 返回距下一次探测的秒数，与真实 record 节奏（每 60s）同源，
// 避免用墙钟整分钟另算导致显示与实际探测脱钩。未探测过时返回整周期。
func (h *healthBar) secsToNext() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.lastAt.IsZero() {
		return 60
	}
	s := 60 - int(time.Since(h.lastAt).Seconds())
	if s < 0 {
		s = 0
	}
	if s > 60 {
		s = 60
	}
	return s
}

// renderTitle 渲染标题：左『近 N 次记录』，右『NNs 后刷新』。
func (h *healthBar) renderTitle(secsToNext int) string {
	return fmt.Sprintf("近 %d 次记录%s%ds 后刷新", h.size, strings_pad(), secsToNext)
}

// renderBar 渲染 N 根竖条：健康=▮，异常=▯，无记录=空格。
func (h *healthBar) renderBar() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	r := make([]rune, h.size)
	start := 0
	if h.n > h.size {
		start = h.n % h.size
	}
	for i := 0; i < h.size; i++ {
		idx := (start + i) % h.size
		switch {
		case !h.used[idx]:
			r[i] = ' '
		case h.ok[idx]:
			r[i] = '▮'
		default:
			r[i] = '▯'
		}
	}
	return string(r)
}

func (h *healthBar) renderFooter() string {
	pad := h.size - len("PAST") - len("NOW")
	if pad < 1 {
		pad = 1
	}
	return "PAST" + spaces(pad) + "NOW"
}

func spaces(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}

func strings_pad() string { return "   " }
