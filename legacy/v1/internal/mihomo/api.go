package mihomo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Client queries the mihomo RESTful API.
type Client struct {
	BaseURL string
	Secret  string
	client  *http.Client
}

func NewClient(baseURL, secret string) *Client {
	return &Client{
		BaseURL: baseURL,
		Secret:  secret,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *Client) get(path string, result interface{}) error {
	req, err := http.NewRequest("GET", c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	if c.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.Secret)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, result)
}

// IsAvailable checks if the mihomo API is reachable.
func (c *Client) IsAvailable() bool {
	req, _ := http.NewRequest("GET", c.BaseURL+"/version", nil)
	if c.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.Secret)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

type VersionInfo struct {
	Version string `json:"version"`
}

func (c *Client) GetVersion() (*VersionInfo, error) {
	var v VersionInfo
	if err := c.get("/version", &v); err != nil {
		return nil, err
	}
	return &v, nil
}

type ProxyGroup struct {
	Now     string   `json:"now"`
	All     []string `json:"all"`
	Type    string   `json:"type"`
	Name    string   `json:"name"`
	Hidden  bool     `json:"hidden"`
	TestURL string   `json:"testUrl"`
}

type ProviderProxy struct {
	Alive   bool     `json:"alive"`
	All     []string `json:"all"`
	Name    string   `json:"name"`
	Now     string   `json:"now"`
	Type    string   `json:"type"`
	TestURL string   `json:"testUrl"`
}

type ProxyProvider struct {
	Name        string          `json:"name"`
	Type        string          `json:"type"`
	VehicleType string          `json:"vehicleType"`
	UpdatedAt   string          `json:"updatedAt"`
	Proxies     []ProviderProxy `json:"proxies"`
	TestURL     string          `json:"testUrl"`
}

func (c *Client) GetProxyGroup(name string) (*ProxyGroup, error) {
	var pg ProxyGroup
	if err := c.get("/proxies/"+url.PathEscape(name), &pg); err != nil {
		return nil, err
	}
	if pg.Name == "" {
		pg.Name = name
	}
	return &pg, nil
}

func (c *Client) ListProxyGroups() ([]ProxyGroup, error) {
	var payload struct {
		Proxies map[string]ProxyGroup `json:"proxies"`
	}
	if err := c.get("/proxies", &payload); err != nil {
		return nil, err
	}

	groups := make([]ProxyGroup, 0, len(payload.Proxies))
	for name, group := range payload.Proxies {
		if group.Name == "" {
			group.Name = name
		}
		if len(group.All) == 0 {
			continue
		}
		groups = append(groups, group)
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})
	return groups, nil
}

func (c *Client) GetProxyProvider(name string) (*ProxyProvider, error) {
	var provider ProxyProvider
	if err := c.get("/providers/proxies/"+url.PathEscape(name), &provider); err != nil {
		return nil, err
	}
	if provider.Name == "" {
		provider.Name = name
	}
	return &provider, nil
}

func (c *Client) SelectProxy(groupName, proxyName string) error {
	body, err := json.Marshal(map[string]string{"name": proxyName})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPut, c.BaseURL+"/proxies/"+url.PathEscape(groupName), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.Secret)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(resp.Body)
		if len(msg) > 0 {
			return fmt.Errorf("switch proxy failed: HTTP %d: %s", resp.StatusCode, string(msg))
		}
		return fmt.Errorf("switch proxy failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

type ProxyDelayResult struct {
	Delay int `json:"delay"`
}

func (c *Client) GetProxyDelay(proxyName, testURL string, timeout time.Duration) (int, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if strings.TrimSpace(testURL) == "" {
		testURL = "http://www.gstatic.com/generate_204"
	}

	req, err := http.NewRequest(http.MethodGet, c.BaseURL+"/proxies/"+url.PathEscape(proxyName)+"/delay", nil)
	if err != nil {
		return 0, err
	}
	q := req.URL.Query()
	q.Set("url", testURL)
	q.Set("timeout", fmt.Sprintf("%d", timeout.Milliseconds()))
	req.URL.RawQuery = q.Encode()
	if c.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.Secret)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(resp.Body)
		if len(msg) > 0 {
			return 0, fmt.Errorf("delay test failed: HTTP %d: %s", resp.StatusCode, string(msg))
		}
		return 0, fmt.Errorf("delay test failed: HTTP %d", resp.StatusCode)
	}

	var result ProxyDelayResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	return result.Delay, nil
}

type ConnectionsInfo struct {
	DownloadTotal int64        `json:"downloadTotal"`
	UploadTotal   int64        `json:"uploadTotal"`
	Connections   []Connection `json:"connections"`
}

type Connection struct {
	ID       string                 `json:"id"`
	Metadata map[string]interface{} `json:"metadata"`
}

func (c *Client) GetConnections() (*ConnectionsInfo, error) {
	var info ConnectionsInfo
	if err := c.get("/connections", &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// UpdateProxyProvider triggers mihomo to re-fetch the subscription.
func (c *Client) UpdateProxyProvider(name string) error {
	req, err := http.NewRequest("PUT", c.BaseURL+"/providers/proxies/"+name, nil)
	if err != nil {
		return err
	}
	if c.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.Secret)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("update provider failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

// CloseAllConnections closes all active connections to free resources.
func (c *Client) CloseAllConnections() error {
	req, err := http.NewRequest("DELETE", c.BaseURL+"/connections", nil)
	if err != nil {
		return err
	}
	if c.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.Secret)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// FormatAPIURL returns the full API URL string.
func FormatAPIURL(ip string, port int) string {
	return fmt.Sprintf("http://%s:%d", ip, port)
}
