package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/tght/lan-proxy-gateway/internal/app"
)

const (
	updateRepo       = "Tght1211/lan-proxy-gateway"
	updateAPIBase    = "https://api.github.com/repos/" + updateRepo
	updateAPITimeout = 20 * time.Second
)

var updateMirrors = []string{
	"https://hub.gitmirror.com/",
	"https://mirror.ghproxy.com/",
	"https://github.moeyy.xyz/",
	"https://gh.ddlc.top/",
}

var updateCmd = &cobra.Command{
	Use:   "update [version]",
	Short: "升级到最新版本，或升级/回退到指定版本",
	Example: `  sudo gateway update
  sudo gateway update latest
  sudo gateway update v3.4.3
  sudo gateway update 3.3.2`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		maybeElevate()
		target := "latest"
		if len(args) > 0 {
			target = args[0]
		}
		return runUpdate(cmd.Context(), target)
	},
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func runUpdate(ctx context.Context, requested string) error {
	target, err := resolveUpdateTag(ctx, requested)
	if err != nil {
		return err
	}
	current := Version
	color.Cyan("当前版本: %s", current)
	color.Cyan("目标版本: %s", target)
	if current == target {
		color.Green("已是目标版本，无需更新")
		return nil
	}

	asset, err := gatewayReleaseAsset(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", updateRepo, target, asset)

	color.Cyan("下载 %s ...", asset)
	tmpPath, err := downloadUpdateAsset(ctx, url)
	if err != nil {
		return err
	}
	keepTmp := false
	defer func() {
		if !keepTmp {
			_ = os.Remove(tmpPath)
		}
	}()
	if runtime.GOOS != "windows" {
		_ = os.Chmod(tmpPath, 0o755)
	}
	if out, err := exec.Command(tmpPath, "--version").Output(); err == nil {
		if text := strings.TrimSpace(string(out)); text != "" {
			color.Green("下载完成: %s", text)
		}
	} else {
		color.Green("下载完成")
	}

	a, err := app.New()
	if err != nil {
		return err
	}
	wasRunning := a.Status().Running
	localDNSWasLoopback := false
	if wasRunning && a.Plat != nil {
		localDNSWasLoopback, _ = a.Plat.LocalDNSIsLoopback()
	}
	if wasRunning {
		color.Cyan("停止当前 gateway ...")
		if err := a.Stop(); err != nil {
			return err
		}
	}

	self, err := currentExecutablePath()
	if err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		if err := scheduleWindowsSelfUpdate(self, tmpPath, wasRunning); err != nil {
			return err
		}
		keepTmp = true
		color.Green("更新已安排，当前进程退出后会自动替换二进制")
		if wasRunning {
			color.Green("替换完成后会自动重新启动 gateway")
		}
		return nil
	}

	color.Cyan("替换二进制 ...")
	if err := replaceExecutable(tmpPath, self); err != nil {
		return err
	}
	color.Green("已更新到 %s", target)

	if wasRunning {
		color.Cyan("重新启动 gateway ...")
		if err := a.Start(ctx); err != nil {
			return err
		}
		if localDNSWasLoopback && a.Plat != nil {
			if err := a.Plat.SetLocalDNSToLoopback(); err != nil {
				color.Yellow("gateway 已启动，但恢复本机 DNS 到 127.0.0.1 失败: %v", err)
			}
		}
		color.Green("gateway 已重新启动")
	}
	return nil
}

func resolveUpdateTag(ctx context.Context, requested string) (string, error) {
	tag := normalizeRequestedVersion(requested)
	if tag == "" {
		return fetchReleaseTag(ctx, updateAPIBase+"/releases/latest")
	}
	return tag, nil
}

func normalizeRequestedVersion(v string) string {
	v = strings.TrimSpace(v)
	switch strings.ToLower(v) {
	case "", "latest", "last", "laste", "lastest":
		return ""
	}
	if !strings.HasPrefix(v, "v") && len(v) > 0 && v[0] >= '0' && v[0] <= '9' {
		return "v" + v
	}
	return v
}

func fetchReleaseTag(ctx context.Context, url string) (string, error) {
	var failures []string
	client := updateHTTPClient()
	for _, candidate := range updateURLCandidates(url) {
		tag, err := fetchReleaseTagFromCandidate(ctx, client, candidate)
		if err == nil {
			return tag, nil
		}
		failures = append(failures, fmt.Sprintf("%s: %v", candidate, err))
	}
	return "", fmt.Errorf("所有版本源均失败: %s", strings.Join(failures, "; "))
}

func fetchReleaseTagFromCandidate(ctx context.Context, client *http.Client, candidate string) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, updateAPITimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, candidate, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "lan-proxy-gateway")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var release githubRelease
	decodeErr := json.NewDecoder(resp.Body).Decode(&release)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if decodeErr != nil {
		return "", fmt.Errorf("解析版本信息失败: %v", decodeErr)
	}
	if release.TagName == "" {
		return "", fmt.Errorf("版本信息里没有 tag_name")
	}
	return release.TagName, nil
}

func gatewayReleaseAsset(goos, goarch string) (string, error) {
	switch goos {
	case "darwin", "linux":
		if goarch != "amd64" && goarch != "arm64" {
			return "", fmt.Errorf("不支持的架构: %s/%s", goos, goarch)
		}
		return fmt.Sprintf("gateway-%s-%s", goos, goarch), nil
	case "windows":
		if goarch != "amd64" {
			return "", fmt.Errorf("不支持的架构: %s/%s", goos, goarch)
		}
		return "gateway-windows-amd64.exe", nil
	default:
		return "", fmt.Errorf("不支持的系统: %s", goos)
	}
}

func updateURLCandidates(url string) []string {
	candidates := []string{url}
	if mirror := strings.TrimSpace(os.Getenv("GITHUB_MIRROR")); mirror != "" {
		return append(candidates, ensureMirrorPrefix(mirror)+url)
	}
	for _, mirror := range updateMirrors {
		candidates = append(candidates, ensureMirrorPrefix(mirror)+url)
	}
	return candidates
}

func ensureMirrorPrefix(mirror string) string {
	mirror = strings.TrimSpace(mirror)
	if mirror == "" {
		return ""
	}
	if !strings.HasSuffix(mirror, "/") {
		mirror += "/"
	}
	return mirror
}

func openUpdateURL(ctx context.Context, url string) (*http.Response, error) {
	var failures []string
	client := updateHTTPClient()
	for _, candidate := range updateURLCandidates(url) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "lan-proxy-gateway")
		req.Header.Set("Accept", "*/*")
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		if resp != nil {
			failures = append(failures, fmt.Sprintf("%s: HTTP %d", candidate, resp.StatusCode))
			resp.Body.Close()
		} else {
			failures = append(failures, fmt.Sprintf("%s: %v", candidate, err))
		}
	}
	return nil, fmt.Errorf("所有下载源均失败: %s", strings.Join(failures, "; "))
}

func updateHTTPClient() *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.ForceAttemptHTTP2 = false
	return &http.Client{
		Timeout:   10 * time.Minute,
		Transport: tr,
	}
}

func downloadUpdateAsset(ctx context.Context, url string) (string, error) {
	tmp, err := os.CreateTemp("", updateTempPattern(runtime.GOOS))
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	tmp.Close()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	resp, err := openUpdateURL(ctx, url)
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	defer resp.Body.Close()

	out, err := os.Create(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return tmpPath, nil
}

func updateTempPattern(goos string) string {
	if goos == "windows" {
		return "gateway-update-*.exe"
	}
	return "gateway-update-*"
}

func currentExecutablePath() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("无法定位当前可执行文件: %w", err)
	}
	if real, err := filepath.EvalSymlinks(self); err == nil {
		self = real
	}
	return self, nil
}

func replaceExecutable(src, target string) error {
	backup := target + ".bak"
	_ = os.Remove(backup)
	if err := os.Rename(target, backup); err != nil {
		return fmt.Errorf("备份旧版本失败: %w", err)
	}
	if err := copyFile(src, target); err != nil {
		_ = os.Rename(backup, target)
		return fmt.Errorf("替换失败: %w（已尝试回滚）", err)
	}
	if err := os.Chmod(target, 0o755); err != nil {
		_ = os.Rename(backup, target)
		return fmt.Errorf("设置执行权限失败: %w（已尝试回滚）", err)
	}
	_ = os.Remove(backup)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func scheduleWindowsSelfUpdate(target, source string, restart bool) error {
	scriptPath, err := writeWindowsUpdateScript(target, source, restart)
	if err != nil {
		return err
	}
	return exec.Command("cmd", "/C", scriptPath).Start()
}

func writeWindowsUpdateScript(target, source string, restart bool) (string, error) {
	f, err := os.CreateTemp("", "gateway-update-*.cmd")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.WriteString(f, buildWindowsUpdateScript(target, source, restart)); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func buildWindowsUpdateScript(target, source string, restart bool) string {
	restartLine := "rem gateway was not running before update"
	if restart {
		restartLine = `"%TARGET%" start >nul 2>&1 <nul`
	}
	return strings.Join([]string{
		"@echo off",
		"setlocal",
		fmt.Sprintf(`set "TARGET=%s"`, escapeWindowsBatchValue(target)),
		fmt.Sprintf(`set "SOURCE=%s"`, escapeWindowsBatchValue(source)),
		`set "BACKUP=%TARGET%.bak"`,
		`del /f /q "%BACKUP%" >nul 2>&1`,
		`for /L %%I in (1,1,60) do (`,
		`  move /Y "%TARGET%" "%BACKUP%" >nul 2>&1`,
		`  if exist "%BACKUP%" goto replace`,
		`  timeout /t 1 /nobreak >nul`,
		`)`,
		`exit /b 1`,
		`:replace`,
		`copy /Y "%SOURCE%" "%TARGET%" >nul 2>&1`,
		`if errorlevel 1 goto rollback`,
		`del /f /q "%SOURCE%" >nul 2>&1`,
		restartLine,
		`del /f /q "%BACKUP%" >nul 2>&1`,
		`del /f /q "%~f0"`,
		`exit /b 0`,
		`:rollback`,
		`move /Y "%BACKUP%" "%TARGET%" >nul 2>&1`,
		`exit /b 1`,
		"",
	}, "\r\n")
}

func escapeWindowsBatchValue(value string) string {
	return strings.ReplaceAll(value, "%", "%%")
}
