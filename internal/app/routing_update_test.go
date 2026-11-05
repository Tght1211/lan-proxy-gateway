package app

import (
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestRoutingSaveAndLearningUseLatestDiskState(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer api.Close()
	path := filepath.Join(t.TempDir(), "gateway.yaml")
	cfg := config.Default()
	cfg.Egress.Mode = config.EgressProxy
	cfg.Runtime.APIPort = api.Listener.Addr().(*net.TCPAddr).Port
	cfg.Routing.Rules = []config.RoutingRule{{Type: config.RuleDomain, Value: "manual.example", Action: config.EgressProxy}}
	if err := config.Save(cfg, path); err != nil {
		t.Fatal(err)
	}
	newApp := func() *App {
		latest, err := config.LoadFrom(path)
		if err != nil {
			t.Fatal(err)
		}
		return &App{Cfg: latest, Paths: config.Paths{ConfigFile: path}}
	}
	// Both objects load before either writes, like app.New versus a live daemon.
	editor, daemon := newApp(), newApp()
	base := slices.Clone(editor.Cfg.Routing.Rules)
	draft := slices.Clone(base)
	draft[0].Action = config.EgressDirect
	if added, err := daemon.PromoteLearnedProxyRule("first.example"); err != nil || !added {
		t.Fatalf("learn: %v %v", added, err)
	}
	if err := editor.SetRoutingRulesIfUnchanged(draft, base); err == nil {
		t.Fatal("stale editor overwrote learning")
	}
	latest := newApp().Cfg
	if len(latest.Routing.Rules) != 2 || latest.Routing.Rules[0].Action != config.EgressProxy {
		t.Fatal("stale editor changed rules")
	}
	base = slices.Clone(latest.Routing.Rules)
	draft = slices.Clone(base)
	draft[0].Action = config.EgressDirect
	// A non-rule update is retained even with an old editor App snapshot.
	_, err := config.Update(path, func(next *config.Config) (bool, error) { next.HTTPProxy.Port = 18001; return true, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := editor.SetRoutingRulesIfUnchanged(draft, base); err != nil {
		t.Fatal(err)
	}
	// The daemon still has the old action and settings in memory.
	if added, err := daemon.PromoteLearnedProxyRule("second.example"); err != nil || !added {
		t.Fatalf("learn after edit: %v %v", added, err)
	}
	final := newApp().Cfg
	if len(final.Routing.Rules) != 3 || final.Routing.Rules[0].Action != config.EgressDirect || final.HTTPProxy.Port != 18001 {
		t.Fatal("learning overwrote a committed edit")
	}
	// Coverage decisions also use disk: don't learn over a new manual rule.
	latest = newApp().Cfg
	latest.Routing.Rules = append(latest.Routing.Rules, config.RoutingRule{Type: config.RuleDomainSuffix, Value: "covered.example", Action: config.RouteReject})
	if err := config.Save(latest, path); err != nil {
		t.Fatal(err)
	}
	if added, err := daemon.PromoteLearnedProxyRule("www.covered.example"); err != nil || added {
		t.Fatalf("ignored fresh coverage: %v %v", added, err)
	}
}
