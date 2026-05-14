package gateway

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// runtimeState 记录上次 Enable() 时我们改了什么，以便 Disable() 只回滚自己改过的部分。
//
// 关键场景 (issue #5)：docker / 用户先把 net.ipv4.ip_forward 设到 1，gateway start
// 看到已经是 1 就不应该把它当成"我们打开的"；gateway stop 也就不应该把它打回 0，
// 不然 docker 暴露给局域网的端口立刻就不通了。
type runtimeState struct {
	NATInterface       string `json:"nat_interface,omitempty"`         // 我们 ConfigureNAT 用的 iface
	WeEnabledIPForward bool   `json:"we_enabled_ip_forward,omitempty"` // 我们是否真的把 ip_forward 从 0 改成 1
}

func readRuntimeState(path string) (runtimeState, error) {
	if path == "" {
		return runtimeState{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return runtimeState{}, nil
		}
		return runtimeState{}, err
	}
	var s runtimeState
	if err := json.Unmarshal(data, &s); err != nil {
		return runtimeState{}, err
	}
	return s, nil
}

func writeRuntimeState(path string, s runtimeState) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func removeRuntimeState(path string) error {
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
