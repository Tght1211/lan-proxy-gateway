// Package ipinfo 通过 mihomo 的 HTTP 代理端口查 https://ipinfo.io/json，拿到
// 真实出口 IP + 地理位置 + 运营商。仪表盘用来显示「落地」位置，比单看链式代
// 理的 server 入口或节点名里的 emoji 准确（多跳 / IP 轮换 / 住宅 IP 出口和
// 入口不同地区的场景都能反映真相）。
//
// ipinfo.io 免费版 1000 次/天，所以调用方要自己做缓存（通常 30 秒够用）。
package ipinfo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// Info 是 /json 返回的关键字段。ipinfo 返回还有 hostname/loc/postal/timezone
// 等，仪表盘暂时不需要。
type Info struct {
	IP      string `json:"ip"`
	City    string `json:"city"`
	Region  string `json:"region"`
	Country string `json:"country"` // ISO-3166-1 alpha-2，例如 "US"
	Org     string `json:"org"`     // 形如 "AS7922 Comcast Cable Communications, LLC"
}

// ISP 把 Org 里的 "ASxxxx " 前缀剥掉，返回人读的运营商名。太长时做一次截断。
func (i *Info) ISP() string {
	s := strings.TrimSpace(i.Org)
	if strings.HasPrefix(s, "AS") {
		if sp := strings.IndexByte(s, ' '); sp > 0 {
			s = strings.TrimSpace(s[sp+1:])
		}
	}
	// 截一下过长的，留给仪表盘一行装得下。按 rune 数算，省略号本身算一个。
	if utf8.RuneCountInString(s) > 28 {
		r := []rune(s)
		s = string(r[:27]) + "…"
	}
	return s
}

// Fetch 通过 proxyURL（例如 "http://127.0.0.1:17890"）向 ipinfo.io 发一次请求。
// proxyURL 为空走直连（调用方一般不想这样；家用宽带出口不是代理落地位置）。
// 超时 5 秒，错误时返回 nil, err 让上层决定降级。
func Fetch(ctx context.Context, proxyURL string) (*Info, error) {
	transport := &http.Transport{}
	if proxyURL != "" {
		u, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("bad proxy url %q: %w", proxyURL, err)
		}
		transport.Proxy = http.ProxyURL(u)
	}
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://ipinfo.io/json", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "lan-proxy-gateway/dashboard")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ipinfo: HTTP %d", resp.StatusCode)
	}
	var info Info
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}
