package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client is a thin HTTP wrapper for mihomo's external-controller REST API.
// It is intentionally small; we only call the endpoints we actually need.
type Client struct {
	baseURL string
	secret  string
	http    *http.Client
}

// NewClient creates a client. baseURL is the http://host:port of mihomo's API.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

// WaitReady polls GET / until it returns 200, or the timeout/context elapses.
func (c *Client) WaitReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL, nil)
		if err != nil {
			return err
		}
		if c.secret != "" {
			req.Header.Set("Authorization", "Bearer "+c.secret)
		}
		resp, err := c.http.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return fmt.Errorf("mihomo API at %s did not become ready in %s", c.baseURL, timeout)
}

// ProxyGroup is a minimal projection of mihomo's proxy-group JSON.
type ProxyGroup struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Now     string   `json:"now"`
	All     []string `json:"all"`
}

// ListProxyGroups returns the proxy groups exposed by mihomo.
func (c *Client) ListProxyGroups(ctx context.Context) ([]ProxyGroup, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/proxies", nil)
	if err != nil {
		return nil, err
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var payload struct {
		Proxies map[string]ProxyGroup `json:"proxies"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	out := make([]ProxyGroup, 0, len(payload.Proxies))
	for _, p := range payload.Proxies {
		if p.Type == "Selector" || p.Type == "URLTest" || p.Type == "Fallback" || p.Type == "LoadBalance" {
			out = append(out, p)
		}
	}
	return out, nil
}

// SelectNode picks a node inside a group.
func (c *Client) SelectNode(ctx context.Context, group, node string) error {
	body := strings.NewReader(fmt.Sprintf(`{"name":%q}`, node))
	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		c.baseURL+"/proxies/"+group, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("select node %s/%s: HTTP %d", group, node, resp.StatusCode)
	}
	return nil
}

// ReloadConfig asks mihomo to reload its config from disk.
func (c *Client) ReloadConfig(ctx context.Context, path string) error {
	body := strings.NewReader(fmt.Sprintf(`{"path":%q}`, path))
	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		c.baseURL+"/configs?force=true", body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("reload config: HTTP %d", resp.StatusCode)
	}
	return nil
}
