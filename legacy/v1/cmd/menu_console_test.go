package cmd

import (
	"testing"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestCurrentConsoleNodeCachedReturnsLoadingWhenNoCache(t *testing.T) {
	runtimeNodeCache = runtimeConsoleNodeCache{}

	cfg := config.DefaultConfig()
	got := currentConsoleNodeCached(cfg)
	if got != "加载中..." {
		t.Fatalf("currentConsoleNodeCached() = %q, want 加载中...", got)
	}
}

func TestCurrentConsoleNodeCachedReturnsCachedValueWithoutBlocking(t *testing.T) {
	runtimeNodeCache = runtimeConsoleNodeCache{
		value:     "Auto",
		hasValue:  true,
		updatedAt: time.Now(),
	}

	cfg := config.DefaultConfig()
	got := currentConsoleNodeCached(cfg)
	if got != "Auto" {
		t.Fatalf("currentConsoleNodeCached() = %q, want Auto", got)
	}
}

func TestLoadUpdateNoticeCachedReturnsCachedValue(t *testing.T) {
	runtimeNoticeCache = runtimeConsoleNoticeCache{
		loaded:   true,
		loadedAt: time.Now(),
		value: &updateNotice{
			Current: "v1.0.0",
			Latest:  "v1.1.0",
		},
	}

	notice := loadUpdateNoticeCached()
	if notice == nil || notice.Latest != "v1.1.0" {
		t.Fatalf("loadUpdateNoticeCached() unexpected result: %+v", notice)
	}
}
