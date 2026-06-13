// internal/console/sparkline.go
package console

// sparkline 是定长环形缓冲，用 Unicode 区块字符渲染成柱状图。
type sparkline struct {
	buf  []float64
	size int
	n    int // 已写入总数（>size 时取最近 size 个）
}

var sparkBlocks = []rune("▁▂▃▄▅▆▇█")

func newSparkline(size int) *sparkline {
	return &sparkline{buf: make([]float64, size), size: size}
}

func (s *sparkline) push(v float64) {
	if v < 0 {
		v = 0
	}
	s.buf[s.n%s.size] = v
	s.n++
}

// ordered 返回按时间从旧到新排列的窗口值。
func (s *sparkline) ordered() []float64 {
	out := make([]float64, 0, s.size)
	count := s.n
	if count > s.size {
		count = s.size
	}
	start := 0
	if s.n > s.size {
		start = s.n % s.size
	}
	for i := 0; i < count; i++ {
		out = append(out, s.buf[(start+i)%s.size])
	}
	// 不足 size 时左侧补零，保证渲染宽度恒定
	for len(out) < s.size {
		out = append([]float64{0}, out...)
	}
	return out
}

func (s *sparkline) render() string {
	vals := s.ordered()
	max := 0.0
	for _, v := range vals {
		if v > max {
			max = v
		}
	}
	r := make([]rune, len(vals))
	for i, v := range vals {
		idx := 0
		if max > 0 {
			idx = int(v / max * float64(len(sparkBlocks)-1))
			if idx >= len(sparkBlocks) {
				idx = len(sparkBlocks) - 1
			}
		}
		r[i] = sparkBlocks[idx]
	}
	return string(r)
}
