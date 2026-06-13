// internal/console/sparkline_test.go
package console

import (
	"strings"
	"testing"
)

func TestSparklinePushAndRender(t *testing.T) {
	s := newSparkline(8)
	for _, v := range []float64{0, 1, 2, 3, 4, 5, 6, 7} {
		s.push(v)
	}
	out := s.render()
	runes := []rune(out)
	if len(runes) != 8 {
		t.Fatalf("应渲染 8 格，得到 %d", len(runes))
	}
	// 最大值映射到最高块 █，最小值映射到最低块 ▁
	if runes[7] != '█' {
		t.Fatalf("最大值应是 █，得到 %q", string(runes[7]))
	}
	if runes[0] != '▁' {
		t.Fatalf("最小值应是 ▁，得到 %q", string(runes[0]))
	}
}

func TestSparklineRingBufferCaps(t *testing.T) {
	s := newSparkline(4)
	for i := 0; i < 10; i++ {
		s.push(float64(i))
	}
	if r := []rune(s.render()); len(r) != 4 {
		t.Fatalf("环形缓冲应固定 4 格，得到 %d", len(r))
	}
}

func TestSparklineAllZero(t *testing.T) {
	s := newSparkline(4)
	for i := 0; i < 4; i++ {
		s.push(0)
	}
	if strings.Trim(s.render(), "▁") != "" {
		t.Fatal("全零应全是最低块，不应除零崩")
	}
}
