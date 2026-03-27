package mihomo

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var DownloadMirrors = []string{
	"",
	"https://hub.gitmirror.com/",
	"https://mirror.ghproxy.com/",
	"https://github.moeyy.xyz/",
	"https://gh.ddlc.top/",
}

type DownloadSource struct {
	URL    string
	Mirror string
	Dest   string
}

func GeoDataSources(dataDir string) []DownloadSource {
	base := "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest"
	mirror := func(url string) string {
		return strings.Replace(url, "https://github.com", "https://ghfast.top/https://github.com", 1)
	}
	files := []string{"country.mmdb", "geosite.dat", "geoip.dat"}

	var sources []DownloadSource
	for _, f := range files {
		url := base + "/" + f
		sources = append(sources, DownloadSource{
			URL:    url,
			Mirror: mirror(url),
			Dest:   filepath.Join(dataDir, f),
		})
	}
	return sources
}

func DownloadFile(url, dest string) (bool, error) {
	if _, err := os.Stat(dest); err == nil {
		return false, nil
	}

	os.MkdirAll(filepath.Dir(dest), 0o755)

	var lastErr error
	for _, prefix := range DownloadMirrors {
		targetURL := url
		if prefix != "" {
			targetURL = prefix + url
		}
		client := &http.Client{Timeout: 90 * time.Second}
		resp, err := client.Get(targetURL)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("%s returned HTTP %d", targetURL, resp.StatusCode)
			resp.Body.Close()
			continue
		}

		f, err := os.Create(dest)
		if err != nil {
			resp.Body.Close()
			return false, err
		}
		_, err = io.Copy(f, resp.Body)
		resp.Body.Close()
		f.Close()
		if err != nil {
			os.Remove(dest)
			lastErr = err
			continue
		}
		return true, nil
	}

	return false, fmt.Errorf("下载失败: %w", lastErr)
}

func DownloadMihomoBinary(version, arch, dest string) error {
	url := fmt.Sprintf("https://github.com/MetaCubeX/mihomo/releases/download/%s/mihomo-%s", version, arch)
	_, err := DownloadFile(url, dest)
	if err != nil {
		return fmt.Errorf("mihomo 下载失败，请检查网络或设置镜像后重试: %w", err)
	}
	return os.Chmod(dest, 0o755)
}
