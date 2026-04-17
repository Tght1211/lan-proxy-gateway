// Package mihomo knows how to download and place the mihomo binary.
// The engine/ package doesn't care where the binary came from; this package
// is only used by `gateway install`.
package mihomo

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Installer knows how to fetch the mihomo binary for the running host.
type Installer struct {
	DestDir string // directory to place the binary, e.g. /usr/local/bin
	Version string // e.g. "v1.18.6"; empty means "latest"
}

// Install downloads the correct archive for GOOS/GOARCH and extracts `mihomo` into DestDir.
// Uses the official github release assets; returns the final binary path.
func (i Installer) Install() (string, error) {
	version := i.Version
	if version == "" {
		v, err := resolveLatest()
		if err != nil {
			return "", fmt.Errorf("resolve latest version: %w", err)
		}
		version = v
	}
	arch := runtime.GOARCH
	goos := runtime.GOOS
	archName, ext, err := assetName(goos, arch, version)
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("https://github.com/MetaCubeX/mihomo/releases/download/%s/%s", version, archName)
	data, err := httpGet(url)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	if err := os.MkdirAll(i.DestDir, 0o755); err != nil {
		return "", err
	}
	outPath := filepath.Join(i.DestDir, binaryName())
	switch ext {
	case ".gz":
		if err := extractGzip(data, outPath); err != nil {
			return "", err
		}
	case ".zip":
		if err := extractZip(data, outPath); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported archive extension: %s", ext)
	}
	if err := os.Chmod(outPath, 0o755); err != nil {
		return "", err
	}
	return outPath, nil
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "mihomo.exe"
	}
	return "mihomo"
}

func resolveLatest() (string, error) {
	// Pinned version; keeps the build hermetic. Bump as needed.
	return "v1.18.6", nil
}

func assetName(goos, arch, version string) (string, string, error) {
	v := version
	switch goos {
	case "darwin":
		switch arch {
		case "amd64":
			return fmt.Sprintf("mihomo-darwin-amd64-%s.gz", v), ".gz", nil
		case "arm64":
			return fmt.Sprintf("mihomo-darwin-arm64-%s.gz", v), ".gz", nil
		}
	case "linux":
		switch arch {
		case "amd64":
			return fmt.Sprintf("mihomo-linux-amd64-%s.gz", v), ".gz", nil
		case "arm64":
			return fmt.Sprintf("mihomo-linux-arm64-%s.gz", v), ".gz", nil
		}
	case "windows":
		switch arch {
		case "amd64":
			return fmt.Sprintf("mihomo-windows-amd64-%s.zip", v), ".zip", nil
		case "arm64":
			return fmt.Sprintf("mihomo-windows-arm64-%s.zip", v), ".zip", nil
		}
	}
	return "", "", fmt.Errorf("unsupported os/arch: %s/%s", goos, arch)
}

func httpGet(url string) ([]byte, error) {
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

func extractGzip(data []byte, outPath string) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gz.Close()
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, gz)
	return err
}

func extractZip(data []byte, outPath string) error {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		// The archive contains exactly one binary, any name ending in .exe or "mihomo".
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		out, err := os.Create(outPath)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			return err
		}
		out.Close()
		return nil
	}
	return fmt.Errorf("zip archive contained no files")
}
