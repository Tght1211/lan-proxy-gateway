// Package mihomo knows how to download and place the mihomo binary.
// The engine/ package doesn't care where the binary came from; this package
// is only used by `gateway install`.
package mihomo

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Installer knows how to fetch the mihomo binary for the running host.
type Installer struct {
	DestDir string                           // directory to place the binary, e.g. /usr/local/bin
	Version string                           // e.g. "v1.19.24"; empty means "latest"
	Logf    func(format string, args ...any) // optional progress log; nil = silent
	BaseURL string                           // release URL prefix; empty = official github. Used by tests.
}

// defaultMirrors is tried in order after the direct URL. Each entry is a
// prefix that gets stitched onto the full https://github.com/... URL.
// Ordered roughly by reliability as observed from mainland China.
var defaultMirrors = []string{
	"https://ghfast.top/",
	"https://hub.gitmirror.com/",
	"https://github.moeyy.xyz/",
	"https://ghp.ci/",
}

// Install downloads the correct archive for GOOS/GOARCH and extracts `mihomo` into DestDir.
// It tries the direct github URL first, then falls back through defaultMirrors; the
// GITHUB_MIRROR env var (single URL prefix) overrides the mirror list when set.
// Returns the final binary path.
func (i Installer) Install() (string, error) {
	logf := i.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}

	version := i.Version
	if version == "" {
		v, err := resolveLatest()
		if err != nil {
			return "", fmt.Errorf("resolve latest version: %w", err)
		}
		version = v
	}
	arch := runtime.GOARCH
	goos := runtime.GOOS
	archName, ext, err := assetName(goos, arch, version)
	if err != nil {
		return "", err
	}
	base := i.BaseURL
	if base == "" {
		base = "https://github.com/MetaCubeX/mihomo/releases/download"
	}
	directURL := fmt.Sprintf("%s/%s/%s", strings.TrimRight(base, "/"), version, archName)

	client := newMihomoClient()
	candidates := mirrorCandidates(directURL)

	var data []byte
	var lastErr error
	for idx, candidate := range candidates {
		label := "直连 github"
		if idx > 0 {
			label = "镜像 " + shortHost(candidate)
		}
		logf("↓ 下载 mihomo %s (%s)", version, label)
		body, err := httpGet(client, candidate)
		if err != nil {
			lastErr = err
			logf("  × %s 失败: %v", label, err)
			continue
		}
		data = body
		lastErr = nil
		break
	}
	if lastErr != nil {
		return "", fmt.Errorf("download mihomo: 所有下载源均失败 (last: %w)", lastErr)
	}

	if err := os.MkdirAll(i.DestDir, 0o755); err != nil {
		return "", err
	}
	outPath := filepath.Join(i.DestDir, binaryName())
	switch ext {
	case ".gz":
		if err := extractGzip(data, outPath); err != nil {
			return "", err
		}
	case ".zip":
		if err := extractZip(data, outPath); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported archive extension: %s", ext)
	}
	if err := os.Chmod(outPath, 0o755); err != nil {
		return "", err
	}
	return outPath, nil
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "mihomo.exe"
	}
	return "mihomo"
}

// PinnedMihomoVersion is the mihomo release this gateway version is verified
// against. Exposed so cmd/install can compare with what's installed on disk
// and warn / re-download when out of date (issue #4: user's subscription used
// type: anytls which mihomo only learned in v1.19.3).
//
// Bump checklist when changing this:
//  1. Verify mihomo release notes for breaking config schema changes
//  2. Update CHANGELOG with the bump rationale
//  3. Check that key adapters in user subscriptions still parse (vmess/vless/
//     hysteria2/anytls/tuic)
//  4. Pin only to non-prerelease tags
const PinnedMihomoVersion = "v1.19.24"

func resolveLatest() (string, error) {
	// Pinned version; keeps the build hermetic. Bump as needed (see comment
	// above PinnedMihomoVersion for the bump checklist).
	return PinnedMihomoVersion, nil
}

func assetName(goos, arch, version string) (string, string, error) {
	v := version
	switch goos {
	case "darwin":
		switch arch {
		case "amd64":
			return fmt.Sprintf("mihomo-darwin-amd64-%s.gz", v), ".gz", nil
		case "arm64":
			return fmt.Sprintf("mihomo-darwin-arm64-%s.gz", v), ".gz", nil
		}
	case "linux":
		switch arch {
		case "amd64":
			return fmt.Sprintf("mihomo-linux-amd64-%s.gz", v), ".gz", nil
		case "arm64":
			return fmt.Sprintf("mihomo-linux-arm64-%s.gz", v), ".gz", nil
		}
	case "windows":
		switch arch {
		case "amd64":
			return fmt.Sprintf("mihomo-windows-amd64-%s.zip", v), ".zip", nil
		case "arm64":
			return fmt.Sprintf("mihomo-windows-arm64-%s.zip", v), ".zip", nil
		}
	}
	return "", "", fmt.Errorf("unsupported os/arch: %s/%s", goos, arch)
}

// mirrorCandidates returns the ordered URL list to try: direct first, then
// each configured mirror with the direct URL appended. When GITHUB_MIRROR env
// is set, its value replaces the default mirror list (empty env = use defaults).
func mirrorCandidates(directURL string) []string {
	out := []string{directURL}
	for _, m := range activeMirrors() {
		out = append(out, ensureMirrorPrefix(m)+directURL)
	}
	return out
}

func activeMirrors() []string {
	if m := strings.TrimSpace(os.Getenv("GITHUB_MIRROR")); m != "" {
		return []string{m}
	}
	return defaultMirrors
}

func ensureMirrorPrefix(m string) string {
	m = strings.TrimSpace(m)
	if m == "" {
		return ""
	}
	if !strings.HasSuffix(m, "/") {
		m += "/"
	}
	return m
}

// newMihomoClient builds an http.Client that fails fast on unreachable hosts
// (so we can move to the next mirror quickly) but allows enough time for the
// body download on slow links. Honors HTTP_PROXY / HTTPS_PROXY env.
func newMihomoClient() *http.Client {
	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 15 * time.Second}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		IdleConnTimeout:       30 * time.Second,
	}
	return &http.Client{Transport: tr, Timeout: 2 * time.Minute}
}

func httpGet(client *http.Client, rawURL string) ([]byte, error) {
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, rawURL)
	}
	return io.ReadAll(resp.Body)
}

func extractGzip(data []byte, outPath string) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gz.Close()
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, gz)
	return err
}

func extractZip(data []byte, outPath string) error {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		// The archive contains exactly one binary, any name ending in .exe or "mihomo".
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		out, err := os.Create(outPath)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			return err
		}
		out.Close()
		return nil
	}
	return fmt.Errorf("zip archive contained no files")
}
