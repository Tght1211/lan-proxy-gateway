package mihomo

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Geodata covers the three files mihomo needs on startup for GEOIP / GEOSITE rules.
// Without these files mihomo blocks on "Can't find MMDB, start download" forever
// if it can't reach GitHub.
var geodataFiles = []struct {
	Name string // final file name in mihomo workdir
	// Mirrors tried in order. Each %s is the raw download path inside the repo.
	// The path is passed to sprintf so every mirror gets it slotted in.
	Mirrors []string
}{
	{
		Name: "geoip.dat",
		Mirrors: []string{
			"https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.dat",
			"https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.dat",
			"https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip.dat",
		},
	},
	{
		Name: "geosite.dat",
		Mirrors: []string{
			"https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geosite.dat",
			"https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geosite.dat",
			"https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geosite.dat",
		},
	},
	{
		Name: "country.mmdb",
		Mirrors: []string{
			"https://cdn.jsdelivr.net/gh/Loyalsoldier/geoip@release/Country.mmdb",
			"https://fastly.jsdelivr.net/gh/Loyalsoldier/geoip@release/Country.mmdb",
			"https://github.com/Loyalsoldier/geoip/releases/latest/download/Country.mmdb",
		},
	},
}

// EnsureGeodata ensures mihomo's workDir has geoip.dat / geosite.dat / country.mmdb.
//
// Three-level lookup so we don't redownload on every install:
//  1. workDir already has a good copy → skip
//  2. cacheDir has a good copy → copy cache → workDir
//  3. neither → download once, persist in BOTH cacheDir and workDir
//
// cacheDir survives `rm -rf ~/.config/lan-proxy-gateway`. Passing "" disables caching.
//
// If `upstreamProxy` is set (e.g. "http://127.0.0.1:6578"), downloads go through
// it — useful in regions where direct GitHub / CDN is unreliable. If the upstream
// proxy is unreachable, we silently fall back to a direct client so the user isn't
// blocked just because their Clash Verge isn't running yet.
func EnsureGeodata(workDir, cacheDir, upstreamProxy string, logf func(format string, args ...any)) error {
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return fmt.Errorf("create mihomo workdir: %w", err)
	}
	if cacheDir != "" {
		_ = os.MkdirAll(cacheDir, 0o755)
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}

	proxyClient := newGeodataClient(upstreamProxy)
	directClient := newGeodataClient("")

	for _, f := range geodataFiles {
		dst := filepath.Join(workDir, f.Name)
		if fileOK(dst) {
			logf("  ✓ %s 已就绪", f.Name)
			continue
		}
		// 缓存命中：复制到 workdir，不走网络。
		if cacheDir != "" {
			cached := filepath.Join(cacheDir, f.Name)
			if fileOK(cached) {
				if err := copyFile(cached, dst); err == nil {
					logf("  ✓ %s 已从缓存复用", f.Name)
					continue
				}
			}
		}
		// 下载：先放缓存（下次免下），再复制到 workdir。
		target := dst
		if cacheDir != "" {
			target = filepath.Join(cacheDir, f.Name)
		}
		var lastErr error
		for _, mirror := range f.Mirrors {
			logf("  ↓ 下载 %s ... (%s)", f.Name, shortHost(mirror))
			// 先试用户指定的上游代理（如果有）
			err := downloadTo(proxyClient, mirror, target)
			if err != nil && upstreamProxy != "" && isProxyUnreachable(err) {
				// 上游代理没开（比如用户 Clash Verge 还没启动）→ 退回直连
				logf("    × 上游代理不可达，改走直连重试")
				err = downloadTo(directClient, mirror, target)
			}
			if err != nil {
				lastErr = err
				logf("    × %s", err)
				continue
			}
			lastErr = nil
			logf("  ✓ %s 下载完成 (已缓存到 %s)", f.Name, dirLabel(cacheDir))
			break
		}
		if lastErr != nil {
			logf("  ! %s 下载失败，跳过（部分规则可能无效）", f.Name)
			continue
		}
		// 如果 target 是 cache，再复制到 workdir。
		if target != dst {
			if err := copyFile(target, dst); err != nil {
				logf("  ! 从缓存复制到 workdir 失败: %v", err)
			}
		}
	}
	return nil
}

// isProxyUnreachable detects the "proxy is not listening" class of errors so we
// can fall back to direct download. Covers:
//   - connection refused on the proxy port
//   - proxyconnect tcp: ...
//   - "Bad Gateway" from a broken proxy
func isProxyUnreachable(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	for _, needle := range []string{"proxyconnect", "connection refused", "no route to host", "bad gateway"} {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

// copyFile does a straight byte copy via temp file + rename for atomicity.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

func dirLabel(p string) string {
	if p == "" {
		return "(无缓存)"
	}
	return p
}

func newGeodataClient(upstreamProxy string) *http.Client {
	tr := &http.Transport{
		ResponseHeaderTimeout: 30 * time.Second,
		IdleConnTimeout:       30 * time.Second,
	}
	if upstreamProxy != "" {
		if u, err := url.Parse(upstreamProxy); err == nil {
			tr.Proxy = http.ProxyURL(u)
		}
	}
	return &http.Client{Transport: tr, Timeout: 2 * time.Minute}
}

func fileOK(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	// Any existing non-empty file is good enough. The user can delete + retry.
	return info.Size() > 1024
}

func downloadTo(client *http.Client, url, dst string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "lan-proxy-gateway/2.0")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	tmp := dst + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

func shortHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Host
}
