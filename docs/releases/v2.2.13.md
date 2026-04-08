# LAN Proxy Gateway v2.2.13

这一版继续补运行中 TUI 的首页体验，重点是把常用入口补齐，并把刷新动作改成不再阻塞界面。

## 重点变更

### 1. 首页新增 `设备接入` 和 `功能导航`

- 首页左侧菜单现在除了仪表盘、连接与流量、IP 与延迟，还补上了 `设备接入` 和 `功能导航`
- 这样在 TUI 里就能直接查看手机、Switch、PS5、Apple TV 等设备的接入参数
- 也能快速回到当前主线和推荐入口，不用再切出去找命令说明

### 2. `R` 刷新改成异步执行

- 之前刷新摘要时，界面会同步等待采集完成
- 现在刷新改成后台执行，UI 会继续响应，不会因为一次刷新把整个界面卡住
- 刷新期间也会保留更明显的进行中提示和收尾脉冲反馈

### 3. 修正首页预览和刷新状态细节

- `设备接入` 预览不再错误暗示有二维码页
- 异步刷新时会对流量趋势数据做独立拷贝，避免后台刷新和前台渲染共享同一段切片状态
- 这让刷新逻辑更稳，也更适合作为后续继续扩展首页内容的基础

### 4. TUI 回归测试补齐

- 增加了首页新入口的测试
- 增加了异步刷新状态与结果处理测试
- 增加了趋势切片深拷贝和首页预览文案的测试

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.13/gateway-darwin-arm64) / [gateway-darwin-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.13/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.13/gateway-darwin-amd64) / [gateway-darwin-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.13/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.13/gateway-linux-amd64) / [gateway-linux-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.13/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.13/gateway-linux-arm64) / [gateway-linux-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.13/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.13/gateway-windows-amd64.exe) / [gateway-windows-amd64.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.13/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.13/SHA256SUMS)

## 建议验证

升级到 `v2.2.13` 后，建议顺手确认这几处：

1. 运行 `sudo gateway console --tui`，确认首页左侧已经出现 `设备接入` 和 `功能导航`
2. 进入 `设备接入`，确认能直接看到网关 / DNS / 子网掩码等参数
3. 在 TUI 里按 `R`，确认刷新期间界面不会卡死，且刷新结束后内容会正常更新
4. 反复刷新 `连接与流量` 或首页摘要，确认流量趋势显示稳定，没有异常跳变或渲染错乱
