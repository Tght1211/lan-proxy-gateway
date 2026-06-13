package console

import (
	"bytes"
	"net"
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

func TestDrawDashboardHopCollapsesWhenNoChain(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = oldNoColor }()

	// 没链式代理 → landing 会被 resolveHops 设为等于 takeoff。渲染应合并成一行
	// "🌐 出口节点"，而不是两行重复的"🛫 起飞 / 🛬 落地"。
	hop := proxyHop{name: "node-only", flag: "🇭🇰"}
	var buf bytes.Buffer
	drawDashboard(&buf, dashboardSnapshot{
		ok:       true,
		localIP:  "192.168.1.2",
		proxySrc: "机场订阅",
		takeoff:  hop,
		landing:  hop,
	}, true, nil, nil)
	out := buf.String()
	if !strings.Contains(out, "出口节点") {
		t.Fatalf("expected single-hop label '出口节点': %s", out)
	}
	if strings.Contains(out, "🛫 起飞") || strings.Contains(out, "🛬 落地") {
		t.Fatalf("no-chain mode should not show takeoff/landing labels: %s", out)
	}
}

func TestDrawDashboardHopShowsBothWhenChain(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = oldNoColor }()

	// 链式代理 → takeoff != landing → 两行都要显示
	var buf bytes.Buffer
	drawDashboard(&buf, dashboardSnapshot{
		ok:       true,
		localIP:  "192.168.1.2",
		proxySrc: "机场订阅",
		takeoff:  proxyHop{name: "airport-node", flag: "🇯🇵"},
		landing:  proxyHop{name: "residential", flag: "🇺🇸", hint: "1.2.3.4:443"},
	}, true, nil, nil)
	out := buf.String()
	for _, want := range []string{"🛫 起飞", "airport-node", "🛬 落地", "residential"} {
		if !strings.Contains(out, want) {
			t.Fatalf("chain mode missing %q: %s", want, out)
		}
	}
	if strings.Contains(out, "出口节点") {
		t.Fatalf("chain mode should not use single-hop label: %s", out)
	}
}

func TestDrawDashboardWarnsOnMixedPortDown(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = oldNoColor }()

	var buf bytes.Buffer
	drawDashboard(&buf, dashboardSnapshot{
		ok:            true,
		localIP:       "192.168.1.2",
		proxySrc:      "订阅",
		takeoff:       proxyHop{name: "a"},
		landing:       proxyHop{name: "a"},
		mixedPortDown: true,
	}, true, nil, nil)
	out := buf.String()
	if !strings.Contains(out, "代理端口不通") || !strings.Contains(out, "LAN 设备") {
		t.Fatalf("expected mixed-port-down warning: %s", out)
	}
}

func TestProbeMixedPortAcceptsLANIP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()

	_, portText, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}
	port, err := net.LookupPort("tcp", portText)
	if err != nil {
		t.Fatalf("LookupPort: %v", err)
	}

	if !probeMixedPort("127.0.0.1", port, time.Second) {
		t.Fatal("expected probeMixedPort to accept reachable LAN/local IP")
	}
	<-done
}
