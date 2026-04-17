package cmd

import (
	"strings"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestParseSubscriptionHintItemRecognizesCommonHints(t *testing.T) {
	tests := []struct {
		input     string
		wantKind  string
		wantValue string
	}{
		{input: "剩余流量：41.45 GB", wantKind: "remaining", wantValue: "41.45 GB"},
		{input: "套餐到期: 2027-03-28", wantKind: "expiry", wantValue: "2027-03-28"},
		{input: "最新网址：fliggycloud.cc", wantKind: "website", wantValue: "fliggycloud.cc"},
		{input: "距离下次重置剩余：29 天", wantKind: "reset", wantValue: "29 天"},
	}

	for _, tc := range tests {
		kind, value, ok := parseSubscriptionHintItem(tc.input)
		if !ok {
			t.Fatalf("parseSubscriptionHintItem(%q) should match", tc.input)
		}
		if kind != tc.wantKind || value != tc.wantValue {
			t.Fatalf("parseSubscriptionHintItem(%q) = (%q, %q), want (%q, %q)", tc.input, kind, value, tc.wantKind, tc.wantValue)
		}
	}

	if _, _, ok := parseSubscriptionHintItem("香港 02"); ok {
		t.Fatal("expected regular node name to stay selectable")
	}
}

func TestSimpleSelectableNodesFiltersSubscriptionHints(t *testing.T) {
	nodes := []string{
		"最新网址：fliggycloud.cc",
		"香港 01",
		"剩余流量：41.45 GB",
		"香港 02",
		"香港 02",
		"距离下次重置剩余：29 天",
	}

	got := simpleSelectableNodes(nodes)
	if len(got) != 2 {
		t.Fatalf("len(simpleSelectableNodes()) = %d, want 2 (%v)", len(got), got)
	}
	if got[0] != "香港 01" || got[1] != "香港 02" {
		t.Fatalf("simpleSelectableNodes() = %v, want only real nodes", got)
	}
}

func TestSortSimpleNodeDelayEntriesOrdersByDelay(t *testing.T) {
	entries := []simpleNodeDelayEntry{
		{Name: "香港 03", Delay: -1, DelayLabel: "失败", Reachable: false},
		{Name: "香港 02", Delay: 86, DelayLabel: "86ms", Reachable: true},
		{Name: "香港 01", Delay: 42, DelayLabel: "42ms", Reachable: true, Current: true},
		{Name: "香港 04", Delay: -1, DelayLabel: "超时", Reachable: false, Current: true},
	}

	sortSimpleNodeDelayEntries(entries)

	want := []string{"香港 01", "香港 02", "香港 04", "香港 03"}
	for i, name := range want {
		if entries[i].Name != name {
			t.Fatalf("sorted entry %d = %q, want %q", i, entries[i].Name, name)
		}
	}
}

func TestBuildSimpleNodeMeasureSummary(t *testing.T) {
	summary := buildSimpleNodeMeasureSummary([]simpleNodeDelayEntry{
		{Name: "a", Reachable: false},
		{Name: "b", Reachable: true},
		{Name: "c", Reachable: false},
	})

	if summary.Total != 3 || summary.Success != 1 || summary.Failure != 2 || summary.AllFailed {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	empty := buildSimpleNodeMeasureSummary(nil)
	if empty.Total != 0 || empty.Success != 0 || empty.Failure != 0 || empty.AllFailed {
		t.Fatalf("unexpected empty summary: %+v", empty)
	}
}

func TestDetectSimpleNodeSourceKind(t *testing.T) {
	cfgURL := config.DefaultConfig()
	cfgURL.Proxy.Source = "url"
	cfgURL.Proxy.SubscriptionURL = "https://example.com/sub"
	if got := detectSimpleNodeSourceKind(cfgURL); got != "url" {
		t.Fatalf("detectSimpleNodeSourceKind(url) = %q, want url", got)
	}

	cfgFile := config.DefaultConfig()
	cfgFile.Proxy.Source = "file"
	cfgFile.Proxy.SubscriptionURL = ""
	cfgFile.Proxy.ConfigFile = "/tmp/demo.yaml"
	if got := detectSimpleNodeSourceKind(cfgFile); got != "file" {
		t.Fatalf("detectSimpleNodeSourceKind(file) = %q, want file", got)
	}

	cfgFallback := config.DefaultConfig()
	cfgFallback.Proxy.Source = ""
	cfgFallback.Proxy.SubscriptionURL = "https://example.com/sub2"
	if got := detectSimpleNodeSourceKind(cfgFallback); got != "url" {
		t.Fatalf("detectSimpleNodeSourceKind(fallback) = %q, want url", got)
	}
}

func TestBuildSimpleNodeConnectivityHint(t *testing.T) {
	summaryAllFailed := simpleNodeMeasureSummary{Total: 5, Success: 0, Failure: 5, AllFailed: true}

	cfgURL := config.DefaultConfig()
	cfgURL.Proxy.Source = "url"
	cfgURL.Proxy.SubscriptionURL = "https://demo.example.com/sub"
	if hint := buildSimpleNodeConnectivityHint(cfgURL, summaryAllFailed, 1); hint != "" {
		t.Fatalf("hint should be empty when all-failed count < 2, got: %q", hint)
	}
	if hint := buildSimpleNodeConnectivityHint(cfgURL, summaryAllFailed, 2); hint == "" || !containsAll(hint, []string{"订阅链接", "demo.example.com"}) {
		t.Fatalf("unexpected url hint: %q", hint)
	}

	cfgFile := config.DefaultConfig()
	cfgFile.Proxy.Source = "file"
	cfgFile.Proxy.SubscriptionURL = ""
	cfgFile.Proxy.ConfigFile = "C:/proxy.yaml"
	if hint := buildSimpleNodeConnectivityHint(cfgFile, summaryAllFailed, 2); hint == "" || !containsAll(hint, []string{"本地代理文件", "订阅文件"}) {
		t.Fatalf("unexpected file hint: %q", hint)
	}

	summaryPartial := simpleNodeMeasureSummary{Total: 5, Success: 1, Failure: 4, AllFailed: false}
	if hint := buildSimpleNodeConnectivityHint(cfgURL, summaryPartial, 3); hint != "" {
		t.Fatalf("hint should be empty when not all failed, got: %q", hint)
	}
}

func containsAll(s string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}
