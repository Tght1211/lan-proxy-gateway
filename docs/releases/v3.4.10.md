# LAN Proxy Gateway v3.4.10

**Gateway Web 控制台可访问性修复版**。这一版修复 `gateway start` 后 `http://<IP>:19091/` 打不开的问题，并把终端控制台里的 Web 引导改回项目自己的 Gateway 控制台。

## 修复

- **`gateway start` 后 Gateway Web 控制台会常驻**

  之前 `gateway start` 默认后台模式只把 mihomo 留在后台，Gateway 自带 WebUI 仍跑在短生命周期 CLI 进程里。CLI 打印完启动信息退出后，`19091` 的 HTTP server 也跟着退出，用户看到的就是 `http://<IP>:19091/` 连接失败。

  现在后台启动会额外拉起一个隐藏的 WebUI daemon，写入 `webui.pid`，并把日志写到配置目录下的 `webui.log`。`gateway stop` 和 `gateway restart` 会同步清理这个 WebUI 进程。

- **终端首页不再把用户引到错误的 Web 页面**

  控制台首页之前展示的是 mihomo 的 `http://<IP>:9090/ui/`，这只能切节点和看连接，不是本项目自己的管理页。现在首页优先展示 Gateway Web 控制台：

  ```text
  http://<IP>:19091/#token=...
  ```

  mihomo 的 `/ui/` 仍保留为“完整控制台”补充地址，方便需要直接操作 mihomo 的用户。

- **`gateway webui` 探活修正**

  `gateway webui` 现在会正确忽略 URL fragment 里的 `#token=...`，探测 `/api/ping` 时不会拼出错误路径。

## 内部改进

- 新增隐藏命令 `gateway __webui-serve`，只供主进程拉起 WebUI daemon 使用。
- WebUI daemon 启停按平台拆分：macOS/Linux 使用进程组和 SIGTERM/SIGKILL，Windows 使用隐藏窗口和 `taskkill /T /F`。
- 控制台菜单里的 Web 控制台连通性探测从 mihomo `/ui/` 切换为 Gateway WebUI 根路径。

## 测试

- `go test ./...`
- `GOOS=windows GOARCH=amd64 go test -c ./cmd -o /tmp/lan-proxy-gateway-cmd-windows.test.exe`
- `make build-all VERSION=v3.4.10`

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.10/gateway-darwin-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.10/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.10/gateway-darwin-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.10/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.10/gateway-linux-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.10/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.10/gateway-linux-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.10/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.10/gateway-windows-amd64.exe) / [.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.10/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.10/SHA256SUMS)

## 升级

```bash
gateway update
```

或指定版本：

```bash
gateway update v3.4.10
```

升级后建议执行一次完整重启，让 WebUI daemon 按新逻辑启动：

```bash
sudo gateway stop
sudo gateway start
```
