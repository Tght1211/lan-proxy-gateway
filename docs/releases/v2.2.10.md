# LAN Proxy Gateway v2.2.10

这一版主要修安装脚本下载 release 资产时“看起来像卡死”的问题。核心思路不是单纯拉长超时，而是让脚本更快判断当前下载源是不是靠谱，并在持续低速时尽早切走。

## 重点变更

### 1. 修正 `install.sh` 长时间卡在 GitHub 资产下载的问题

- 之前的 Unix 安装脚本会优先直连 GitHub release 资产
- 在“没有断、但速度很慢”的网络里，这一步可能会挂很久，看起来像卡住
- 现在脚本会先快速探测可用下载源，再进入正式下载

### 2. 新增下载源探测、低速切换和候选轮换

- 安装脚本现在会为 API 和二进制资产构建候选下载源列表
- 会先用小范围探测请求验证候选源是否可用
- 正式下载时加入低速阈值判断；如果长时间低于阈值，就会主动失败并切到下一个候选源
- 仍然支持手动指定 `GITHUB_MIRROR`

### 3. 默认优先 `curl --http1.1`

- 在部分 macOS 网络环境里，HTTP/1.1 比默认链路更稳定
- 安装脚本现在默认会带上 `--http1.1`
- 如果确实需要改回去，也可以通过环境变量关闭

### 4. README 和安装提示同步更新

- README / README_EN 的安装部分已经补上自动探测下载源的说明
- 安装完成后的终端提示，也同步成当前版本的默认 simple 模式启动文案

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.10/gateway-darwin-arm64) / [gateway-darwin-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.10/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.10/gateway-darwin-amd64) / [gateway-darwin-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.10/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.10/gateway-linux-amd64) / [gateway-linux-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.10/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.10/gateway-linux-arm64) / [gateway-linux-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.10/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.10/gateway-windows-amd64.exe) / [gateway-windows-amd64.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.10/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.10/SHA256SUMS)

## 建议验证

升级到 `v2.2.10` 后，建议顺手确认这几处：

1. 执行 `./install.sh`，确认会先获取版本，再较快进入可用下载源，而不是长时间停在下载提示上
2. 在直连 GitHub 偏慢的网络里，确认脚本会自动切换候选源，而不是一直卡在同一个大文件连接上
3. 执行 `GITHUB_MIRROR=https://hub.gitmirror.com/ ./install.sh`，确认手动镜像入口仍然可用
4. 安装完成后查看终端提示，确认 `gateway start` 和 `gateway start --tui` 的说明与当前默认模式一致
