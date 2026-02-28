package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// MigrateFromSecret reads a legacy .secret file (KEY=VALUE format)
// and returns a Config. Returns nil if the file doesn't exist.
func MigrateFromSecret(secretPath string) (*Config, error) {
	f, err := os.Open(secretPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	kv := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// strip surrounding quotes
		val = strings.Trim(val, `"'`)
		kv[key] = val
	}

	cfg := DefaultConfig()

	if v, ok := kv["PROXY_SOURCE"]; ok {
		cfg.ProxySource = v
	}
	if v, ok := kv["SUBSCRIPTION_URL"]; ok {
		cfg.SubscriptionURL = v
	}
	if v, ok := kv["PROXY_CONFIG_FILE"]; ok {
		cfg.ProxyConfigFile = v
	}
	if v, ok := kv["SUBSCRIPTION_NAME"]; ok && v != "" {
		cfg.SubscriptionName = v
	}
	if v, ok := kv["API_SECRET"]; ok {
		cfg.APISecret = v
	}
	if v, ok := kv["MIXED_PORT"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Ports.Mixed = n
		}
	}
	if v, ok := kv["REDIR_PORT"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Ports.Redir = n
		}
	}
	if v, ok := kv["API_PORT"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Ports.API = n
		}
	}
	if v, ok := kv["DNS_LISTEN_PORT"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Ports.DNS = n
		}
	}

	fmt.Println()
	return cfg, nil
}
