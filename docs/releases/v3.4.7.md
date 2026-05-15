# LAN Proxy Gateway v3.4.7

**本机单点代理网关模式修复版**。这一版纠正 v3.4.6 的过度保护：本机已有代理端口（如 `127.0.0.1:6578`）仍然可以同时用于 `17890` 局域网代理和“改网关 + DNS”的透明网关模式。

## 修复

- **恢复本机单点代理的透明网关能力**

  v3.4.6 为了避免自代理回环，把本机单点代理模式运行时的透明网关、TUN、DNS 都关掉了。这会导致：

  - 手机 / 电脑手动填 `gateway IP:17890` 可用
  - Switch / PS5 / Apple TV / 手机改网关 + DNS 时无网络

  v3.4.7 改为：

  - 保留 `mixed-port`，继续支持 `gateway IP:17890`
  - 保留透明网关，继续支持设备改网关 + DNS
  - `gateway.enabled=true` 时运行时强制打开 TUN / DNS
  - 本机单点上游强制 `strict-route: false`，避免第三方代理客户端自己的出站被 gateway 抓回去

  也就是保留原本可用的完整能力，同时避免自代理回环。

- **本机单点代理 supervisor 不再误切直连**

  后台健康检查不再对本机单点代理执行固定 `generate_204` HTTP 探测，也不会因为探测超时自动把 mihomo 切到 `direct`。这类拓扑下 supervisor 只确认 `127.0.0.1:6578` 端口还开着；手动“代理源测试”仍保留真实代理请求，方便排查上游质量。

- **手动健康检查更稳**

  `external` / `remote` 手动测试现在会尝试多个轻量探测 URL，任意一个成功即视为代理可用；端口开着但完全不是代理的情况仍会失败。

- **Fast.com 测速链补齐**

  内置 global 规则补上：

  - `fast.com`
  - `api.fast.com`
  - `api-global.netflix.com`
  - `ichnaea.netflix.com`
  - `ichnaea-web.netflix.com`
  - `nflxvideo.net`

  避免页面能打开但测速 API / Netflix CDN 测速链路没有完整走代理。

- **补上 `gateway restart` 命令**

  文档一直写着 `sudo gateway restart`，但 CLI 没注册这个子命令。现在它等同于 `stop + start`，可直接用于完整重启。

- **菜单里的 Q 返回首页**

  首页仪表盘按 `Q` 仍是退出控制台、mihomo 留后台；进入菜单抽屉后，`Q` / `0` / 回车现在返回首页，不再直接退出。

## 保留的既有修复

- **v3.4.6** 继续保留“手动代理源测试必须是真实代理请求”的修复，但后台 supervisor 对本机单点代理改为 TCP-only，避免误伤。
- **v3.4.5** 继续保留 `gateway update latest`、指定版本不查 tag API、Cobra 错误不重复打印。
- **v3.4.4** 继续保留 Docker / ip_forward / NAT MASQUERADE 修复。
- **v3.4.3** 继续保留 `gateway update [version]`、`gateway stop` 时恢复本机 DNS。
- **v3.4.2** 继续保留 mihomo v1.19.24、anytls 支持、SIGTERM 8s grace、Linux `PostStopCleanup()`。

## 测试

- `go test ./...`
- `git diff --check`
- `go build -o /tmp/gateway-check .`
- `make build-all VERSION=v3.4.7`
- `/tmp/gateway-check --version`
- `dist/gateway-darwin-arm64 --version`

新增 / 更新测试覆盖：

- 本机 loopback external 运行时保留 gateway / TUN / DNS，并强制 `strict-route: false`
- `gateway.enabled=false` 时仍可保留纯 `17890` 端口共享模式
- 本机单点代理 supervisor TCP-only，不再执行 HTTP 健康探测
- 手动代理源测试多目标探测
- Fast.com / Netflix 测速链规则
- 菜单抽屉 `Q` 返回首页

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.7/gateway-darwin-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.7/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.7/gateway-darwin-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.7/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.7/gateway-linux-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.7/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.7/gateway-linux-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.7/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.7/gateway-windows-amd64.exe) / [.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.7/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.7/SHA256SUMS)

## 升级

```bash
sudo gateway update
```

或者指定版本：

```bash
sudo gateway update v3.4.7
```
