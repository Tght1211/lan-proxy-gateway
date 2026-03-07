//go:build darwin

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"text/template"
)

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.lan-proxy-gateway</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.BinaryPath}}</string>
        <string>start</string>
        <string>--config</string>
        <string>{{.ConfigFile}}</string>
        <string>--data-dir</string>
        <string>{{.DataDir}}</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
        <key>Crashed</key>
        <true/>
    </dict>
    <key>StandardOutPath</key>
    <string>{{.LogDir}}/service.log</string>
    <key>StandardErrorPath</key>
    <string>{{.LogDir}}/service-error.log</string>
    <key>WorkingDirectory</key>
    <string>{{.WorkDir}}</string>
    <key>UserName</key>
    <string>root</string>
</dict>
</plist>
`

const healthPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.lan-proxy-gateway.health</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.BinaryPath}}</string>
        <string>health</string>
        <string>--config</string>
        <string>{{.ConfigFile}}</string>
        <string>--data-dir</string>
        <string>{{.DataDir}}</string>
    </array>
    <key>StartCalendarInterval</key>
    <array>
        <dict>
            <key>Hour</key>
            <integer>4</integer>
            <key>Minute</key>
            <integer>0</integer>
        </dict>
        <dict>
            <key>Hour</key>
            <integer>12</integer>
            <key>Minute</key>
            <integer>0</integer>
        </dict>
    </array>
    <key>StandardOutPath</key>
    <string>{{.LogDir}}/health.log</string>
    <key>StandardErrorPath</key>
    <string>{{.LogDir}}/health.log</string>
    <key>WorkingDirectory</key>
    <string>{{.WorkDir}}</string>
    <key>UserName</key>
    <string>root</string>
</dict>
</plist>
`

const plistPath = "/Library/LaunchDaemons/com.lan-proxy-gateway.plist"
const healthPlistPath = "/Library/LaunchDaemons/com.lan-proxy-gateway.health.plist"

func (p *impl) InstallService(cfg ServiceConfig) error {
	os.MkdirAll(cfg.LogDir, 0755)

	// Main service plist
	if err := writePlist(plistTemplate, plistPath, cfg); err != nil {
		return err
	}
	if err := exec.Command("launchctl", "bootstrap", "system", plistPath).Run(); err != nil {
		exec.Command("launchctl", "load", "-w", plistPath).Run()
	}

	// Health check timer plist
	if err := writePlist(healthPlistTemplate, healthPlistPath, cfg); err != nil {
		return fmt.Errorf("安装健康检查服务失败: %w", err)
	}
	if err := exec.Command("launchctl", "bootstrap", "system", healthPlistPath).Run(); err != nil {
		exec.Command("launchctl", "load", "-w", healthPlistPath).Run()
	}

	return nil
}

func writePlist(tmplStr, path string, cfg ServiceConfig) error {
	tmpl, err := template.New("plist").Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("模板解析失败: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("无法创建 plist 文件 %s: %w", path, err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, cfg); err != nil {
		return fmt.Errorf("模板渲染失败: %w", err)
	}
	os.Chmod(path, 0644)
	return nil
}

func (p *impl) UninstallService() error {
	exec.Command("launchctl", "bootout", "system/com.lan-proxy-gateway.health").Run()
	exec.Command("launchctl", "unload", "-w", healthPlistPath).Run()
	os.Remove(healthPlistPath)

	exec.Command("launchctl", "bootout", "system/com.lan-proxy-gateway").Run()
	exec.Command("launchctl", "unload", "-w", plistPath).Run()

	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("无法删除 plist 文件: %w", err)
	}
	return nil
}
