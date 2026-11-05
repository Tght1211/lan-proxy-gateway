package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestRoutingSetExpectedBaseline(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	paths, err := config.ResolvePaths()
	if err != nil {
		t.Fatal(err)
	}
	// A reload must never reach the host's running gateway.
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer api.Close()
	cfg := config.Default()
	cfg.Runtime.APIPort = api.Listener.Addr().(*net.TCPAddr).Port
	cfg.Routing.Rules = []config.RoutingRule{{Type: "domain", Value: "base.example", Action: "proxy"}, {Type: "domain", Value: "learned.example", Action: "proxy", Learned: true, Group: "自动学习"}}
	config.Normalize(cfg)
	if err := config.Save(cfg, paths.ConfigFile); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(paths.ConfigFile)
	oldRules, oldExpected := routingRulesJSON, routingExpectedJSON
	flag := routingSetCmd.Flags().Lookup("expected-rules-json")
	oldChanged := flag.Changed
	t.Cleanup(func() {
		routingRulesJSON, routingExpectedJSON = oldRules, oldExpected
		flag.Changed = oldChanged
		routingSetCmd.SetOut(nil)
	})
	routingSetCmd.SetOut(io.Discard)
	flag.Changed = true
	desired := slices.Clone(cfg.Routing.Rules)
	desired[0].Action = "direct"
	data, _ := json.Marshal(desired)
	routingRulesJSON = string(data)
	routingExpectedJSON = `[]`
	if err := routingSetCmd.RunE(routingSetCmd, nil); err == nil || !strings.Contains(err.Error(), "未覆盖") {
		t.Fatalf("expected stale baseline rejection: %v", err)
	}
	unchanged, _ := os.ReadFile(paths.ConfigFile)
	if !bytes.Equal(original, unchanged) {
		t.Fatal("stale editor overwrote config")
	}
	data, _ = json.Marshal(cfg.Routing.Rules)
	routingExpectedJSON = string(data)
	if err := routingSetCmd.RunE(routingSetCmd, nil); err != nil {
		t.Fatal(err)
	}
	saved, err := config.LoadFrom(paths.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(saved.Routing.Rules, desired) {
		t.Fatal("matching baseline failed to save or lost learned rule")
	}
}
