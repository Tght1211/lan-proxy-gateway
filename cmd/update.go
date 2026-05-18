package cmd

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/tght/lan-proxy-gateway/internal/platform"
)

const (
	updateRepo       = "Tght1211/lan-proxy-gateway"
	updateAPIBase    = "https://api.github.com/repos/" + updateRepo
	updateLatestPage = "https://github.com/" + updateRepo + "/releases/latest"
	updateAPITimeout = 20 * time.Second

	updateUserAgentHeader = "User-Agent"
	updateUserAgentValue  = "lan-proxy-gateway"
	updateErrCandidateFmt = "%s: %v"
)

var updateMirrors = []string{
	"https://hub.gitmirror.com/",
	"https://mirror.ghproxy.com/",
	"https://github.moeyy.xyz/",
	"https://gh.ddlc.top/",
}

// 内部 flag：elevate 后的子进程通过这两个参数复用父进程已下载好的产物，
// 避免 sudo 把代理类环境变量剥掉之后子进程没法重新下载。
var (
	updatePrefetchedAsset string
	updatePrefetchedTag   string
)

var updateCmd = &cobra.Command{
	Use:   "update [version]",
	Short: "升级到最新版本，或升级/回退到指定版本",
	Example: `  gateway update
  gateway update latest
  gateway update v3.4.3
  gateway update 3.3.2`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := "latest"
		if len(args) > 0 {
			target = args[0]
		}
		return runUpdate(cmd.Context(), target)
	},
}

func init() {
	updateCmd.Flags().StringVar(&updatePrefetchedAsset, "prefetched-asset", "", "")
	updateCmd.Flags().StringVar(&updatePrefetchedTag, "prefetched-tag", "", "")
	_ = updateCmd.Flags().MarkHidden("prefetched-asset")
	_ = updateCmd.Flags().MarkHidden("prefetched-tag")
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

// runUpdate 把 update 流程拆成两段：
//  1. 用户权限阶段：版本查询 + 下载产物到 /tmp。这两步要访问 GitHub，必须能用
//     用户的 HTTPS_PROXY/HTTP_PROXY；不能让 sudo 把这些变量先剥掉。
//  2. root 权限阶段：stop / 替换 /usr/local/bin/gateway / restart。
//
// 完成 (1) 后通过 sudo 重新 exec 自己进入 (2)，把下载好的临时文件路径用隐藏
// flag 透传给子进程，避免子进程在 root 身份下无代理可用。
func runUpdate(ctx context.Context, requested string) error {
	// elevate 后的 root 子进程：跳过下载，直接接管安装阶段。
	if updatePrefetchedAsset != "" && updatePrefetchedTag != "" {
		return installUpdateBinary(ctx, updatePrefetchedTag, updatePrefetchedAsset)
	}

	admin, _ := platform.Current().IsAdmin()
	if !admin && runtime.GOOS == "windows" {
		color.Red("此操作需要管理员权限。")
		color.Yellow("请关闭当前窗口，右键 PowerShell → 以管理员身份运行，再执行：")
		fmt.Printf("  gateway update %s\n", requested)
		return errors.New("admin required")
	}
	if admin && runtime.GOOS != "windows" && os.Getenv("SUDO_USER") != "" && proxyEnvMissing() {
		color.Yellow("提示：通过 sudo 启动会清除 HTTPS_PROXY 等代理变量。")
		color.Yellow("如下载失败，请改用：gateway update %s（不要预先 sudo，程序会按需切换 sudo 并保留代理）。", requested)
	}

	tag, tmpPath, err := prepareUpdateBinary(ctx, requested)
	if err != nil {
		return err
	}
	if tmpPath == "" {
		// prepareUpdateBinary 已打印 "已是目标版本"
		return nil
	}

	if !admin {
		color.Cyan("切换到 sudo 进行替换 ...")
		if err := reexecForUpdateInstall(tag, tmpPath); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
		// syscall.Exec 成功时不会返回到这里。
		return nil
	}
	return installUpdateBinary(ctx, tag, tmpPath)
}

// prepareUpdateBinary resolves the requested tag, downloads the matching
// release asset to a temp path under the current user's identity, and
// returns the resolved tag plus the temp path. Returns (tag, "", nil)
// when the current version already matches and no download was needed.
func prepareUpdateBinary(ctx context.Context, requested string) (string, string, error) {
	tag, err := resolveUpdateTag(ctx, requested)
	if err != nil {
		return "", "", err
	}
	current := Version
	color.Cyan("当前版本: %s", current)
	color.Cyan("目标版本: %s", tag)
	if current == tag {
		color.Green("已是目标版本，无需更新")
		return tag, "", nil
	}

	asset, err := gatewayReleaseAsset(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", "", err
	}
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", updateRepo, tag, asset)

	color.Cyan("下载 %s ...", asset)
	tmpPath, err := downloadUpdateAsset(ctx, url)
	if err != nil {
		return "", "", err
	}
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
	return tag, tmpPath, nil
}

// installUpdateBinary 接管 stop / 替换 / restart，要求当前进程已具备 admin。
func installUpdateBinary(ctx context.Context, target, tmpPath string) error {
	keepTmp := false
	defer func() {
		if !keepTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	a, err := app.New()
	if err != nil {
		return err
	}
	wasRunning, localDNSWasLoopback, err := stopGatewayBeforeUpdate(a)
	if err != nil {
		return err
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
		if err := restartGatewayAfterUpdate(ctx, a, localDNSWasLoopback); err != nil {
			return err
		}
	}
	return nil
}

// stopGatewayBeforeUpdate captures whether gateway was running and whether
// its loopback DNS pinning was active, then stops the daemon if needed.
// Returns (wasRunning, localDNSWasLoopback, error) so the caller can
// later decide whether to restart and restore the DNS pin.
func stopGatewayBeforeUpdate(a *app.App) (bool, bool, error) {
	wasRunning := a.Status().Running
	localDNSWasLoopback := false
	if wasRunning && a.Plat != nil {
		localDNSWasLoopback, _ = a.Plat.LocalDNSIsLoopback()
	}
	if wasRunning {
		color.Cyan("停止当前 gateway ...")
		if err := a.Stop(); err != nil {
			return false, false, err
		}
	}
	return wasRunning, localDNSWasLoopback, nil
}

// restartGatewayAfterUpdate brings gateway back up after a successful
// binary swap, restoring the loopback DNS pinning if it was active
// before the stop. Caller is expected to have already replaced the
// binary on disk.
func restartGatewayAfterUpdate(ctx context.Context, a *app.App, localDNSWasLoopback bool) error {
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
	return nil
}

func proxyEnvMissing() bool {
	for _, k := range []string{
		"HTTPS_PROXY", "HTTP_PROXY", "ALL_PROXY",
		"https_proxy", "http_proxy", "all_proxy",
	} {
		if os.Getenv(k) != "" {
			return false
		}
	}
	return true
}

func resolveUpdateTag(ctx context.Context, requested string) (string, error) {
	tag := normalizeRequestedVersion(requested)
	if tag == "" {
		return fetchLatestReleaseTag(ctx)
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
		failures = append(failures, fmt.Sprintf(updateErrCandidateFmt, candidate, err))
	}
	return "", fmt.Errorf("所有版本源均失败: %s", strings.Join(failures, "; "))
}

func fetchLatestReleaseTag(ctx context.Context) (string, error) {
	apiTag, apiErr := fetchReleaseTag(ctx, updateAPIBase+"/releases/latest")
	if apiErr == nil {
		return apiTag, nil
	}
	pageTag, pageErr := fetchLatestReleaseTagFromRedirect(ctx, updateLatestPage)
	if pageErr == nil {
		color.Yellow("GitHub API 暂不可用，已通过 release 页面跳转解析最新版本: %s", pageTag)
		return pageTag, nil
	}
	return "", fmt.Errorf("无法获取最新版本: API 查询失败: %v; release 页面跳转失败: %v", apiErr, pageErr)
}

func fetchLatestReleaseTagFromRedirect(ctx context.Context, url string) (string, error) {
	var failures []string
	client := updateHTTPClient()
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	for _, candidate := range updateURLCandidates(url) {
		tag, err := fetchLatestReleaseTagFromRedirectCandidate(ctx, client, candidate)
		if err == nil {
			return tag, nil
		}
		failures = append(failures, fmt.Sprintf(updateErrCandidateFmt, candidate, err))
	}
	return "", fmt.Errorf("所有页面跳转源均失败: %s", strings.Join(failures, "; "))
}

func fetchLatestReleaseTagFromRedirectCandidate(ctx context.Context, client *http.Client, candidate string) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, updateAPITimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, candidate, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set(updateUserAgentHeader, updateUserAgentValue)
	req.Header.Set("Accept", "text/html,*/*")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		if tag := tagFromReleaseURL(resp.Request.URL.String()); tag != "" {
			return tag, nil
		}
		return "", fmt.Errorf("页面没有跳转到 release tag")
	}
	if resp.StatusCode < http.StatusMultipleChoices || resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	location := resp.Header.Get("Location")
	if location == "" {
		return "", fmt.Errorf("跳转响应没有 Location")
	}
	if tag := tagFromReleaseURL(location); tag != "" {
		return tag, nil
	}
	return "", fmt.Errorf("跳转地址里没有 release tag: %s", location)
}

func tagFromReleaseURL(raw string) string {
	const marker = "/releases/tag/"
	idx := strings.Index(raw, marker)
	if idx < 0 {
		return ""
	}
	tag := raw[idx+len(marker):]
	if cut := strings.IndexAny(tag, "?#/"); cut >= 0 {
		tag = tag[:cut]
	}
	return strings.TrimSpace(tag)
}

func fetchReleaseTagFromCandidate(ctx context.Context, client *http.Client, candidate string) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, updateAPITimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, candidate, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set(updateUserAgentHeader, updateUserAgentValue)
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
		req.Header.Set(updateUserAgentHeader, updateUserAgentValue)
		req.Header.Set("Accept", "*/*")
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		if resp != nil {
			failures = append(failures, fmt.Sprintf("%s: HTTP %d", candidate, resp.StatusCode))
			resp.Body.Close()
		} else {
			failures = append(failures, fmt.Sprintf(updateErrCandidateFmt, candidate, err))
		}
	}
	return nil, fmt.Errorf("所有下载源均失败: %s", strings.Join(failures, "; "))
}

func updateHTTPClient() *http.Client {
	// 直接克隆默认 transport，保留它的 ProxyFromEnvironment 与 HTTP/2 支持。
	// 之前曾通过 tr.ForceAttemptHTTP2 = false 试图禁用 HTTP/2，但 Clone 后的
	// TLSNextProto 是空 map（非 nil），加上 TLS ALPN 仍会协商成 h2，结果是
	// 传输层握手成 h2、应用层却按 h1 读响应，复现为 EOF 或 malformed HTTP
	// response（HTTP/2 SETTINGS 帧）。让 Go 自己处理 HTTP/2 是最简单且正确
	// 的选择。
	tr := http.DefaultTransport.(*http.Transport).Clone()
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
