package console

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/tght/lan-proxy-gateway/internal/ipinfo"
)

func TestDrawEgressLine(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = oldNoColor }()

	t.Run("renders info with location and ISP", func(t *testing.T) {
		var buf bytes.Buffer
		drawEgressLine(&buf, dashboardSnapshot{
			egress: &ipinfo.Info{
				IP: "203.0.113.9", City: "Los Angeles", Region: "California",
				Country: "US", Org: "AS7922 Comcast Cable Communications, LLC",
			},
			egressAge: 5 * time.Second,
		})
		out := buf.String()
		for _, want := range []string{"真实出口", "203.0.113.9", "Los Angeles", "California", "US", "Comcast Cable"} {
			if !strings.Contains(out, want) {
				t.Fatalf("missing %q in output: %s", want, out)
			}
		}
		if strings.Contains(out, "前的数据") {
			t.Fatalf("fresh data should not show stale hint: %s", out)
		}
	})

	t.Run("stale info shows age hint", func(t *testing.T) {
		var buf bytes.Buffer
		drawEgressLine(&buf, dashboardSnapshot{
			egress:    &ipinfo.Info{IP: "1.2.3.4", Country: "JP"},
			egressAge: 90 * time.Second,
		})
		if !strings.Contains(buf.String(), "前的数据") {
			t.Fatalf("expected stale hint: %s", buf.String())
		}
	})

	t.Run("pending shows querying placeholder", func(t *testing.T) {
		var buf bytes.Buffer
		drawEgressLine(&buf, dashboardSnapshot{egressPending: true})
		if !strings.Contains(buf.String(), "查询中") {
			t.Fatalf("expected querying placeholder: %s", buf.String())
		}
	})

	t.Run("error without info shows failure hint", func(t *testing.T) {
		var buf bytes.Buffer
		drawEgressLine(&buf, dashboardSnapshot{egressErr: "dial tcp 127.0.0.1:7890: connection refused"})
		if !strings.Contains(buf.String(), "查询失败") {
			t.Fatalf("expected failure hint: %s", buf.String())
		}
	})

	t.Run("nothing shown when no state", func(t *testing.T) {
		var buf bytes.Buffer
		drawEgressLine(&buf, dashboardSnapshot{})
		if buf.Len() != 0 {
			t.Fatalf("empty snap should draw nothing, got: %q", buf.String())
		}
	})
}

func TestEgressLocationDedupesCityRegion(t *testing.T) {
	got := egressLocation(&ipinfo.Info{City: "Hong Kong", Region: "Hong Kong", Country: "HK"})
	if got != "Hong Kong, HK" {
		t.Fatalf("expected City==Region to be deduped, got %q", got)
	}
}
