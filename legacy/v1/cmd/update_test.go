package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestReleaseAssetName(t *testing.T) {
	cases := []struct {
		goos   string
		goarch string
		want   string
	}{
		{goos: "darwin", goarch: "arm64", want: "gateway-darwin-arm64"},
		{goos: "linux", goarch: "amd64", want: "gateway-linux-amd64"},
		{goos: "windows", goarch: "amd64", want: "gateway-windows-amd64.exe"},
	}

	for _, tc := range cases {
		if got := releaseAssetName(tc.goos, tc.goarch); got != tc.want {
			t.Fatalf("releaseAssetName(%q, %q) = %q, want %q", tc.goos, tc.goarch, got, tc.want)
		}
	}
}

func TestUpdateTempPattern(t *testing.T) {
	cases := []struct {
		goos string
		want string
	}{
		{goos: "darwin", want: "gateway-update-*"},
		{goos: "linux", want: "gateway-update-*"},
		{goos: "windows", want: "gateway-update-*.exe"},
	}

	for _, tc := range cases {
		if got := updateTempPattern(tc.goos); got != tc.want {
			t.Fatalf("updateTempPattern(%q) = %q, want %q", tc.goos, got, tc.want)
		}
	}
}

func TestBuildWindowsUpdateScript(t *testing.T) {
	script := buildWindowsUpdateScript(
		`C:\Program Files\gateway\gateway.exe`,
		`C:\Temp\gateway-update.exe`,
		`C:\Users\demo\gateway.yaml`,
		`C:\Users\demo\data`,
	)

	wants := []string{
		`set "TARGET=C:\Program Files\gateway\gateway.exe"`,
		`set "SOURCE=C:\Temp\gateway-update.exe"`,
		`"%TARGET%" start --config "%CONFIG%" --data-dir "%DATA%" >nul 2>&1 <nul`,
		`move /Y "%TARGET%" "%BACKUP%"`,
	}

	for _, want := range wants {
		if !strings.Contains(script, want) {
			t.Fatalf("expected script to contain %q, got:\n%s", want, script)
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

func TestExtractLatestTagFromReleaseURL(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{
			raw:  "https://github.com/Tght1211/lan-proxy-gateway/releases/tag/v2.2.11",
			want: "v2.2.11",
		},
		{
			raw:  "https://github.com/Tght1211/lan-proxy-gateway/releases/tag/v2.2.11?expanded=true",
			want: "v2.2.11",
		},
		{
			raw:  "https://ghproxy.example/https://github.com/Tght1211/lan-proxy-gateway/releases/tag/v2.2.11",
			want: "v2.2.11",
		},
		{
			raw:  "https://github.com/Tght1211/lan-proxy-gateway/releases/latest",
			want: "",
		},
	}

	for _, tc := range cases {
		if got := extractLatestTagFromReleaseURL(tc.raw); got != tc.want {
			t.Fatalf("extractLatestTagFromReleaseURL(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestBuildGatewayURLCandidatesUsesOverrideMirror(t *testing.T) {
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

	candidates := buildGatewayURLCandidates("https://github.com/Tght1211/lan-proxy-gateway/releases/latest")
	if len(candidates) != 2 {
		t.Fatalf("len(candidates) = %d, want 2", len(candidates))
	}
	if candidates[0] != "https://github.com/Tght1211/lan-proxy-gateway/releases/latest" {
		t.Fatalf("unexpected direct candidate: %q", candidates[0])
	}
	if candidates[1] != "https://example.com/proxy/https://github.com/Tght1211/lan-proxy-gateway/releases/latest" {
		t.Fatalf("unexpected mirror candidate: %q", candidates[1])
	}
}
