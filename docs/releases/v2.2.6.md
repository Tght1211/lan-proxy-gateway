# LAN Proxy Gateway v2.2.6

这一版把运行中控制台继续往“真正能用的工作台”推进了一步。除了收口导航和刷新反馈，还把一批高频配置动作直接做进了 TUI 和纯命令模式。

## 这一版最重要的变化

### 1. TUI 工作台现在能直接改常用配置

- `策略与节点` 里新增了：
  - 代理来源
  - 规则开关
  - 节点与测速
- `扩展` 里新增了：
  - 扩展模式
  - 住宅代理
- `系统` 里新增了：
  - 运行模式

这些工作台可以直接在 TUI 内切换:

- 代理来源 `url / file`
- 订阅链接 / 本地配置路径 / 订阅名称
- `TUN` 与 `bypass_local`
- 国内直连、广告拦截等推荐规则
- `chains / script / off`
- `rule / global`
- 住宅代理服务器、端口、协议、认证和机场组

### 2. 刷新和边界反馈更像一个完整页面

- 按 `R` 时，现在会看到整页的轻微脉冲反馈
- 标题、边框和命令栏会一起响应
- 在左侧导航区误按 `←/→` 时，会出现一个短暂的红框反馈
- 同时会明确提示: `先按 Esc 回顶部`
- 这样不用再猜“为什么没切过去”或者“现在焦点到底在哪”

### 3. 纯命令模式不再只是看信息

现在简单模式也支持同一批核心配置动作，例如:

- `proxy source url`
- `proxy url https://example.com/sub`
- `tun on`
- `bypass off`
- `rule ads off`
- `extension chains`
- `chain mode global`
- `chain airport Auto`

### 4. 安装完成页补上双模式入口

- `gateway install` 完成后，现在会直接提示:
  - `sudo gateway start`
  - `sudo gateway start --simple`
  - `sudo gateway console`
  - `sudo gateway console --simple`
- 安装完马上就能知道默认工作台、简单模式和重进入口

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.6/gateway-darwin-arm64) / [gateway-darwin-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.6/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.6/gateway-darwin-amd64) / [gateway-darwin-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.6/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.6/gateway-linux-amd64) / [gateway-linux-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.6/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.6/gateway-linux-arm64) / [gateway-linux-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.6/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.6/gateway-windows-amd64.exe) / [gateway-windows-amd64.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.6/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.6/SHA256SUMS)

## 升级建议

升级到 `v2.2.6` 后，建议顺手验证三处:

1. 在 TUI 左侧导航区按 `←/→`，确认不会误切顶部 tab，而是出现红框和 `Esc` 提示
2. 在 TUI 中依次打开 `代理来源`、`运行模式`、`扩展模式`，确认可以直接改配置
3. 重新运行一次 `gateway install`，确认完成页已经显示默认模式、简单模式和 `console` 重进入口
