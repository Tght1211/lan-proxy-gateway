package devices

import (
	"testing"
	"time"
)

func TestLabelPriority(t *testing.T) {
	r := NewResolver(map[string]string{"192.168.1.23": "Switch"})
	if got := r.LookupName("192.168.1.23"); got != "Switch" {
		t.Errorf("want Switch, got %q", got)
	}
}

func TestEmptyIP(t *testing.T) {
	r := NewResolver(nil)
	if got := r.LookupName(""); got != "" {
		t.Errorf("empty ip should return empty, got %q", got)
	}
}

func TestCacheHit(t *testing.T) {
	r := NewResolver(nil)
	// 手动塞缓存，模拟异步完成
	r.mu.Lock()
	r.cache["1.2.3.4"] = cacheEntry{name: "cached", expires: time.Now().Add(time.Minute)}
	r.mu.Unlock()
	if got := r.LookupName("1.2.3.4"); got != "cached" {
		t.Errorf("want cached, got %q", got)
	}
}

func TestCleanHostname(t *testing.T) {
	cases := map[string]string{
		"iPhone.local.":   "iPhone",
		"MacBook-Pro":     "MacBook-Pro",
		"switch.lan.":     "switch",
		"":                "",
		".":               "",
	}
	for in, want := range cases {
		if got := cleanHostname(in); got != want {
			t.Errorf("cleanHostname(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSetLabels(t *testing.T) {
	r := NewResolver(nil)
	r.SetLabels(map[string]string{"10.0.0.1": "Router"})
	if got := r.LookupName("10.0.0.1"); got != "Router" {
		t.Errorf("after SetLabels want Router, got %q", got)
	}
}
