package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestValidateSubscriptionFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "proxy.yaml")
	content := "proxies:\n  - name: hk-01\n    type: ss\n    server: 1.1.1.1\n    port: 443\n    cipher: aes-128-gcm\n    password: demo\n"
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	validated, count, err := validateSubscriptionFile(configPath)
	if err != nil {
		t.Fatalf("validateSubscriptionFile() error = %v", err)
	}
	if validated != configPath {
		t.Fatalf("validated path = %q, want %q", validated, configPath)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
}

func TestValidateSubscriptionURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("proxies:\n  - name: sg-01\n    type: ss\n    server: 2.2.2.2\n    port: 443\n    cipher: aes-128-gcm\n    password: demo\n"))
	}))
	defer server.Close()

	count, err := validateSubscriptionURL(server.URL)
	if err != nil {
		t.Fatalf("validateSubscriptionURL() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
}

func TestValidateSubscriptionURLRejectsInvalidContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-a-proxy-subscription"))
	}))
	defer server.Close()

	_, err := validateSubscriptionURL(server.URL)
	if err == nil || !strings.Contains(err.Error(), "订阅链接内容无效") {
		t.Fatalf("expected invalid subscription content error, got %v", err)
	}
}

func TestValidateActiveSubscription(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "proxy.yaml")
	content := "proxies:\n  - name: hk-01\n    type: ss\n    server: 1.1.1.1\n    port: 443\n    cipher: aes-128-gcm\n    password: demo\n"
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Proxy.Source = "file"
	cfg.Proxy.SubscriptionURL = ""
	cfg.Proxy.ConfigFile = configPath

	source, count, err := validateActiveSubscription(cfg)
	if err != nil {
		t.Fatalf("validateActiveSubscription() error = %v", err)
	}
	if source != "file" || count != 1 {
		t.Fatalf("validateActiveSubscription() = (%q, %d), want (file, 1)", source, count)
	}
}

func TestEvaluateSubscriptionStartupStatusMissingURLReturnsNotReady(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Proxy.Source = "url"
	cfg.Proxy.SubscriptionURL = ""

	status := evaluateSubscriptionStartupStatus(cfg)
	if status.Ready {
		t.Fatalf("expected status.Ready to be false")
	}
	if status.Source != "url" {
		t.Fatalf("status.Source = %q, want url", status.Source)
	}
	if status.Err == nil || !strings.Contains(status.Err.Error(), "订阅链接不能为空") {
		t.Fatalf("expected missing URL error, got %v", status.Err)
	}
}

func TestEvaluateSubscriptionStartupStatusMissingFileReturnsNotReady(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Proxy.Source = "file"
	cfg.Proxy.ConfigFile = filepath.Join(t.TempDir(), "missing.yaml")

	status := evaluateSubscriptionStartupStatus(cfg)
	if status.Ready {
		t.Fatalf("expected status.Ready to be false")
	}
	if status.Source != "file" {
		t.Fatalf("status.Source = %q, want file", status.Source)
	}
	if status.Err == nil || !strings.Contains(status.Err.Error(), "文件不存在") {
		t.Fatalf("expected missing file error, got %v", status.Err)
	}
}
