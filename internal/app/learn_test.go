package app

import (
	"log/slog"
	"testing"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func newTestLearner(t *testing.T, path string, now time.Time) (*fallbackLearner, *time.Time, *[]string) {
	t.Helper()
	promoted := &[]string{}
	l := newFallbackLearner(path, slog.Default())
	l.now = func() time.Time { return now }
	l.promote = func(host string) { *promoted = append(*promoted, host) }
	return l, &now, promoted
}

func TestFallbackLearnerThreshold(t *testing.T) {
	l, _, promoted := newTestLearner(t, t.TempDir()+"/learn.json", time.Now())

	l.Record("example.com")
	l.Record("example.com")
	if len(*promoted) != 0 {
		t.Fatalf("promoted after 2 records: %v", *promoted)
	}
	l.Record("EXAMPLE.com.")
	if *promoted == nil || len(*promoted) != 1 || (*promoted)[0] != "example.com" {
		t.Fatalf("promoted = %v, want [example.com] (case/trailing-dot normalized)", *promoted)
	}
	// counts reset after promotion: one more success must not re-promote
	l.Record("example.com")
	if len(*promoted) != 1 {
		t.Fatalf("promoted = %v after reset, want 1 entry", *promoted)
	}
}

func TestFallbackLearnerWindowExpiry(t *testing.T) {
	start := time.Now()
	l, nowPtr, promoted := newTestLearner(t, t.TempDir()+"/learn.json", start)

	l.Record("example.com")
	*nowPtr = start.Add(10 * time.Hour)
	l.Record("example.com")
	// 25h later the first record is outside the 24h window
	*nowPtr = start.Add(25 * time.Hour)
	l.Record("example.com")
	if len(*promoted) != 0 {
		t.Fatalf("promoted with expired record counted: %v", *promoted)
	}
	l.Record("example.com")
	if len(*promoted) != 1 {
		t.Fatalf("promoted = %v, want promotion after 3 in-window successes", *promoted)
	}
}

func TestFallbackLearnerPersistence(t *testing.T) {
	path := t.TempDir() + "/learn.json"
	start := time.Now()
	l1, _, promoted1 := newTestLearner(t, path, start)
	l1.Record("example.com")
	l1.Record("example.com")
	if len(*promoted1) != 0 {
		t.Fatalf("unexpected promote: %v", *promoted1)
	}

	// daemon restart: a fresh learner on the same file keeps the counts
	l2, _, promoted2 := newTestLearner(t, path, start.Add(time.Hour))
	l2.Record("example.com")
	if len(*promoted2) != 1 || (*promoted2)[0] != "example.com" {
		t.Fatalf("promoted after reload = %v, want [example.com]", *promoted2)
	}
}

func TestPromoteLearnedDirectRule(t *testing.T) {
	a := &App{Cfg: config.Default(), Paths: config.Paths{ConfigFile: t.TempDir() + "/gateway.yaml"}}

	added, err := a.PromoteLearnedDirectRule("example.com")
	if err != nil || !added {
		t.Fatalf("first promote = %v, %v", added, err)
	}
	rules := a.Cfg.Routing.Rules
	if len(rules) != 1 {
		t.Fatalf("rules = %+v", rules)
	}
	r := rules[0]
	if r.Type != config.RuleDomainSuffix || r.Value != "example.com" || r.Action != config.EgressDirect || !r.Learned {
		t.Fatalf("learned rule = %+v", r)
	}

	// covered host → no duplicate
	added, err = a.PromoteLearnedDirectRule("api.example.com")
	if err != nil || added {
		t.Fatalf("covered promote = %v, %v, want false,nil", added, err)
	}
	if len(a.Cfg.Routing.Rules) != 1 {
		t.Fatalf("rules after covered promote = %+v", a.Cfg.Routing.Rules)
	}

	// persisted with the learned marker
	loaded, err := config.LoadFrom(a.Paths.ConfigFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(loaded.Routing.Rules) != 1 || !loaded.Routing.Rules[0].Learned {
		t.Fatalf("persisted rules = %+v", loaded.Routing.Rules)
	}
}

func TestPromoteLearnedDirectRuleSkipsWhenUserRuleCovers(t *testing.T) {
	cfg := config.Default()
	cfg.Routing.Rules = []config.RoutingRule{
		{Type: config.RuleDomainSuffix, Value: "example.com", Action: config.EgressDirect},
	}
	a := &App{Cfg: cfg, Paths: config.Paths{ConfigFile: t.TempDir() + "/gateway.yaml"}}

	added, err := a.PromoteLearnedDirectRule("www.example.com")
	if err != nil || added {
		t.Fatalf("promote = %v, %v, want false,nil when a user rule already directs", added, err)
	}
	if len(a.Cfg.Routing.Rules) != 1 || a.Cfg.Routing.Rules[0].Learned {
		t.Fatalf("rules = %+v, user rule must stay untouched", a.Cfg.Routing.Rules)
	}
}
