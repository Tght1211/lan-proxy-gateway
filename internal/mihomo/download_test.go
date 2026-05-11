package mihomo

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

// 锚住 mihomo 版本 pin：v3.4.2 起 mihomo 必须 ≥ v1.19.3 才能识别订阅里的
// `type: anytls` 节点 (issue #4)。降到 v1.19.x 以下属于回归 —— 这测试就是
// 防回归栏，提醒未来改 PinnedMihomoVersion 的人意识到这条最低线。
func TestPinnedMihomoVersionSatisfiesAnytls(t *testing.T) {
	v := PinnedMihomoVersion
	if !strings.HasPrefix(v, "v") {
		t.Fatalf("pinned version must start with 'v', got %q", v)
	}
	parts := strings.SplitN(strings.TrimPrefix(v, "v"), ".", 3)
	if len(parts) < 2 {
		t.Fatalf("pinned version too short: %q", v)
	}
	major, err1 := atoiTest(parts[0])
	minor, err2 := atoiTest(parts[1])
	if err1 != nil || err2 != nil {
		t.Fatalf("non-numeric major/minor in %q", v)
	}
	// anytls 在 v1.19.3 加入；最低门槛 v1.19。
	switch {
	case major < 1:
		t.Fatalf("pinned mihomo %s < v1.x — way too old, missing modern adapters", v)
	case major == 1 && minor < 19:
		t.Fatalf("pinned mihomo %s < v1.19 — anytls / vless-reality not supported (issue #4 regression)", v)
	}
}

// 小工具，避开测试文件单独 import strconv
func atoiTest(s string) (int, error) {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("non-digit %q", r)
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

func TestMirrorCandidatesDefault(t *testing.T) {
	t.Setenv("GITHUB_MIRROR", "")
	got := mirrorCandidates("https://github.com/foo/bar")

	if len(got) != 1+len(defaultMirrors) {
		t.Fatalf("len = %d, want %d (direct + %d mirrors)", len(got), 1+len(defaultMirrors), len(defaultMirrors))
	}
	if got[0] != "https://github.com/foo/bar" {
		t.Fatalf("first candidate should be the direct URL, got %q", got[0])
	}
	// Every mirror-prefixed candidate must actually contain the original URL.
	for i, c := range got[1:] {
		if !strings.HasSuffix(c, "https://github.com/foo/bar") {
			t.Errorf("mirror[%d] = %q, missing direct URL suffix", i, c)
		}
	}
}

func TestMirrorCandidatesEnvOverride(t *testing.T) {
	t.Setenv("GITHUB_MIRROR", "https://my.mirror.example")
	got := mirrorCandidates("https://github.com/foo/bar")

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (direct + 1 overridden mirror)", len(got))
	}
	if got[1] != "https://my.mirror.example/https://github.com/foo/bar" {
		t.Fatalf("unexpected override candidate: %q", got[1])
	}
}

func TestMirrorCandidatesEnvOverrideAddsTrailingSlash(t *testing.T) {
	// Mirror without trailing slash should still produce a well-formed URL.
	t.Setenv("GITHUB_MIRROR", "https://my.mirror.example")
	got := mirrorCandidates("https://github.com/x")
	if got[1] != "https://my.mirror.example/https://github.com/x" {
		t.Fatalf("candidate missing separating slash: %q", got[1])
	}
}

func TestEnsureMirrorPrefixIdempotent(t *testing.T) {
	cases := map[string]string{
		"":                    "",
		"https://foo":         "https://foo/",
		"https://foo/":        "https://foo/",
		"  https://foo/  ":    "https://foo/",
	}
	for in, want := range cases {
		if got := ensureMirrorPrefix(in); got != want {
			t.Errorf("ensureMirrorPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestInstallFallsBackToNextMirror points the direct URL at a failing server
// and uses GITHUB_MIRROR to route the fallback at a good server. Install
// should try direct (503), fail, try mirror (200), and write the binary.
func TestInstallFallsBackToNextMirror(t *testing.T) {
	archive, err := buildFakeArchive(runtime.GOOS)
	if err != nil {
		t.Skipf("cannot build fake archive for %s: %v", runtime.GOOS, err)
	}

	var directHits, mirrorHits atomic.Int32
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		directHits.Add(1)
		http.Error(w, "boom", http.StatusServiceUnavailable)
	}))
	t.Cleanup(direct.Close)

	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mirrorHits.Add(1)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(archive)
	}))
	t.Cleanup(mirror.Close)

	t.Setenv("GITHUB_MIRROR", mirror.URL+"/")

	dest := t.TempDir()
	var logs []string
	inst := Installer{
		DestDir: dest,
		Version: "v0.0.0-test",
		BaseURL: direct.URL, // direct candidate points at the failing server
		Logf: func(format string, args ...any) {
			logs = append(logs, fmt.Sprintf(format, args...))
		},
	}
	path, err := inst.Install()
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if filepath.Base(path) != binaryName() {
		t.Errorf("binary basename = %q, want %q", filepath.Base(path), binaryName())
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat installed binary: %v", err)
	}
	if info.Size() == 0 {
		t.Error("installed binary is empty")
	}
	if directHits.Load() == 0 {
		t.Error("direct server never hit — direct candidate not attempted")
	}
	if mirrorHits.Load() == 0 {
		t.Error("mirror server never hit — fallback didn't happen")
	}
	if len(logs) < 2 {
		t.Errorf("expected Logf to log both attempts, got %d entries: %v", len(logs), logs)
	}
}

// TestInstallAllMirrorsFailReturnsError ensures we don't silently succeed
// when every candidate is broken.
func TestInstallAllMirrorsFailReturnsError(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusServiceUnavailable)
	}))
	t.Cleanup(dead.Close)

	t.Setenv("GITHUB_MIRROR", dead.URL+"/")

	inst := Installer{
		DestDir: t.TempDir(),
		Version: "v0.0.0-test",
		BaseURL: dead.URL,
	}
	if _, err := inst.Install(); err == nil {
		t.Fatal("expected error when all candidates fail, got nil")
	}
}

// buildFakeArchive produces a minimal gz (unix) or zip (windows) containing
// a tiny "binary" payload so we don't have to fetch a real mihomo release.
func buildFakeArchive(goos string) ([]byte, error) {
	const payload = "#!/bin/sh\necho fake mihomo\n"
	if goos == "windows" {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		w, err := zw.Create("mihomo.exe")
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(payload)); err != nil {
			return nil, err
		}
		if err := zw.Close(); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte(payload)); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
