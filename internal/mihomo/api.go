package mihomo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
}

func (c *Client) GetProxyGroup(name string) (*ProxyGroup, error) {
	var pg ProxyGroup
	if err := c.get("/proxies/"+name, &pg); err != nil {
		return nil, err
	}
	return &pg, nil
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

// FormatAPIURL returns the full API URL string.
func FormatAPIURL(ip string, port int) string {
	return fmt.Sprintf("http://%s:%d", ip, port)
}
