package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/ui"
)

const (
	repo    = "Tght1211/lan-proxy-gateway"
	apiBase = "https://api.github.com/repos/" + repo
)

var mirrors = []string{
	"https://hub.gitmirror.com/",
	"https://mirror.ghproxy.com/",
	"https://github.moeyy.xyz/",
	"https://gh.ddlc.top/",
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "升级到最新版本（自动下载、替换、重启）",
	Run:   runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

type releaseInfo struct {
	TagName string `json:"tag_name"`
}

func runUpdate(cmd *cobra.Command, args []string) {
	checkRoot()

	ui.ShowLogo()
	ui.Step(1, 4, "检查最新版本...")

	latest, err := fetchLatestTag()
	if err != nil {
		ui.Error("获取最新版本失败: %s", err)
		os.Exit(1)
	}

	current := version
	ui.Info("当前版本: %s", current)
	ui.Info("最新版本: %s", latest)

	if current == latest {
		ui.Success("已是最新版本，无需升级")
		return
	}

	ui.Step(2, 4, "下载新版本...")

	asset := releaseAssetName(runtime.GOOS, runtime.GOARCH)
	downloadURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, latest, asset)

	tmpFile, err := downloadWithFallbackRobust(downloadURL)
	if err != nil {
		ui.Error("下载失败: %s", err)
		os.Exit(1)
	}
	keepTmpFile := false
	defer func() {
		if !keepTmpFile {
			os.Remove(tmpFile)
		}
	}()

	os.Chmod(tmpFile, 0755)

	out, _ := exec.Command(tmpFile, "--version").Output()
	newVer := strings.TrimSpace(string(out))
	if newVer != "" {
		ui.Success("下载完成: %s", newVer)
	} else {
		ui.Success("下载完成")
	}

	self, err := os.Executable()
	if err != nil {
		ui.Error("无法获取当前可执行文件路径: %s", err)
		os.Exit(1)
	}
	self, _ = filepath.EvalSymlinks(self)

	if runtime.GOOS == "windows" {
		cfgPath, _ := filepath.Abs(resolveConfigPath())
		dDir, _ := filepath.Abs(resolveDataDir())

		ui.Step(3, 4, "停止当前网关...")
		runStop(cmd, args)

		ui.Step(4, 4, "后台应用更新并重新启动...")
		if err := scheduleWindowsSelfUpdate(self, tmpFile, cfgPath, dDir); err != nil {
			ui.Error("安排 Windows 更新失败: %s", err)
			os.Exit(1)
		}
		keepTmpFile = true
		ui.Success("更新已安排，当前进程退出后会自动替换并重新启动网关")
		return
	}

	ui.Step(3, 4, "替换二进制文件...")

	backupPath := self + ".bak"
	if err := os.Rename(self, backupPath); err != nil {
		ui.Error("备份旧版本失败: %s", err)
		os.Exit(1)
	}

	if err := copyFile(tmpFile, self); err != nil {
		os.Rename(backupPath, self)
		ui.Error("替换失败: %s (已回滚)", err)
		os.Exit(1)
	}
	os.Chmod(self, 0755)
	os.Remove(backupPath)
	ui.Success("二进制文件已更新: %s", self)

	ui.Step(4, 4, "重启网关...")

	runStop(cmd, args)
	runStart(cmd, args)
}

func fetchLatestTag() (string, error) {
	return fetchLatestTagWithTimeout(15 * time.Second)
}

func releaseAssetName(goos, goarch string) string {
	asset := fmt.Sprintf("gateway-%s-%s", goos, goarch)
	if goos == "windows" {
		return asset + ".exe"
	}
	return asset
}

func fetchLatestTagWithTimeout(timeout time.Duration) (string, error) {
	tag, err := fetchLatestTagFromAPI(timeout)
	if err == nil {
		return tag, nil
	}

	redirectTag, redirectErr := fetchLatestTagFromReleasePage(timeout)
	if redirectErr == nil {
		return redirectTag, nil
	}

	return "", fmt.Errorf("GitHub API 失败: %v；release 页面回退失败: %v", err, redirectErr)
}

func fetchLatestTagFromAPI(timeout time.Duration) (string, error) {
	body, err := httpGetWithFallbackTimeout(apiBase+"/releases/latest", timeout)
	if err != nil {
		return "", err
	}
	defer body.Close()

	var info releaseInfo
	if err := json.NewDecoder(body).Decode(&info); err != nil {
		return "", fmt.Errorf("解析版本信息失败: %w", err)
	}
	if info.TagName == "" {
		return "", fmt.Errorf("未找到版本号")
	}
	return info.TagName, nil
}

func fetchLatestTagFromReleasePage(timeout time.Duration) (string, error) {
	client := newGatewayHTTPClient(timeout)
	url := "https://github.com/" + repo + "/releases/latest"

	resp, err := openGatewayURLWithFallbackTimeout(client, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	tag := extractLatestTagFromReleaseURL(resp.Request.URL.String())
	if tag == "" {
		return "", fmt.Errorf("未能从跳转地址识别版本号: %s", resp.Request.URL.String())
	}
	return tag, nil
}

func httpGetWithFallback(url string) (io.ReadCloser, error) {
	return httpGetWithFallbackTimeout(url, 15*time.Second)
}

func httpGetWithFallbackTimeout(url string, timeout time.Duration) (io.ReadCloser, error) {
	client := newGatewayHTTPClient(timeout)
	resp, err := openGatewayURLWithFallbackTimeout(client, url)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func openGatewayURLWithFallbackTimeout(client *http.Client, url string) (*http.Response, error) {
	var failures []string

	for _, candidate := range buildGatewayURLCandidates(url) {
		resp, err := openGatewayURL(client, candidate)
		if err == nil && resp.StatusCode == http.StatusOK {
			return resp, nil
		}

		if resp != nil {
			failures = append(failures, fmt.Sprintf("%s: HTTP %d", gatewayURLCandidateLabel(url, candidate), resp.StatusCode))
			resp.Body.Close()
		} else if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", gatewayURLCandidateLabel(url, candidate), err))
		}
	}

	if len(failures) == 0 {
		return nil, fmt.Errorf("所有下载源均失败")
	}
	return nil, fmt.Errorf("所有下载源均失败: %s", strings.Join(failures, "; "))
}

func downloadWithFallback(url string) (string, error) {
	tmpFile, err := os.CreateTemp("", updateTempPattern(runtime.GOOS))
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	urls := buildGatewayURLCandidates(url)

	for i, u := range urls {
		if i > 0 {
			ui.Warn("直连失败，尝试镜像...")
		}
		if err := downloadGatewayURLToFileWithWindowsFallback(u, tmpPath, 120*time.Second); err != nil {
			continue
		}
		return tmpPath, nil
	}

	os.Remove(tmpPath)
	return "", fmt.Errorf("所有下载源均失败")
}

func downloadWithFallbackRobust(url string) (string, error) {
	tmpFile, err := os.CreateTemp("", updateTempPattern(runtime.GOOS))
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	urls := buildGatewayURLCandidates(url)

	var lastErr error
	for i, u := range urls {
		if i > 0 {
			ui.Warn("Direct download failed, trying mirrors...")
		}
		if err := downloadGatewayURLToFileWithWindowsFallback(u, tmpPath, 120*time.Second); err != nil {
			lastErr = err
			continue
		}
		return tmpPath, nil
	}

	os.Remove(tmpPath)
	if lastErr != nil {
		return "", fmt.Errorf("all download sources failed: %w", lastErr)
	}
	return "", fmt.Errorf("all download sources failed")
}

func updateTempPattern(goos string) string {
	if goos == "windows" {
		return "gateway-update-*.exe"
	}
	return "gateway-update-*"
}

func configuredMirrors() []string {
	if mirror := strings.TrimSpace(os.Getenv("GITHUB_MIRROR")); mirror != "" {
		return []string{ensureMirrorPrefix(mirror)}
	}
	return slices.Clone(mirrors)
}

func buildGatewayURLCandidates(url string) []string {
	candidates := []string{url}
	for _, mirror := range configuredMirrors() {
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

func gatewayURLCandidateLabel(originalURL, candidate string) string {
	if candidate == originalURL {
		return "直连"
	}
	return candidate
}

func extractLatestTagFromReleaseURL(raw string) string {
	const marker = "/releases/tag/"

	idx := strings.Index(raw, marker)
	if idx < 0 {
		return ""
	}

	tag := raw[idx+len(marker):]
	if cut := strings.IndexAny(tag, "/?#"); cut >= 0 {
		tag = tag[:cut]
	}
	return strings.TrimSpace(tag)
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

func scheduleWindowsSelfUpdate(target, source, configPath, dataDir string) error {
	scriptPath, err := writeWindowsUpdateScript(target, source, configPath, dataDir)
	if err != nil {
		return err
	}
	cmd := exec.Command("cmd", "/C", scriptPath)
	if err := cmd.Start(); err != nil {
		return err
	}
	return nil
}

func writeWindowsUpdateScript(target, source, configPath, dataDir string) (string, error) {
	f, err := os.CreateTemp("", "gateway-update-*.cmd")
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.WriteString(f, buildWindowsUpdateScript(target, source, configPath, dataDir)); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func buildWindowsUpdateScript(target, source, configPath, dataDir string) string {
	return strings.Join([]string{
		"@echo off",
		"setlocal",
		fmt.Sprintf(`set "TARGET=%s"`, escapeWindowsBatchValue(target)),
		fmt.Sprintf(`set "SOURCE=%s"`, escapeWindowsBatchValue(source)),
		fmt.Sprintf(`set "CONFIG=%s"`, escapeWindowsBatchValue(configPath)),
		fmt.Sprintf(`set "DATA=%s"`, escapeWindowsBatchValue(dataDir)),
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
		`"%TARGET%" start --config "%CONFIG%" --data-dir "%DATA%" >nul 2>&1 <nul`,
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
