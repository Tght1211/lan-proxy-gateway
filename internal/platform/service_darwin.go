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

const plistPath = "/Library/LaunchDaemons/com.lan-proxy-gateway.plist"

func (p *impl) InstallService(cfg ServiceConfig) error {
	os.MkdirAll(cfg.LogDir, 0755)

	tmpl, err := template.New("plist").Parse(plistTemplate)
	if err != nil {
		return fmt.Errorf("模板解析失败: %w", err)
	}

	f, err := os.Create(plistPath)
	if err != nil {
		return fmt.Errorf("无法创建 plist 文件: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, cfg); err != nil {
		return fmt.Errorf("模板渲染失败: %w", err)
	}

	os.Chmod(plistPath, 0644)

	if err := exec.Command("launchctl", "bootstrap", "system", plistPath).Run(); err != nil {
		exec.Command("launchctl", "load", "-w", plistPath).Run()
	}

	return nil
}

func (p *impl) UninstallService() error {
	exec.Command("launchctl", "bootout", "system/com.lan-proxy-gateway").Run()
	exec.Command("launchctl", "unload", "-w", plistPath).Run()

	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("无法删除 plist 文件: %w", err)
	}
	return nil
}
