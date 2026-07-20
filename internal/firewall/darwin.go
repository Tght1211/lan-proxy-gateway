//go:build darwin

package firewall

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	pfAnchorName = "com.apple/lan-proxy-gateway"
	pfAnchorFile = "/etc/pf.anchors/lan-proxy-gateway"
)

type darwinManager struct {
	// seams for tests
	run    func(args ...string) error
	output func(args ...string) (string, error)
	write  func(path string, data []byte) error
}

func newPlatformManager() Manager {
	return &darwinManager{
		run: func(args ...string) error {
			out, err := exec.Command("pfctl", args...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("pfctl %s: %v: %s", strings.Join(args, " "), err, out)
			}
			return nil
		},
		output: func(args ...string) (string, error) {
			out, err := exec.Command("pfctl", args...).CombinedOutput()
			return string(out), err
		},
		write: func(path string, data []byte) error {
			return os.WriteFile(path, data, 0o600)
		},
	}
}

func (m *darwinManager) Apply(c Config) (Report, error) {
	if err := m.write(pfAnchorFile, []byte(renderPFAnchor(c))); err != nil {
		return Report{}, fmt.Errorf("写入 pf anchor 失败: %w", err)
	}
	if err := m.run("-a", pfAnchorName, "-f", pfAnchorFile); err != nil {
		return Report{}, err
	}
	// Stock macOS pf.conf references the com.apple/* wildcard anchors; a custom
	// pf.conf without them would load our rules but never execute them. Detect
	// that and tell the user exactly what to add instead of editing their file.
	nat, err := m.output("-s", "nat")
	if err != nil || !strings.Contains(nat, "com.apple") {
		return Report{}, fmt.Errorf("pf 主规则集缺少 com.apple/* 通配 anchor，请在 /etc/pf.conf 的对应位置加入：\n" +
			"  nat-anchor \"com.apple/*\"\n  rdr-anchor \"com.apple/*\"\n  anchor \"com.apple/*\"\n然后执行 sudo pfctl -f /etc/pf.conf")
	}
	enabled, err := m.pfEnabled()
	if err != nil {
		return Report{}, err
	}
	var rep Report
	if !enabled {
		if err := m.run("-e"); err != nil {
			return Report{}, err
		}
		rep.WeEnabledPF = true
	}
	return rep, nil
}

func (m *darwinManager) Remove() error {
	// flush our anchor; disabling pf itself is the caller's decision (only
	// when WeEnabledPF was recorded)
	if err := m.run("-a", pfAnchorName, "-F", "all"); err != nil {
		return err
	}
	_ = os.Remove(pfAnchorFile)
	return nil
}

func (m *darwinManager) pfEnabled() (bool, error) {
	out, err := m.output("-s", "info")
	if err != nil {
		return false, fmt.Errorf("查询 pf 状态失败: %v: %s", err, out)
	}
	for line := range strings.Lines(out) {
		if strings.HasPrefix(line, "Status:") {
			return strings.Contains(line, "Enabled"), nil
		}
	}
	return false, fmt.Errorf("无法解析 pf 状态: %s", out)
}

// DisablePF turns pf off — only call when Report.WeEnabledPF was recorded.
func DisablePF() error {
	out, err := exec.Command("pfctl", "-d").CombinedOutput()
	if err != nil {
		return fmt.Errorf("pfctl -d: %v: %s", err, out)
	}
	return nil
}
