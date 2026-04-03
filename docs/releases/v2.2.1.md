# LAN Proxy Gateway v2.2.1

这一版重点是把运行中控制台修到真正可用，同时补上更自然的重新进入入口。

## 这一版最重要的变化

### 1. TUI 控制台不再“切一下就卡一下”

- 方向键切换分区和功能时，不再反复触发重型状态探测
- 布局从更重的三栏收成两栏，重绘更轻
- 去掉了宠物区，减少干扰和排版压力

### 2. 状态页和配置摘要改成 TUI 原生渲染

- `运行状态` 不再把 CLI 输出整块塞进 TUI
- `配置摘要` 也改成按区块显示
- 不会再出现分隔线、标题和内容相互打散的问题

### 3. 控制台重新进入更顺手

- 新增 `sudo gateway console`
- 新增 `sudo gateway console --simple`
- 退出后不需要重新 `start`，可以直接回到 TUI 或纯命令模式

### 4. 双模式切换更清楚

- 纯命令模式里把主入口文案统一成 `nodes`
- 保留 `groups` 兼容
- 纯命令模式里新增 `tui`
- TUI 里 `config` 会先留在 TUI；`/config open` 才打开完整交互式配置中心

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.1/gateway-darwin-arm64) / [gateway-darwin-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.1/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.1/gateway-darwin-amd64) / [gateway-darwin-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.1/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.1/gateway-linux-amd64) / [gateway-linux-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.1/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.1/gateway-linux-arm64) / [gateway-linux-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.1/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.1/gateway-windows-amd64.exe) / [gateway-windows-amd64.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.1/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.1/SHA256SUMS)

## 升级建议

从 `v2.2.0` 升级:

1. 使用 `sudo gateway update`
2. 执行 `sudo gateway console` 或 `sudo gateway console --simple`
3. 在 TUI 里验证 `运行状态`、`配置摘要`、`配置中心`
4. 如需兼容模式，执行 `sudo gateway start --simple`
