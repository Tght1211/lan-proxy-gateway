package app

import (
	"encoding/json"
	"log/slog"
	"os"
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
	cfg.Egress.Mode = config.EgressProxy
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
	if len(restored.Ignored()) != 2 {
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

func TestLearnProxyRulePersistsAndPreservesExplicitRules(t *testing.T) {
	a := &App{Cfg: config.Default(), Paths: config.Paths{ConfigFile: t.TempDir() + "/gateway.yaml"}}
	a.Cfg.Egress.Mode = config.EgressProxy
	a.Cfg.Routing.Rules = nil
	added, err := a.PromoteLearnedProxyRule("UNKNOWN.example.")
	if err != nil || !added {
		t.Fatalf("promote: %v %v", added, err)
	}
	loaded, err := config.LoadFrom(a.Paths.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Routing.Rules) != 1 || loaded.Routing.Rules[0].Value != "unknown.example" || loaded.Routing.Rules[0].Action != config.EgressProxy || !loaded.Routing.Rules[0].Learned {
		t.Fatalf("rules: %+v", loaded.Routing.Rules)
	}
	if added, err = a.PromoteLearnedProxyRule("unknown.example"); added || err != nil {
		t.Fatal("duplicate", err)
	}
	a.Cfg.Routing.Rules = append(a.Cfg.Routing.Rules, config.RoutingRule{Type: config.RuleDomainSuffix, Value: "manual.example", Action: config.EgressDirect})
	if added, err = a.PromoteLearnedProxyRule("www.manual.example"); added || err != nil {
		t.Fatal("overrode explicit rule", err)
	}
	if added, err = a.PromoteLearnedProxyRule("1.2.3.4"); added || err != nil {
		t.Fatal("learned IP", err)
	}
}

func TestProxyLearningConfigurationAndUndo(t *testing.T) {
	dir := t.TempDir()
	a := &App{Cfg: config.Default(), Paths: config.Paths{ConfigFile: dir + "/gateway.yaml"}}
	a.Cfg.Egress.Mode = config.EgressProxy
	a.Cfg.Routing.Rules = nil
	l := newFallbackLearner(dir+"/learn.json", slog.Default())
	configure := func(enabled, auto bool, count int) {
		t.Helper()
		data, _ := json.Marshal(LearningSettings{Enabled: enabled, AutoSave: auto, Confirmations: count})
		if err := a.applyLearningAction(l, "configure", string(data)); err != nil {
			t.Fatal(err)
		}
	}
	configure(true, true, 2)
	if added, err := a.learnProxyResponse(l, "auto.example"); added || err != nil {
		t.Fatal("premature learning", err)
	}
	if added, err := a.learnProxyResponse(l, "auto.example"); !added || err != nil {
		t.Fatal("threshold ignored", err)
	}
	if len(l.Snapshot()) != 0 {
		t.Fatal("saved record remains pending")
	}
	if err := a.applyLearningAction(l, "undo", "auto.example"); err != nil {
		t.Fatal(err)
	}
	if added, err := a.learnProxyResponse(l, "auto.example"); added || err != nil {
		t.Fatal("undo immediately relearned", err)
	}
	restored := newFallbackLearner(dir+"/learn.json", slog.Default())
	if restored.Settings().Confirmations != 2 || len(restored.Ignored()) != 1 {
		t.Fatal("settings/exclusions not persisted")
	}
	configure(false, true, 1)
	a.learnProxyResponse(l, "paused.example")
	if len(l.Snapshot()) != 0 || len(a.Cfg.Routing.Rules) != 0 {
		t.Fatal("global pause ignored")
	}
	configure(true, false, 1)
	if added, err := a.learnProxyResponse(l, "manual.example"); added || err != nil {
		t.Fatal("manual mode auto-saved", err)
	}
	if len(l.Snapshot()) != 1 {
		t.Fatal("missing candidate")
	}
	if err := a.applyLearningAction(l, "accept", "manual.example"); err != nil {
		t.Fatal(err)
	}
	if len(a.Cfg.Routing.Rules) != 1 || a.Cfg.Routing.Rules[0].Action != config.EgressProxy {
		t.Fatal("wrong accepted route")
	}
	if err := a.applyLearningAction(l, "configure", `{"enabled":true,"auto_save":true,"confirmations":0}`); err == nil {
		t.Fatal("invalid threshold accepted")
	}
}

func TestLegacyLearningEvidenceIsNotReversed(t *testing.T) {
	path := t.TempDir() + "/learn.json"
	data, _ := json.Marshal(map[string]any{"counts": map[string][]time.Time{"old.example": {time.Now()}}, "ignored": map[string]bool{"paused.example": true}})
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	l := newFallbackLearner(path, slog.Default())
	if len(l.Snapshot()) != 0 || len(l.Ignored()) != 1 {
		t.Fatal("legacy migration mixed evidence or lost exclusions")
	}
}

func TestLearningResponseSettingsPersistAndRejectInvalid(t *testing.T) {
	dir := t.TempDir()
	a := &App{Cfg: config.Default(), Paths: config.Paths{ConfigFile: dir + "/gateway.yaml"}}
	l := newFallbackLearner(dir+"/learn.json", slog.Default())
	input := `{"enabled":true,"confirmations":2,"auto_save":false,"direct_wait_seconds":3,"proxy_wait_seconds":4,"max_direct_wait_seconds":20,"cooldown_seconds":60,"memory_minutes":15}`
	if err := a.applyLearningAction(l, "configure", input); err != nil {
		t.Fatal(err)
	}
	restored := newFallbackLearner(dir+"/learn.json", slog.Default())
	p := restored.ResponsePolicy()
	if p.DirectWait != 3*time.Second || p.ProxyWait != 4*time.Second || p.MaxDirectWait != 20*time.Second || p.Cooldown != time.Minute || p.MemoryTTL != 15*time.Minute {
		t.Fatal(p)
	}
	if err := a.applyLearningAction(l, "configure", `{"enabled":true,"confirmations":1,"direct_wait_seconds":8,"max_direct_wait_seconds":3}`); err == nil {
		t.Fatal("accepted inverted budgets")
	}
	if l.Settings().DirectWaitSeconds != 3 {
		t.Fatal("invalid input mutated settings")
	}
}
