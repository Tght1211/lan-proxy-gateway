# LAN Proxy Gateway v2.2.3

这一版补的是运行中控制台里 `R` 刷新键的互动反馈。

## 这一版最重要的变化

### 1. `R` 现在会刷新当前页面

- 不再把当前内容替换成一段统一的“已刷新”提示
- 如果你正在看 `运行状态`、`配置摘要`、`功能导航` 或日志页，按 `R` 会直接刷新它本身

### 2. 刷新动作现在有短促的终端反馈

- 按下 `R` 后，当前内容区会有一个很短的脉冲反馈
- 同时保留刷新时间更新
- 效果刻意做得比较轻，不会让整个终端晃得太夸张

### 3. 文档同步更新

- README 中英版补上了 `R` 的作用
- 命令文档也补上了刷新反馈说明

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.3/gateway-darwin-arm64) / [gateway-darwin-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.3/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.3/gateway-darwin-amd64) / [gateway-darwin-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.3/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.3/gateway-linux-amd64) / [gateway-linux-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.3/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.3/gateway-linux-arm64) / [gateway-linux-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.3/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.3/gateway-windows-amd64.exe) / [gateway-windows-amd64.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.3/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.3/SHA256SUMS)

## 升级建议

从 `v2.2.2` 升级:

1. 使用 `sudo gateway update`
2. 执行 `sudo gateway console`
3. 在 TUI 里打开 `运行状态` 或 `功能导航`
4. 按 `R`，确认当前页面刷新且有短促反馈
