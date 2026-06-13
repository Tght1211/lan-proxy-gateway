// internal/console/health_test.go
package console

import (
	"strings"
	"sync"
	"testing"
)

// TestHealthBarConcurrentRecordRender 固化 record(ticker goroutine) 与
// render(主循环) 的并发安全。须配合 `go test -race` 才能抓到回归。
func TestHealthBarConcurrentRecordRender(t *testing.T) {
	h := newHealthBar(60)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			h.record(i%2 == 0)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_ = h.renderBar()
			_ = h.secsToNext()
		}
	}()
	wg.Wait()
}

func TestHealthBarRecordsAndRenders(t *testing.T) {
	h := newHealthBar(60)
	// 记 3 次健康
	for i := 0; i < 3; i++ {
		h.record(true)
	}
	body := h.renderBar()
	if len([]rune(body)) != 60 {
		t.Fatalf("应渲染 60 格，得到 %d", len([]rune(body)))
	}
}

func TestHealthBarTitleHasCountdown(t *testing.T) {
	h := newHealthBar(60)
	title := h.renderTitle(42)
	if !strings.Contains(title, "近 60 次记录") {
		t.Fatalf("标题应含『近 60 次记录』: %q", title)
	}
	if !strings.Contains(title, "42") {
		t.Fatalf("标题应含倒计时秒数: %q", title)
	}
}

func TestHealthBarFooterPastNow(t *testing.T) {
	h := newHealthBar(60)
	f := h.renderFooter()
	if !strings.Contains(f, "PAST") || !strings.Contains(f, "NOW") {
		t.Fatalf("脚注应含 PAST/NOW: %q", f)
	}
}
