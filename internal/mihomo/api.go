package mihomo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

func (c *Client) request(method, path string, payload interface{}) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.Secret)
	}
	return c.client.Do(req)
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

type DelayHistory struct {
	Time  string `json:"time"`
	Delay int    `json:"delay"`
}

type ProxyNode struct {
	Name    string         `json:"name"`
	Type    string         `json:"type"`
	Alive   bool           `json:"alive"`
	History []DelayHistory `json:"history"`
}

type ProxyGroup struct {
	Now  string   `json:"now"`
	All  []string `json:"all"`
	Type string   `json:"type"`
	Name string   `json:"name"`
}

func (c *Client) GetProxyGroup(name string) (*ProxyGroup, error) {
	var pg ProxyGroup
	if err := c.get("/proxies/"+url.PathEscape(name), &pg); err != nil {
		return nil, err
	}
	return &pg, nil
}

func (c *Client) GetProxyNode(name string) (*ProxyNode, error) {
	var node ProxyNode
	if err := c.get("/proxies/"+url.PathEscape(name), &node); err != nil {
		return nil, err
	}
	node.Name = name
	return &node, nil
}

func (c *Client) SetProxyGroup(name, target string) error {
	resp, err := c.request("PUT", "/proxies/"+url.PathEscape(name), map[string]string{"name": target})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("set proxy group failed: HTTP %d", resp.StatusCode)
	}
	return nil
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
	resp, err := c.request("PUT", "/providers/proxies/"+url.PathEscape(name), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("update provider failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

// CloseAllConnections closes all active connections to free resources.
func (c *Client) CloseAllConnections() error {
	resp, err := c.request("DELETE", "/connections", nil)
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
