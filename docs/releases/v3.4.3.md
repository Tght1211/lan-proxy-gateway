# LAN Proxy Gateway v3.4.3

**维护与升级体验修复版**。这一版建立在 v3.4.2 之上，不回退 anytls / mihomo v1.19.24 / Linux TUN ip rule cleanup，也不改变 v3.3.2 的 IPv6/AAAA 行为；只补齐指定版本更新、本机 DNS 停止清理、设备接入指引。

## 新增

- **指定版本更新 / 回退**

```bash
sudo gateway update          # 升级到最新 release
sudo gateway update v3.4.3   # 更新或回退到指定 tag
sudo gateway update 3.3.2    # 可以省略开头的 v
```

`gateway update` 会继续自动尝试 GitHub 镜像源。更新前如果 gateway 正在运行，会先停止当前网关，替换二进制后再恢复启动；如果本机 DNS 原本指向 `127.0.0.1`，维护重启完成后会重新切回，避免更新过程改变你的本机接入方式。

## 修复

- **停止 gateway 后恢复本机 DNS**
  `gateway stop`、控制台「启动 / 重启 / 停止」里的停止、以及「关闭 gateway 并退出」现在都会检查本机 DNS。如果发现 DNS 仍指向 `127.0.0.1`，会在停止 gateway 时恢复默认 DNS，避免 gateway 已停但本机网络还被卡在本地 DNS 上。

- **设备接入指引更短**
  菜单里的「设备接入指引」改成速查表：参数、改网关、填代理、本机使用一眼能扫完，减少长段解释造成的误读。

## 保留的既有修复

- **v3.4.2** 继续保留 mihomo v1.19.24、anytls 支持、SIGTERM 8s grace、Linux `PostStopCleanup()` 清理残留 TUN strict-route ip rule。
- **v3.4.1** 继续保留 systemd `network-online.target` 修复，以及 `traffic.auto_groups` 注入 Auto / Fallback / DIRECT 到 `Proxy` 组的行为。
- **v3.3.2** 继续保留 IPv6 / DNS AAAA 默认开启，不新增关闭 IPv6 的配置项，避免功能倒退。

## 发布构建

- GitHub Actions release workflow 改用 Go 1.25，和 `go.mod` 保持一致。

## 测试

- `go test ./...`
- `git diff --check`
- `make build-all VERSION=v3.4.3`
- `dist/gateway-darwin-arm64 --version`
- 新增 `cmd/update_test.go` 覆盖指定版本规范化、release 资产名、镜像 URL、Windows 自更新脚本。
- 新增 `internal/app/app_test.go` 覆盖停止时恢复本机 DNS。
- 新增 `internal/gateway/guide_test.go` 锁定精简后的设备指引。

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.3/gateway-darwin-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.3/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.3/gateway-darwin-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.3/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.3/gateway-linux-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.3/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.3/gateway-linux-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.3/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.3/gateway-windows-amd64.exe) / [.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.3/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.3/SHA256SUMS)

## 升级

```bash
sudo gateway update v3.4.3
```

如果你从 v3.4.1 或更早版本升级，且订阅里有 anytls / vless reality 等新协议节点，升级 gateway 后还需要强制重装 mihomo 内核到 v1.19.24：

```bash
sudo gateway install --reinstall-mihomo
sudo gateway restart
```
