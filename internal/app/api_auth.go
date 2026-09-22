package app

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

// A separate, per-run secret protects loopback services even when a LAN client
// can reach them through an explicit proxy or a CONNECT tunnel.
func createAPIToken(configFile string) (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	token := hex.EncodeToString(secret)
	dir := filepath.Dir(configFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, ".api-token-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	_, err = f.WriteString(token)
	closeErr := f.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	config.ReclaimToSudoUser(f.Name())
	if err := os.Rename(f.Name(), filepath.Join(dir, "api-token")); err != nil {
		return "", err
	}
	return token, nil
}

func authenticateAPI(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			http.Error(w, "Local API authentication required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (c *APIClient) do(req *http.Request) (*http.Response, error) {
	data, err := os.ReadFile(c.tokenPath)
	if err != nil {
		return nil, fmt.Errorf("读取本机 API 认证令牌: %w", err)
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return nil, fmt.Errorf("本机 API 认证令牌为空")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return c.hc.Do(req)
}
