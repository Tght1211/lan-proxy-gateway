package app

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/tght/lan-proxy-gateway/internal/config"
	"github.com/tght/lan-proxy-gateway/internal/httpproxy"
)

// A nil password preserves an existing credential; an explicit empty password
// is validated normally. Status responses never include the password.
type HTTPProxyUpdate struct {
	Enabled  bool    `json:"enabled"`
	Port     int     `json:"port"`
	Auth     string  `json:"auth"`
	Username string  `json:"username"`
	Password *string `json:"password"`
}

func (a *App) SetHTTPProxy(update HTTPProxyUpdate) error {
	next := *a.getCfg()
	next.HTTPProxy.Enabled = update.Enabled
	next.HTTPProxy.Port = update.Port
	next.HTTPProxy.Auth = update.Auth
	next.HTTPProxy.Username = update.Username
	if update.Password != nil {
		next.HTTPProxy.Password = *update.Password
	}
	config.Normalize(&next)
	if err := config.Validate(&next); err != nil {
		return err
	}
	if err := config.Save(&next, a.Paths.ConfigFile); err != nil {
		return err
	}
	a.setCfg(&next)
	if a.Running() {
		if err := apiClient(next.Runtime.APIPort, a.Paths.ConfigFile).Reload(context.Background()); err != nil {
			return fmt.Errorf("配置已保存，但核心尚未应用，请检查端口和日志: %w", err)
		}
	}
	return nil
}

func (rt *daemonRuntime) syncHTTPProxy(cfg *config.Config) error {
	rt.httpMu.Lock()
	defer rt.httpMu.Unlock()
	p := cfg.HTTPProxy
	if !p.Enabled {
		if rt.httpProxy != nil {
			_ = rt.httpProxy.Close()
			rt.httpProxy = nil
		}
		return nil
	}
	credentials := httpproxy.Credentials{Username: p.Username, Password: p.Password}
	if p.Auth == "none" {
		credentials = httpproxy.Credentials{}
	}
	if rt.httpProxy != nil && rt.httpProxyPort == p.Port {
		rt.httpProxy.SetCredentials(credentials)
		return nil
	}
	// Bind before closing the old listener, preserving service on port conflicts.
	ln, err := net.Listen("tcp4", net.JoinHostPort("0.0.0.0", strconv.Itoa(p.Port)))
	if err != nil {
		return fmt.Errorf("HTTP 代理端口 %d 不可用: %w", p.Port, err)
	}
	srv := httpproxy.New(func(ctx context.Context, src, target string) (net.Conn, error) {
		relaySrv, _, _ := rt.services()
		return relaySrv.DialExplicit(ctx, src, target)
	}, credentials)
	if rt.httpProxy != nil {
		_ = rt.httpProxy.Close()
	}
	rt.httpProxy = srv
	rt.httpProxyPort = p.Port
	go func() {
		if err := srv.Serve(ln); err != nil {
			rt.eventCh <- serviceEvent{name: "http-proxy", err: err}
		}
	}()
	return nil
}
