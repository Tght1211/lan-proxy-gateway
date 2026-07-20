//go:build darwin

package systemproxy

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func networkServices() ([]string, error) {
	out, err := exec.Command("networksetup", "-listallnetworkservices").Output()
	if err != nil {
		return nil, fmt.Errorf("读取网络服务: %w", err)
	}
	var services []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "An asterisk") {
			continue
		}
		services = append(services, line)
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("没有可用的网络服务")
	}
	return services, nil
}

func runNetworkSetup(args ...string) error {
	out, err := exec.Command("networksetup", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("networksetup %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func enable(mode Mode, host string, port int) error {
	services, err := networkServices()
	if err != nil {
		return err
	}
	portText := strconv.Itoa(port)
	var firstErr error
	for _, service := range services {
		var commands [][]string
		if mode == ModeSOCKS5 {
			commands = [][]string{
				{"-setsocksfirewallproxy", service, host, portText},
				{"-setsocksfirewallproxystate", service, "on"},
				{"-setwebproxystate", service, "off"},
				{"-setsecurewebproxystate", service, "off"},
			}
		} else {
			commands = [][]string{
				{"-setwebproxy", service, host, portText},
				{"-setsecurewebproxy", service, host, portText},
				{"-setwebproxystate", service, "on"},
				{"-setsecurewebproxystate", service, "on"},
				{"-setsocksfirewallproxystate", service, "off"},
			}
		}
		for _, command := range commands {
			if err := runNetworkSetup(command...); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func disable() error {
	services, err := networkServices()
	if err != nil {
		return err
	}
	var firstErr error
	for _, service := range services {
		for _, flag := range []string{"-setwebproxystate", "-setsecurewebproxystate", "-setsocksfirewallproxystate"} {
			if err := runNetworkSetup(flag, service, "off"); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func get() (Status, error) {
	services, err := networkServices()
	if err != nil {
		return Status{}, err
	}
	status := Status{Services: make([]Service, 0, len(services))}
	for _, name := range services {
		entry := Service{Name: name}
		if proxy, ok := readProxy("-getsocksfirewallproxy", name); ok && proxy.Enabled {
			entry.Mode, entry.Enabled, entry.Host, entry.Port = ModeSOCKS5, true, proxy.Host, proxy.Port
		} else if proxy, ok := readProxy("-getwebproxy", name); ok && proxy.Enabled {
			entry.Mode, entry.Enabled, entry.Host, entry.Port = ModeHTTP, true, proxy.Host, proxy.Port
		}
		status.Services = append(status.Services, entry)
	}
	return status, nil
}

func readProxy(flag, service string) (Service, bool) {
	out, err := exec.Command("networksetup", flag, service).Output()
	if err != nil {
		return Service{}, false
	}
	return parseProxyOutput(string(out)), true
}
