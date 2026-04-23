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

// GroupDelay 让 mihomo 并发测一个 group 里所有节点的延迟。
// testURL 是测试目标（常用 http://www.gstatic.com/generate_204 / http://cp.cloudflare.com）。
// timeoutMs 单节点超时。
// 返回 map: 节点名 → 延迟 ms（0 = 超时或拒绝）。
func (c *Client) GroupDelay(ctx context.Context, group, testURL string, timeoutMs int) (map[string]int, error) {
	q := fmt.Sprintf("?url=%s&timeout=%d",
		strings.ReplaceAll(testURL, " ", "%20"),
		timeoutMs,
	)
	escapedGroup := strings.ReplaceAll(group, " ", "%20")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/group/"+escapedGroup+"/delay"+q, nil)
	if err != nil {
		return nil, err
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}
	// 给整组测速留足时间：timeout 单节点 + 网络抖动 + 序列化
	client := &http.Client{Timeout: time.Duration(timeoutMs+3000) * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("group delay %s: HTTP %d", group, resp.StatusCode)
	}
	var out map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
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

// SetMode switches mihomo's traffic mode live (rule / global / direct) via the
// /configs endpoint. Used by the source supervisor to fall back to direct when
// the upstream source is dead, then restore the user's chosen mode when it
// comes back.
func (c *Client) SetMode(ctx context.Context, mode string) error {
	body := strings.NewReader(fmt.Sprintf(`{"mode":%q}`, mode))
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, c.baseURL+"/configs", body)
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
		return fmt.Errorf("set mode %s: HTTP %d", mode, resp.StatusCode)
	}
	return nil
}

// ConnectionsSnapshot 是 /connections 端点一次拉回来的全量：累计流量、
// 实时连接（含每条的 metadata.sourceIP，用来聚合 LAN 设备）。
// 仪表盘两次采样做差算速率；累计字段本身就是 mihomo 进程启动起的累计。
type ConnectionsSnapshot struct {
	DownloadTotal int64        `json:"downloadTotal"`
	UploadTotal   int64        `json:"uploadTotal"`
	Connections   []Connection `json:"connections"`
}

// Connection 只投影仪表盘会用的字段。mihomo 字段很多，这里按需扩。
type Connection struct {
	ID       string             `json:"id"`
	Upload   int64              `json:"upload"`
	Download int64              `json:"download"`
	Start    string             `json:"start"`    // RFC3339
	Chains   []string           `json:"chains"`   // 代理链（最后一跳通常是最终出口节点名）
	Rule     string             `json:"rule"`
	Metadata ConnectionMetadata `json:"metadata"`
}

// ConnectionMetadata 关键字段：sourceIP 用来聚合 LAN 设备；host 给「连了什么」
// 用；network 区分 TCP / UDP。
type ConnectionMetadata struct {
	Network     string `json:"network"`
	Type        string `json:"type"`
	SourceIP    string `json:"sourceIP"`
	DestinationIP string `json:"destinationIP"`
	SourcePort  string `json:"sourcePort"`
	DestinationPort string `json:"destinationPort"`
	Host        string `json:"host"`
	ProcessPath string `json:"processPath"`
}

// GetConnections 一次 REST 拉回 downloadTotal/uploadTotal + 所有活跃连接。
// 仪表盘靠这个做所有实时数据（速率、累计、连接数、设备分布）。
func (c *Client) GetConnections(ctx context.Context) (*ConnectionsSnapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/connections", nil)
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
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("get connections: HTTP %d", resp.StatusCode)
	}
	var snap ConnectionsSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		return nil, err
	}
	return &snap, nil
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
