package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

func TestSaveRejectsStaleSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.yaml")
	cfg := Default()
	if err := Save(cfg, path); err != nil {
		t.Fatal(err)
	}
	stale, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Update(path, func(next *Config) (bool, error) {
		next.Routing.Rules = append(next.Routing.Rules, RoutingRule{Type: RuleDomain, Value: "learned.example", Action: EgressProxy, Learned: true})
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	stale.HTTPProxy.Port++
	if err := Save(stale, path); !errors.Is(err, ErrConfigChanged) {
		t.Fatalf("stale save: %v", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("stale save changed file")
	}
	_, err = Update(path, func(next *Config) (bool, error) {
		next.Routing.Rules = []RoutingRule{{Type: "invalid", Value: "bad", Action: "bad"}}
		return true, nil
	})
	if err == nil {
		t.Fatal("invalid transaction succeeded")
	}
	after, _ = os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("invalid transaction changed file")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("config permissions changed")
	}
}

// Independent processes exercise the actual sidecar flock, not just Go mutexes.
// Concurrent appends must all survive; conditional replacements have one winner.
func TestUpdateAcrossProcesses(t *testing.T) {
	for _, mode := range []string{"append", "conditional"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "gateway.yaml")
			cfg := Default()
			cfg.Routing.Rules = nil
			if err := Save(cfg, path); err != nil {
				t.Fatal(err)
			}
			var wg sync.WaitGroup
			for i := 0; i < 6; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					cmd := exec.Command(os.Args[0], "-test.run=^TestConfigUpdateWorker$")
					cmd.Env = append(os.Environ(), "GATEWAY_TEST_CONFIG="+path, "GATEWAY_TEST_UPDATE="+mode, fmt.Sprintf("GATEWAY_TEST_WORKER=%d", i))
					if output, err := cmd.CombinedOutput(); err != nil {
						t.Errorf("worker failed: %v %s", err, output)
					}
				}(i)
			}
			wg.Wait()
			saved, err := LoadFrom(path)
			if err != nil {
				t.Fatal(err)
			}
			want := 60
			if mode == "conditional" {
				want = 1
			}
			if len(saved.Routing.Rules) != want {
				t.Fatalf("got %d rules, want %d", len(saved.Routing.Rules), want)
			}
		})
	}
}

func TestConfigUpdateWorker(t *testing.T) {
	path := os.Getenv("GATEWAY_TEST_CONFIG")
	if path == "" {
		t.Skip("subprocess helper")
	}
	for i := 0; i < 10; i++ {
		_, err := Update(path, func(next *Config) (bool, error) {
			if os.Getenv("GATEWAY_TEST_UPDATE") == "conditional" && len(next.Routing.Rules) != 0 {
				return false, ErrConfigChanged
			}
			next.Routing.Rules = append(next.Routing.Rules, RoutingRule{Type: RuleDomain, Value: fmt.Sprintf("worker-%s-%d.example", os.Getenv("GATEWAY_TEST_WORKER"), i), Action: EgressProxy})
			return true, nil
		})
		if err != nil && !errors.Is(err, ErrConfigChanged) {
			t.Fatal(err)
		}
	}
}
