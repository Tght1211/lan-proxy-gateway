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
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tght/lan-proxy-gateway/internal/config"
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

type releaseAsset struct {
	Name string `json:"name"`
}

type releaseInfo struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

func (r releaseInfo) HasAsset(name string) bool {
	for _, a := range r.Assets {
		if a.Name == name {
			return true
		}
	}
	return false
}

func runUpdate(cmd *cobra.Command, args []string) {
	checkRoot()

	ui.ShowLogo()
	ui.Step(1, 5, "检查最新版本...")

	rel, err := fetchLatestRelease()
	if err != nil {
		ui.Error("获取最新版本失败: %s", err)
		os.Exit(1)
	}

	latest := rel.TagName
	current := version
	ui.Info("当前版本: %s", current)
	ui.Info("最新版本: %s", latest)

	if current == latest {
		ui.Success("已是最新版本，无需升级")
		return
	}

	ui.Step(2, 5, "下载新版本...")

	asset := fmt.Sprintf("gateway-%s-%s", runtime.GOOS, runtime.GOARCH)
	if !rel.HasAsset(asset) {
		ui.Error("最新版本缺少当前平台资产: %s", asset)
		ui.Error("请稍后重试，或手动到 Release 页面确认附件")
		os.Exit(1)
	}
	downloadURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, latest, asset)

	tmpFile, err := downloadWithFallback(downloadURL)
	if err != nil {
		ui.Error("下载失败: %s", err)
		os.Exit(1)
	}
	defer os.Remove(tmpFile)

	os.Chmod(tmpFile, 0755)

	out, _ := exec.Command(tmpFile, "version").Output()
	newVer := strings.TrimSpace(string(out))
	if newVer == "" {
		out, _ = exec.Command(tmpFile, "--version").Output()
		newVer = strings.TrimSpace(string(out))
	}
	if newVer != "" {
		ui.Success("下载完成: %s", newVer)
	} else {
		ui.Success("下载完成")
	}

	ui.Step(3, 5, "替换二进制文件...")

	self, err := os.Executable()
	if err != nil {
		ui.Error("无法获取当前可执行文件路径: %s", err)
		os.Exit(1)
	}
	self, _ = filepath.EvalSymlinks(self)

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

	ui.Step(4, 5, "升级配置兼容性...")
	if err := ensureUpdatedConfig(); err != nil {
		ui.Warn("配置升级跳过: %s", err)
	} else {
		ui.Success("配置兼容升级完成")
	}

	ui.Step(5, 5, "重启网关...")
	runStop(cmd, args)
	runStart(cmd, args)
	ui.Info("可通过 gateway version 确认版本")
	ui.Info("可通过 gateway ui 打开本地个性化配置页面")
}

func fetchLatestRelease() (*releaseInfo, error) {
	url := apiBase + "/releases/latest"
	body, err := httpGetWithFallback(url)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var info releaseInfo
	if err := json.NewDecoder(body).Decode(&info); err != nil {
		return nil, fmt.Errorf("解析版本信息失败: %w", err)
	}
	if info.TagName == "" {
		return nil, fmt.Errorf("未找到版本号")
	}
	return &info, nil
}

func ensureUpdatedConfig() error {
	path := resolveConfigPath()
	if path == ".secret" {
		cfg, err := config.MigrateFromSecret(path)
		if err != nil {
			return err
		}
		if cfg == nil {
			return nil
		}
		return config.Save(cfg, "gateway.yaml")
	}

	cfg, err := config.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return config.Save(cfg, path)
}

func httpGetWithFallback(url string) (io.ReadCloser, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	resp, err := client.Get(url)
	if err == nil && resp.StatusCode == http.StatusOK {
		return resp.Body, nil
	}
	if resp != nil {
		resp.Body.Close()
	}

	for _, m := range mirrors {
		resp, err = client.Get(m + url)
		if err == nil && resp.StatusCode == http.StatusOK {
			return resp.Body, nil
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	return nil, fmt.Errorf("所有下载源均失败")
}

func downloadWithFallback(url string) (string, error) {
	tmpFile, err := os.CreateTemp("", "gateway-update-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()

	client := &http.Client{Timeout: 120 * time.Second}

	urls := []string{url}
	for _, m := range mirrors {
		urls = append(urls, m+url)
	}

	for i, u := range urls {
		if i > 0 {
			ui.Warn("直连失败，尝试镜像...")
		}
		resp, err := client.Get(u)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}

		tmpFile.Seek(0, 0)
		tmpFile.Truncate(0)

		_, err = io.Copy(tmpFile, resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		tmpFile.Close()
		return tmpPath, nil
	}

	tmpFile.Close()
	os.Remove(tmpPath)
	return "", fmt.Errorf("所有下载源均失败")
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
