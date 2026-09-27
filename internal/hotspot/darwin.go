//go:build darwin

package hotspot

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func Detect() Status {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	read := func(path string, args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, path, args...)
		cmd.Env = append(cmd.Environ(), "LC_ALL=C", "LANG=C")
		out, err := cmd.Output()
		return string(out), err
	}
	failed := func(step string, err error) Status {
		return Status{Supported: true, Stage: "detection_failed", Message: fmt.Sprintf("无法读取%s，请重新检测；这不表示共享没有开启：%v", step, err)}
	}
	route, err := read("/sbin/route", "-n", "get", "default")
	if err != nil {
		return failed("上网接口", err)
	}
	hardware, err := read("/usr/sbin/networksetup", "-listallhardwareports")
	if err != nil {
		return failed("网卡信息", err)
	}
	interfaces, err := read("/sbin/ifconfig", "-a")
	if err != nil {
		return failed("共享网络", err)
	}
	process, _ := read("/usr/bin/pgrep", "-x", "InternetSharing")
	// Extract only the enable flag; never read Wi-Fi names or credentials.
	enabled, _ := read("/usr/bin/plutil", "-extract", "NAT.Enabled", "raw", "-o", "-", "/Library/Preferences/SystemConfiguration/com.apple.nat.plist")
	flag := strings.TrimSpace(enabled)
	state := Readiness{SharingRunning: strings.TrimSpace(process) != "", SharingConfigured: flag == "1" || flag == "true"}
	if device := wifiDevice(hardware); device != "" {
		power, err := read("/usr/sbin/networksetup", "-getairportpower", device)
		if err == nil {
			state.WiFiPowerKnown, state.WiFiPowered = wifiPower(power)
		}
	}
	if ctx.Err() != nil {
		return failed("系统共享状态", ctx.Err())
	}
	return InspectReadiness(route, hardware, interfaces, state)
}
