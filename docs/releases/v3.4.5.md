# LAN Proxy Gateway v3.4.5

**升级链路 + 控制台退出 hotfix**。这一版修正 `gateway update` 指定版本时被 GitHub API 卡住的问题，也把控制台里 `Q` 的行为改回文案承诺的“只退出控制台，mihomo 留后台”。

## 修复

- **指定版本更新不再先查 GitHub API**

  `gateway update v3.4.4` / `gateway update 3.4.4` 现在会直接解析成指定 tag，再和当前版本比较。机器已经是目标版本时会立即返回“无需更新”，不会因为 `https://api.github.com/repos/.../releases/tags/v3.4.4` 超时而失败。

- **latest 语义明确**

```bash
sudo gateway update          # 查最新 release，并更新到最新
sudo gateway update latest   # 同上
sudo gateway update v3.4.5   # 更新或回退到指定 tag
sudo gateway update 3.4.5    # 可以省略开头的 v
```

  另外兼容 `laste` / `lastest` 这类常见手滑输入，按 `latest` 处理。

- **版本源 fallback 真的逐个尝试**

  之前检查最新版本时，所有候选源共用一个 20 秒 context。第一条 GitHub API 直连如果卡满 20 秒，后面的镜像源会拿着已经超时的 context 立刻失败，看起来“所有源都失败”，实际没有真正尝试。现在每个候选源都有独立超时预算。

- **错误不再重复打印**

  关闭 Cobra 默认错误打印，避免失败时出现 `Error: ...` 加上一遍同样错误正文的重复输出。

- **控制台 `Q` 真正退出控制台**

  首页和主菜单的 `Q` 都只退出控制台，mihomo 继续在后台跑；要停止 gateway/mihomo 仍然用主菜单 `6 关闭 gateway 并退出`。

## 文档

- README / README_EN / commands 文档补充 `gateway update latest`。

## 保留的既有修复

- **v3.4.4** 继续保留 Docker / ip_forward / NAT MASQUERADE 修复。
- **v3.4.3** 继续保留 `gateway update [version]`、`gateway stop` 时恢复本机 DNS。
- **v3.4.2** 继续保留 mihomo v1.19.24、anytls 支持、SIGTERM 8s grace、Linux `PostStopCleanup()` 清理残留 TUN strict-route ip rule。
- **v3.4.1** 继续保留 systemd `network-online.target` 修复，以及 `traffic.auto_groups` 注入 Auto / Fallback / DIRECT。
- **v3.3.2** 继续保留 IPv6 / DNS AAAA 默认开启。

## 测试

- `go test ./...`
- `go build -o /tmp/gateway-check .`
- `/tmp/gateway-check update --help`
- `git diff --check`
- `make build-all VERSION=v3.4.5`
- 新增 `cmd/update_test.go` 用例覆盖：
  - 指定版本解析不依赖网络
  - `laste` / `lastest` 归一为 latest
- 新增 `internal/console/console_test.go` 用例覆盖：
  - 主菜单 `Q` 退出控制台
  - 主菜单 `0` 返回仪表盘

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.5/gateway-darwin-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.5/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.5/gateway-darwin-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.5/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.5/gateway-linux-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.5/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.5/gateway-linux-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.5/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.5/gateway-windows-amd64.exe) / [.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.5/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.5/SHA256SUMS)

## 升级

```bash
sudo gateway update
```

或者指定版本：

```bash
sudo gateway update v3.4.5
```
