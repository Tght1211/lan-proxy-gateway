# LAN Proxy Gateway v2.2.4

这一版继续收口运行中控制台，重点是把“信息查看”和“交互操作”分清楚。

## 这一版最重要的变化

### 1. 右侧页面现在会明确标出类型

- 会直接显示当前是 `信息页`、`可操作页` 还是 `确认页`
- 不用再看半天文本去猜这页到底能不能操作

### 2. 右侧内容区现在有真正的焦点

- 在左侧选中功能后按 `Enter`，焦点会进入右侧内容区
- 右侧内容区聚焦时，可以用 `↑/↓` 滚动
- 按 `Esc` 再回到左侧功能列表

### 3. 节点入口改成了节点工作台

- `Ctrl+P` 或 `/nodes` 打开节点工作台
- 进入后可以切换节点
- 也可以按 `T` 直接测试当前节点延迟
- 节点页不再只是提示文字，而是明确的操作页

### 4. 设备和扩展页改成了原生 TUI 页面

- `设备接入` 不再抓 CLI 输出
- `扩展状态` 也改成统一的 TUI 渲染
- 页面样式更一致，阅读成本更低

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.4/gateway-darwin-arm64) / [gateway-darwin-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.4/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.4/gateway-darwin-amd64) / [gateway-darwin-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.4/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.4/gateway-linux-amd64) / [gateway-linux-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.4/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.4/gateway-linux-arm64) / [gateway-linux-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.4/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.4/gateway-windows-amd64.exe) / [gateway-windows-amd64.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.4/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.4/SHA256SUMS)

## 升级建议

从 `v2.2.3` 升级:

1. 使用 `sudo gateway update`
2. 执行 `sudo gateway console`
3. 在 TUI 里打开 `节点与测速`
4. 验证 `Enter` 会把焦点切到右侧，`T` 能测试当前节点延迟
