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

type DownloadSource struct {
	URL     string
	Mirror  string
	Dest    string
}

// GeoDataSources returns the download sources for GeoIP/GeoSite data files.
func GeoDataSources(dataDir string) []DownloadSource {
	base := "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest"
	mirror := func(url string) string {
		return strings.Replace(url, "github.com", "mirror.ghproxy.com/https://github.com", 1)
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

// DownloadFile downloads a URL to a local path. Skips if the file already exists.
// Returns true if actually downloaded, false if skipped.
func DownloadFile(url, dest string) (bool, error) {
	if _, err := os.Stat(dest); err == nil {
		return false, nil // already exists
	}

	// Ensure parent directory
	os.MkdirAll(filepath.Dir(dest), 0755)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return false, err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(dest) // cleanup partial download
		return false, err
	}
	return true, nil
}
