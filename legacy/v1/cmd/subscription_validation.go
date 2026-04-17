package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/proxy"
)

const subscriptionValidationTimeout = 10 * time.Second

type subscriptionStartupStatus struct {
	Source string
	Count  int
	Ready  bool
	Err    error
}

func detectActiveSubscriptionSourceKind(cfg *config.Config) string {
	if cfg == nil {
		return "unknown"
	}
	source := strings.ToLower(strings.TrimSpace(cfg.Proxy.Source))
	switch source {
	case "url", "file":
		return source
	}
	if strings.TrimSpace(cfg.Proxy.SubscriptionURL) != "" {
		return "url"
	}
	if strings.TrimSpace(cfg.Proxy.ConfigFile) != "" {
		return "file"
	}
	profile := activeProxyProfile(cfg)
	source = strings.ToLower(strings.TrimSpace(profile.Source))
	switch source {
	case "url", "file":
		return source
	}
	if strings.TrimSpace(profile.SubscriptionURL) != "" {
		return "url"
	}
	if strings.TrimSpace(profile.ConfigFile) != "" {
		return "file"
	}
	return "unknown"
}

func currentSubscriptionURL(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	if value := strings.TrimSpace(cfg.Proxy.SubscriptionURL); value != "" {
		return value
	}
	return strings.TrimSpace(activeProxyProfile(cfg).SubscriptionURL)
}

func currentSubscriptionFile(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	if value := strings.TrimSpace(cfg.Proxy.ConfigFile); value != "" {
		return value
	}
	return strings.TrimSpace(activeProxyProfile(cfg).ConfigFile)
}

func validateSubscriptionURL(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("订阅链接不能为空")
	}

	tmpFile, err := os.CreateTemp("", "gateway-subscription-url-*.yaml")
	if err != nil {
		return 0, fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer os.Remove(tmpPath)

	if err := downloadGatewayURLToFileWithWindowsFallback(value, tmpPath, subscriptionValidationTimeout); err != nil {
		return 0, fmt.Errorf("无法访问订阅链接: %w", err)
	}

	count, err := extractProxyCountForValidation(tmpPath)
	if err != nil {
		return 0, fmt.Errorf("订阅链接内容无效: %w", err)
	}
	return count, nil
}

func validateSubscriptionFile(path string) (string, int, error) {
	validated, err := validateExistingFile(path)
	if err != nil {
		return "", 0, err
	}
	count, err := extractProxyCountForValidation(validated)
	if err != nil {
		return "", 0, fmt.Errorf("订阅文件校验失败: %w", err)
	}
	return validated, count, nil
}

func validateActiveSubscription(cfg *config.Config) (string, int, error) {
	source := detectActiveSubscriptionSourceKind(cfg)
	status := evaluateSubscriptionStartupStatus(cfg)
	if status.Ready {
		return source, status.Count, nil
	}
	if status.Err != nil {
		return source, 0, status.Err
	}
	return source, 0, fmt.Errorf("当前代理来源未设置，请先配置订阅链接或本地代理文件")
}

func evaluateSubscriptionStartupStatus(cfg *config.Config) subscriptionStartupStatus {
	source := detectActiveSubscriptionSourceKind(cfg)
	status := subscriptionStartupStatus{Source: source}

	switch source {
	case "url":
		count, err := validateSubscriptionURL(currentSubscriptionURL(cfg))
		if err != nil {
			status.Err = err
			return status
		}
		status.Ready = true
		status.Count = count
		return status
	case "file":
		_, count, err := validateSubscriptionFile(currentSubscriptionFile(cfg))
		if err != nil {
			status.Err = err
			return status
		}
		status.Ready = true
		status.Count = count
		return status
	default:
		status.Err = fmt.Errorf("当前代理来源未设置，请先配置订阅链接或本地代理文件")
		return status
	}
}

func extractProxyCountForValidation(inputPath string) (int, error) {
	tmpFile, err := os.CreateTemp("", "gateway-subscription-provider-*.yaml")
	if err != nil {
		return 0, fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer os.Remove(tmpPath)

	count, err := proxy.ExtractProxies(inputPath, tmpPath)
	if err != nil {
		return 0, err
	}
	return count, nil
}
