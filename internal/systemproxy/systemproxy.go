package systemproxy

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrNotSupported = errors.New("当前系统不支持自动配置系统代理")

type Mode string

const (
	ModeHTTP   Mode = "http"
	ModeSOCKS5 Mode = "socks5"
)

type Service struct {
	Name    string `json:"name"`
	Mode    Mode   `json:"mode,omitempty"`
	Enabled bool   `json:"enabled"`
	Host    string `json:"host,omitempty"`
	Port    int    `json:"port,omitempty"`
}

type Status struct {
	Services []Service `json:"services"`
}

func (s Status) Summary() string {
	for _, service := range s.Services {
		if service.Enabled {
			return fmt.Sprintf("%s %s:%d", service.Mode, service.Host, service.Port)
		}
	}
	return "关闭"
}

func Enable(mode Mode, host string, port int) error {
	host = strings.TrimSpace(host)
	if mode != ModeHTTP && mode != ModeSOCKS5 {
		return fmt.Errorf("代理类型必须是 http 或 socks5")
	}
	if host == "" || port < 1 || port > 65535 {
		return fmt.Errorf("代理地址或端口无效")
	}
	return enable(mode, host, port)
}

func Disable() error { return disable() }

func Get() (Status, error) { return get() }

func parseProxyOutput(output string) Service {
	var proxy Service
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "Enabled":
			proxy.Enabled = strings.EqualFold(strings.TrimSpace(value), "Yes")
		case "Server":
			proxy.Host = strings.TrimSpace(value)
		case "Port":
			proxy.Port, _ = strconv.Atoi(strings.TrimSpace(value))
		}
	}
	return proxy
}
