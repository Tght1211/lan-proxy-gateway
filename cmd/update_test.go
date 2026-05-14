package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestNormalizeRequestedVersion(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "", want: ""},
		{in: "latest", want: ""},
		{in: "LATEST", want: ""},
		{in: "3.4.3", want: "v3.4.3"},
		{in: " v3.3.2 ", want: "v3.3.2"},
		{in: "nightly", want: "nightly"},
	}
	for _, tc := range cases {
		if got := normalizeRequestedVersion(tc.in); got != tc.want {
			t.Fatalf("normalizeRequestedVersion(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestGatewayReleaseAsset(t *testing.T) {
	cases := []struct {
		goos   string
		goarch string
		want   string
	}{
		{goos: "darwin", goarch: "arm64", want: "gateway-darwin-arm64"},
		{goos: "darwin", goarch: "amd64", want: "gateway-darwin-amd64"},
		{goos: "linux", goarch: "arm64", want: "gateway-linux-arm64"},
		{goos: "linux", goarch: "amd64", want: "gateway-linux-amd64"},
		{goos: "windows", goarch: "amd64", want: "gateway-windows-amd64.exe"},
	}
	for _, tc := range cases {
		got, err := gatewayReleaseAsset(tc.goos, tc.goarch)
		if err != nil {
			t.Fatalf("gatewayReleaseAsset(%q, %q): %v", tc.goos, tc.goarch, err)
		}
		if got != tc.want {
			t.Fatalf("gatewayReleaseAsset(%q, %q) = %q, want %q", tc.goos, tc.goarch, got, tc.want)
		}
	}
}

func TestGatewayReleaseAssetRejectsUnsupported(t *testing.T) {
	if _, err := gatewayReleaseAsset("linux", "386"); err == nil {
		t.Fatal("expected unsupported arch error")
	}
	if _, err := gatewayReleaseAsset("freebsd", "amd64"); err == nil {
		t.Fatal("expected unsupported os error")
	}
}

func TestUpdateURLCandidatesUsesOverrideMirror(t *testing.T) {
	const mirror = "https://example.com/proxy"
	old := os.Getenv("GITHUB_MIRROR")
	if err := os.Setenv("GITHUB_MIRROR", mirror); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer func() {
		if old == "" {
			_ = os.Unsetenv("GITHUB_MIRROR")
			return
		}
		_ = os.Setenv("GITHUB_MIRROR", old)
	}()

	const raw = "https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.3/gateway-darwin-arm64"
	got := updateURLCandidates(raw)
	if len(got) != 2 {
		t.Fatalf("len(candidates) = %d, want 2", len(got))
	}
	if got[0] != raw {
		t.Fatalf("first candidate = %q, want direct url", got[0])
	}
	wantMirror := "https://example.com/proxy/" + raw
	if got[1] != wantMirror {
		t.Fatalf("mirror candidate = %q, want %q", got[1], wantMirror)
	}
}

func TestUpdateTempPattern(t *testing.T) {
	if got := updateTempPattern("windows"); got != "gateway-update-*.exe" {
		t.Fatalf("windows pattern = %q", got)
	}
	if got := updateTempPattern("darwin"); got != "gateway-update-*" {
		t.Fatalf("unix pattern = %q", got)
	}
}

func TestBuildWindowsUpdateScript(t *testing.T) {
	script := buildWindowsUpdateScript(
		`C:\Program Files\gateway\gateway.exe`,
		`C:\Temp\gateway-update.exe`,
		true,
	)
	wants := []string{
		`set "TARGET=C:\Program Files\gateway\gateway.exe"`,
		`set "SOURCE=C:\Temp\gateway-update.exe"`,
		`move /Y "%TARGET%" "%BACKUP%"`,
		`"%TARGET%" start >nul 2>&1 <nul`,
	}
	for _, want := range wants {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
}

func TestEscapeWindowsBatchValue(t *testing.T) {
	got := escapeWindowsBatchValue(`C:\Users\%USERNAME%\gateway.exe`)
	want := `C:\Users\%%USERNAME%%\gateway.exe`
	if got != want {
		t.Fatalf("escapeWindowsBatchValue() = %q, want %q", got, want)
	}
}
