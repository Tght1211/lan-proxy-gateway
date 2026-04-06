# LAN Proxy Gateway v2.2.12

这一版专门修 `gateway update` 的“检查最新版本”链路。问题不在下载资产本身，而是在第一步只盯着 GitHub API；一旦这一步偶发失败，后面的 release 下载根本来不及开始。

## 重点变更

### 1. `gateway update` 新增 release 页面回退

- 之前检查最新版本主要依赖 `https://api.github.com/repos/.../releases/latest`
- 现在会先试 GitHub API；如果失败，再回退到 `https://github.com/.../releases/latest`
- release 页面本身会跳转到具体 tag，因此可以直接从跳转地址里识别最新版本号

### 2. 下载源候选逻辑统一收口

- `update` 现在和下载逻辑共用同一套候选 URL 生成方式
- 直连失败后仍会继续尝试镜像
- 同时支持通过 `GITHUB_MIRROR` 手动覆盖镜像前缀，方便在特殊网络环境里强制指定下载源

### 3. 错误信息更可排查

- 之前失败时经常只剩一句“所有下载源均失败”
- 现在会把直连 / 镜像各自的失败原因一起带出来
- 这样至少能区分是 GitHub API 不通、镜像握手失败，还是被限流

### 4. 测试补齐

- 增加了 release 跳转 URL 解析测试
- 增加了 `GITHUB_MIRROR` 覆盖候选地址的测试
- 也顺手做了轻量级全仓编译校验，确保这次改动没有影响其它命令

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.12/gateway-darwin-arm64) / [gateway-darwin-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.12/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.12/gateway-darwin-amd64) / [gateway-darwin-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.12/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.12/gateway-linux-amd64) / [gateway-linux-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.12/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.12/gateway-linux-arm64) / [gateway-linux-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.12/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.12/gateway-windows-amd64.exe) / [gateway-windows-amd64.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.12/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.12/SHA256SUMS)

## 建议验证

升级到 `v2.2.12` 后，建议顺手确认这几处：

1. 执行 `sudo gateway update`，确认能正常显示当前版本和最新版本，而不是停在“获取最新版本失败”
2. 如果 GitHub API 偶发不稳定，确认仍然能通过 release 页面跳转识别出最新 tag
3. 执行 `sudo env GITHUB_MIRROR=https://你的镜像/ gateway update`，确认手动镜像覆盖入口可用
4. 如果更新失败，确认终端错误里能看到更具体的直连 / 镜像失败原因
