package cmd

import (
	"math"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/mihomo"
)

func TestMatchRegions(t *testing.T) {
	mapping := map[string][]string{
		"HK": []string{"香港", "HK"},
		"JP": []string{"日本", "Tokyo"},
	}

	matched := matchRegions("高速-HK-01", mapping, []string{"HK", "JP"})
	if len(matched) != 1 || matched[0] != "HK" {
		t.Fatalf("unexpected matched regions: %#v", matched)
	}
}

func TestLatestDelay(t *testing.T) {
	history := []mihomo.DelayHistory{{Delay: 230}, {Delay: 120}, {Delay: 150}}
	if got := latestDelay(history); got != 120 {
		t.Fatalf("expected 120, got %d", got)
	}
	if got := latestDelay(nil); got != math.MaxInt {
		t.Fatalf("expected MaxInt for empty history, got %d", got)
	}
}

func TestResolveSwitchRegions(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Regions.Include = []string{"HK", "JP"}

	switchRegionCodes = "SG,HK,UNKNOWN"
	defer func() { switchRegionCodes = "" }()

	regions := resolveSwitchRegions(cfg)
	if len(regions) != 2 {
		t.Fatalf("expected 2 valid regions, got %d", len(regions))
	}
	if regions[0] != "SG" || regions[1] != "HK" {
		t.Fatalf("unexpected regions order: %#v", regions)
	}
}

func TestSelectBestCandidate(t *testing.T) {
	candidates := []candidateNode{
		{Name: "A", Alive: false, Delay: 30},
		{Name: "B", Alive: true, Delay: 200},
		{Name: "C", Alive: true, Delay: 80},
	}
	best := selectBestCandidate(candidates)
	if best.Name != "C" {
		t.Fatalf("expected C, got %s", best.Name)
	}
}
