package source

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

var proxyHealthURLs = []string{
	"http://www.gstatic.com/generate_204",
	"http://cp.cloudflare.com/generate_204",
	"http://connectivitycheck.gstatic.com/generate_204",
}

// TestOptions tweaks runtime-only test behavior.
type TestOptions struct {
	SubscriptionProxyURL string
	ProxyTCPOnly         bool
}

// Test 探测当前源是否可达。每种 type 做不同的检查：
//   - external / remote: TCP dial server:port
//   - subscription: HTTP GET url（状态码 < 400）
//   - file: 读文件 + 粗看像 Clash YAML（有 proxies / proxy-providers）
//   - none: 直连不用测
//
// 失败返回中文 error；成功返回 nil。超时走 ctx。
func Test(ctx context.Context, src config.SourceConfig) error {
	return TestWithOptions(ctx, src, TestOptions{})
}

// TestWithOptions is the runtime-aware variant used by console/supervisor.
func TestWithOptions(ctx context.Context, src config.SourceConfig, opts TestOptions) error {
	switch src.Type {
	case config.SourceTypeExternal:
		if opts.ProxyTCPOnly {
			return testTCP(ctx, src.External.Server, src.External.Port)
		}
		return testProxy(ctx, src.External.Kind, src.External.Server, src.External.Port, "", "")
	case config.SourceTypeRemote:
		if opts.ProxyTCPOnly {
			return testTCP(ctx, src.Remote.Server, src.Remote.Port)
		}
		return testProxy(ctx, src.Remote.Kind, src.Remote.Server, src.Remote.Port, src.Remote.Username, src.Remote.Password)
	case config.SourceTypeSubscription:
		proxyURL := opts.SubscriptionProxyURL
		if proxyURL == "" {
			proxyURL = firstUpstreamProxyURL(src)
		}
		return testURL(ctx, src.Subscription.URL, proxyURL)
	case config.SourceTypeFile:
		return testFile(src.File.Path)
	case config.SourceTypeNone, "":
		return nil
	}
	return fmt.Errorf("未知源类型: %s", src.Type)
}

func testTCP(ctx context.Context, host string, port int) error {
	if host == "" || port <= 0 {
		return fmt.Errorf("主机或端口未填")
	}
	d := net.Dialer{Timeout: 3 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return fmt.Errorf("连不上 %s:%d → %w", host, port, err)
	}
	_ = conn.Close()
	return nil
}

func testProxy(ctx context.Context, kind, host string, port int, username, password string) error {
	if err := testTCP(ctx, host, port); err != nil {
		return err
	}
	proxyURL := proxyURLForProxy(kind, host, port, username, password)
	if proxyURL == "" {
		return fmt.Errorf("不支持的代理类型: %s", kind)
	}
	u, err := url.Parse(proxyURL)
	if err != nil {
		return err
	}
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(u)},
		Timeout:   8 * time.Second,
	}
	var lastErr error
	for _, target := range proxyHealthURLs {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode < 400 {
			return nil
		}
		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return fmt.Errorf("%s:%d 端口开着，但代理健康检查失败 → %w", host, port, lastErr)
}

func testURL(ctx context.Context, url string, proxyURL string) error {
	if url == "" {
		return fmt.Errorf("订阅 URL 为空")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "clash-meta/1.18")
	client := newSubscriptionClient(proxyURL)
	client.Timeout = 10 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("访问订阅失败 → %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("订阅返回 HTTP %d", resp.StatusCode)
	}
	return nil
}

func testFile(path string) error {
	if path == "" {
		return fmt.Errorf("文件路径为空")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("找不到文件 → %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s 是目录，不是文件", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读不了文件 → %w", err)
	}
	s := string(data)
	if !strings.Contains(s, "proxies") && !strings.Contains(s, "proxy-providers") {
		return fmt.Errorf("文件里没找到 proxies / proxy-providers，看起来不是 Clash/mihomo YAML")
	}
	return nil
}
