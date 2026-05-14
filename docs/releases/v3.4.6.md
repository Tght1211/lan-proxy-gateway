# LAN Proxy Gateway v3.4.6

**本机单点代理安全模式 hotfix**。这一版专门修复“gateway 基于本机第三方代理端口（如 `127.0.0.1:6578`）运行时，会反过来影响第三方代理软件网络”的问题。

## 修复

- **本机单点代理自动进入安全模式**

  当代理源是：

```yaml
source:
  type: external
  external:
    server: 127.0.0.1
    port: 6578
```

  或 `localhost` / `::1` 这类本机地址时，gateway 现在只做“局域网 mixed 端口共享”：

  - 保留 `mixed-port`，手机/电脑继续填 `gateway IP:17890`
  - 运行时关闭透明网关
  - 运行时关闭 TUN
  - 运行时关闭 DNS 监听

  原因是这类拓扑里 gateway 依赖同一台机器上的第三方代理软件。如果 gateway 再开启 TUN/透明路由，就可能把第三方代理软件自己的出站流量也抓回来，形成“gateway → 本机代理端口 → 第三方代理出站 → 又被 gateway TUN 接住”的回环或全链路拖慢。

  这个安全模式不改写用户配置；只是启动 / 热重载 / 渲染 mihomo 配置时使用运行时安全副本。切回订阅、文件、远程单点代理后，原本的 TUN/DNS 设置仍然保留。

- **单点代理健康检查改成真实代理请求**

  之前只测 TCP 端口是否能连上，端口开着但代理请求超时 / 502 也会被误判为健康。现在会真实通过该代理访问：

```text
http://www.gstatic.com/generate_204
```

  这样 `127.0.0.1:6578` 端口开着但实际代理不可用时，会立刻提示“端口开着，但代理请求失败”，而不是让手机端卡很久。

- **控制台文案收口**

  TUN 菜单说明现在明确区分：

  - Switch / PS5 / Apple TV 这类“改网关”设备需要 TUN
  - 手机 / 电脑手动填 `gateway IP:17890` 时，尤其是基于本机第三方代理端口时，建议关 TUN，gateway 只共享代理端口

## 保留的既有修复

- **v3.4.5** 继续保留 `gateway update latest`、指定版本不查 tag API、控制台 `Q` 退出修复。
- **v3.4.4** 继续保留 Docker / ip_forward / NAT MASQUERADE 修复。
- **v3.4.3** 继续保留 `gateway update [version]`、`gateway stop` 时恢复本机 DNS。
- **v3.4.2** 继续保留 mihomo v1.19.24、anytls 支持、SIGTERM 8s grace、Linux `PostStopCleanup()`。

## 测试

- `go test ./...`
- `git diff --check`
- `go build -o /tmp/gateway-check .`
- 新增测试覆盖：
  - loopback external proxy 检测
  - runtime safety config 不改写原始配置
  - 本机 external 渲染结果中 `tun.enable=false`、`dns.enable=false`
  - app start 在本机 external 模式下不启用透明网关
  - external/remote 源健康检查必须是真实可用代理，不能只看端口 open

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.6/gateway-darwin-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.6/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.6/gateway-darwin-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.6/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.6/gateway-linux-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.6/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.6/gateway-linux-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.6/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.6/gateway-windows-amd64.exe) / [.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.6/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.6/SHA256SUMS)

## 升级

```bash
sudo gateway update
```

或者指定版本：

```bash
sudo gateway update v3.4.6
```
