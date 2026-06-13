// internal/console/health.go
package console

import "fmt"

// healthBar 是定长环形缓冲，每分钟记一次测速结果，渲染成 PAST→NOW 的健康柱。
type healthBar struct {
	ok   []bool // 每格是否健康
	used []bool // 该格是否已有记录
	size int
	n    int
}

func newHealthBar(size int) *healthBar {
	return &healthBar{ok: make([]bool, size), used: make([]bool, size), size: size}
}

func (h *healthBar) record(healthy bool) {
	i := h.n % h.size
	h.ok[i] = healthy
	h.used[i] = true
	h.n++
}

// renderTitle 渲染标题：左『近 N 次记录』，右『NNs 后刷新』。
func (h *healthBar) renderTitle(secsToNext int) string {
	return fmt.Sprintf("近 %d 次记录%s%ds 后刷新", h.size, strings_pad(), secsToNext)
}

// renderBar 渲染 60 根竖条：健康=▮，异常=▯，无记录=空格。
func (h *healthBar) renderBar() string {
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
