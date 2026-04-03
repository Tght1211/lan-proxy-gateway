package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const updateCheckTTL = 12 * time.Hour

type updateNotice struct {
	Current   string    `json:"current"`
	Latest    string    `json:"latest"`
	CheckedAt time.Time `json:"checked_at"`
}

func loadUpdateNotice() *updateNotice {
	if version == "" || version == "dev" {
		return nil
	}

	cachePath := updateNoticeCachePath()
	cached := readCachedUpdateNotice(cachePath)
	if cached != nil && cached.Current == version {
		if time.Since(cached.CheckedAt) < updateCheckTTL {
			if isNewerVersion(cached.Latest, version) {
				return cached
			}
			return nil
		}
	}

	latest, err := fetchLatestTagWithTimeout(4 * time.Second)
	if err != nil {
		if cached != nil && cached.Current == version && isNewerVersion(cached.Latest, version) {
			return cached
		}
		return nil
	}

	notice := &updateNotice{
		Current:   version,
		Latest:    latest,
		CheckedAt: time.Now(),
	}
	writeCachedUpdateNotice(cachePath, notice)

	if isNewerVersion(latest, version) {
		return notice
	}
	return nil
}

func renderUpdateNoticeLines(notice *updateNotice) []string {
	if notice == nil {
		return nil
	}

	return []string{
		fmt.Sprintf("发现新版本 %s（当前 %s）", notice.Latest, notice.Current),
		fmt.Sprintf("运行 %s 可一键升级；GitHub 直连失败时会自动尝试镜像", elevatedCmd("update")),
	}
}

func updateNoticeCachePath() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "lan-proxy-gateway", "update-check.json")
}

func readCachedUpdateNotice(path string) *updateNotice {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var notice updateNotice
	if err := json.Unmarshal(data, &notice); err != nil {
		return nil
	}
	return &notice
}

func writeCachedUpdateNotice(path string, notice *updateNotice) {
	if notice == nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}

	data, err := json.Marshal(notice)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0644)
}

func isNewerVersion(latest, current string) bool {
	latest = normalizeSemver(latest)
	current = normalizeSemver(current)
	if latest == "" || current == "" {
		return false
	}
	return semver.Compare(latest, current) > 0
}

func normalizeSemver(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	if !semver.IsValid(v) {
		return ""
	}
	return v
}
