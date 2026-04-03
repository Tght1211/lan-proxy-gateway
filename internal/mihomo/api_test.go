package mihomo

import (
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"
)

func TestListProxyGroupsFiltersSelectableEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxies" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{
			"proxies": {
				"Proxy": {"name":"Proxy","type":"select","now":"香港 01","all":["香港 01","香港 02"]},
				"Auto": {"name":"Auto","type":"url-test","now":"香港 02","all":["香港 01","香港 02"]},
				"DIRECT": {"name":"DIRECT","type":"Direct"}
			}
		}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	groups, err := client.ListProxyGroups()
	if err != nil {
		t.Fatalf("ListProxyGroups() error = %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	if !slices.ContainsFunc(groups, func(group ProxyGroup) bool { return group.Name == "Proxy" }) {
		t.Fatal("missing Proxy group")
	}
}

func TestSelectProxySendsExpectedRequest(t *testing.T) {
	var gotMethod, gotPath, gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody = string(body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	if err := client.SelectProxy("Proxy", "香港 03"); err != nil {
		t.Fatalf("SelectProxy() error = %v", err)
	}

	if gotMethod != http.MethodPut {
		t.Fatalf("method = %s, want PUT", gotMethod)
	}
	if gotPath != "/proxies/Proxy" {
		t.Fatalf("path = %s, want /proxies/Proxy", gotPath)
	}
	if gotBody != `{"name":"香港 03"}` {
		t.Fatalf("body = %s, want expected json", gotBody)
	}
}

func TestGetProxyDelaySendsExpectedRequest(t *testing.T) {
	var gotMethod, gotPath, gotQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"delay":184}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	delay, err := client.GetProxyDelay("香港 02", "http://www.gstatic.com/generate_204", 5*time.Second)
	if err != nil {
		t.Fatalf("GetProxyDelay() error = %v", err)
	}

	if delay != 184 {
		t.Fatalf("delay = %d, want 184", delay)
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("method = %s, want GET", gotMethod)
	}
	if gotPath != "/proxies/%E9%A6%99%E6%B8%AF%2002/delay" && gotPath != "/proxies/香港 02/delay" {
		t.Fatalf("path = %s, want encoded delay path", gotPath)
	}
	if gotQuery != "timeout=5000&url=http%3A%2F%2Fwww.gstatic.com%2Fgenerate_204" && gotQuery != "url=http%3A%2F%2Fwww.gstatic.com%2Fgenerate_204&timeout=5000" {
		t.Fatalf("query = %s, want timeout and url", gotQuery)
	}
}
