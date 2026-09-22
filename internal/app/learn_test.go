package app

import (
	"log/slog"
	"testing"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestFallbackSuggestionsPersistWithoutPromotion(t *testing.T) {
	path := t.TempDir() + "/learn.json"
	l := newFallbackLearner(path, slog.Default())
	now := time.Now()
	l.now = func() time.Time { return now }
	for i := 0; i < 5; i++ {
		l.Record("EXAMPLE.com.")
	}
	if got := l.Snapshot(); len(got) != 1 || got[0].Count != 5 {
		t.Fatalf("suggestions: %+v", got)
	}
	restored := newFallbackLearner(path, slog.Default())
	if got := restored.Snapshot(); len(got) != 1 || got[0].Count != 5 {
		t.Fatalf("restored: %+v", got)
	}
	now = now.Add(25 * time.Hour)
	if len(l.Snapshot()) != 0 {
		t.Fatal("expired suggestions retained")
	}
}

func TestLearningActionsPreserveUserRules(t *testing.T) {
	dir := t.TempDir()
	l := newFallbackLearner(dir+"/learn.json", slog.Default())
	cfg := config.Default()
	config.Normalize(cfg)
	a := &App{Cfg: cfg, Paths: config.Paths{ConfigFile: dir + "/gateway.yaml"}}
	for i := 0; i < 3; i++ {
		l.Record("example.com")
	}
	before := len(cfg.Routing.Rules)
	if err := a.applyLearningAction(l, "accept", "example.com"); err != nil {
		t.Fatal(err)
	}
	if len(a.Cfg.Routing.Rules) != before+1 {
		t.Fatal("accept lost user rules")
	}
	if rule := a.Cfg.Routing.Rules[before]; rule.Type != config.RuleDomain || !rule.Learned {
		t.Fatalf("not exact learned rule: %+v", rule)
	}
	if err := a.applyLearningAction(l, "undo", "example.com"); err != nil {
		t.Fatal(err)
	}
	if len(a.Cfg.Routing.Rules) != before {
		t.Fatal("undo lost user rules")
	}
	for i := 0; i < 3; i++ {
		l.Record("youtube.com")
	}
	if err := a.applyLearningAction(l, "accept", "youtube.com"); err == nil {
		t.Fatal("overrode explicit proxy rule")
	}
	if err := a.applyLearningAction(l, "ignore", "youtube.com"); err != nil {
		t.Fatal(err)
	}
	l.Record("youtube.com")
	if len(l.Snapshot()) != 0 {
		t.Fatal("ignored host observed")
	}
	restored := newFallbackLearner(dir+"/learn.json", slog.Default())
	if len(restored.Ignored()) != 1 {
		t.Fatal("ignore not persisted")
	}
	if err := a.applyLearningAction(l, "restore", "youtube.com"); err != nil {
		t.Fatal(err)
	}
	l.Record("youtube.com")
	if len(l.Snapshot()) != 1 {
		t.Fatal("did not restore observation")
	}
}

func TestPromoteLearnedDirectRule(t *testing.T) {
	a := &App{Cfg: config.Default(), Paths: config.Paths{ConfigFile: t.TempDir() + "/gateway.yaml"}}

	a.Cfg.Routing.Rules = nil
	added, err := a.PromoteLearnedDirectRule("example.com")
	if err != nil || !added {
		t.Fatalf("first promote = %v, %v", added, err)
	}
	rules := a.Cfg.Routing.Rules
	if len(rules) != 1 {
		t.Fatalf("rules = %+v", rules)
	}
	r := rules[0]
	if r.Type != config.RuleDomain || r.Value != "example.com" || r.Action != config.EgressDirect || !r.Learned {
		t.Fatalf("learned rule = %+v", r)
	}

	// Same exact host → no duplicate
	added, err = a.PromoteLearnedDirectRule("example.com")
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
