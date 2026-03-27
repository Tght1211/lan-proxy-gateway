package mihomo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetProxyGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxies/Proxy" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name": "Proxy",
			"type": "Selector",
			"now":  "HK-01",
			"all":  []string{"HK-01", "JP-01"},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	group, err := client.GetProxyGroup("Proxy")
	if err != nil {
		t.Fatalf("GetProxyGroup: %v", err)
	}
	if group.Now != "HK-01" {
		t.Fatalf("unexpected current node: %s", group.Now)
	}
}

func TestSetProxyGroup(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/proxies/Proxy" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	if err := client.SetProxyGroup("Proxy", "HK-01"); err != nil {
		t.Fatalf("SetProxyGroup: %v", err)
	}
	if !called {
		t.Fatalf("expected request to be sent")
	}
}
