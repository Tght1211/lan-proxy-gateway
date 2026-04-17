package cmd

import (
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveMihomoAssetWindowsAMD64(t *testing.T) {
	spec, err := resolveMihomoAsset("windows", "amd64")
	if err != nil {
		t.Fatalf("resolveMihomoAsset returned error: %v", err)
	}
	if spec.Format != "zip" {
		t.Fatalf("expected zip format, got %q", spec.Format)
	}
	want := "mihomo-windows-amd64-compatible-v1.19.8.zip"
	if spec.AssetName != want {
		t.Fatalf("expected asset %q, got %q", want, spec.AssetName)
	}
}

func TestExtractZipBinary(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "mihomo.zip")
	binPath := filepath.Join(dir, "mihomo.exe")

	out, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}

	zw := zip.NewWriter(out)
	w, err := zw.Create("release/mihomo-windows-amd64-compatible.exe")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := w.Write([]byte("zip-binary")); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close archive file: %v", err)
	}

	if err := extractZipBinary(archivePath, binPath); err != nil {
		t.Fatalf("extractZipBinary returned error: %v", err)
	}

	data, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}
	if string(data) != "zip-binary" {
		t.Fatalf("unexpected extracted content: %q", string(data))
	}
}

func TestIsMihomoWindowsBinary(t *testing.T) {
	cases := map[string]bool{
		"mihomo.exe": true,
		"release/mihomo-windows-amd64-compatible.exe": true,
		"MIHOMO-WINDOWS-AMD64.EXE":                    true,
		"mihomo":                                      false,
		"gateway.exe":                                 false,
		"mihomo-linux-amd64":                          false,
	}

	for name, want := range cases {
		if got := isMihomoWindowsBinary(name); got != want {
			t.Fatalf("isMihomoWindowsBinary(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestExtractGzipBinary(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "mihomo.gz")
	binPath := filepath.Join(dir, "mihomo")

	out, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}

	gw := gzip.NewWriter(out)
	if _, err := gw.Write([]byte("gz-binary")); err != nil {
		t.Fatalf("write gzip payload: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close archive file: %v", err)
	}

	if err := extractGzipBinary(archivePath, binPath); err != nil {
		t.Fatalf("extractGzipBinary returned error: %v", err)
	}

	data, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}
	if string(data) != "gz-binary" {
		t.Fatalf("unexpected extracted content: %q", string(data))
	}
}
