# LAN Proxy Gateway v2.2.8

这一版对 TUI 控制台做了一次产品化改版，首页真正变成了 Clash 风格的状态 Dashboard，同时彻底清理了旧命令栏的残留逻辑。

## 重点变更

### 1. TUI 首页改为 Dashboard

- 启动后默认聚焦顶部，先看整体状态，再决定进入哪个工作台
- 首页一屏展示：
  - **订阅情况**：订阅名称、来源站点、到期时间、剩余流量、用量进度条
  - **当前节点**：工作模式（规则/全局/直连）、策略组、当前节点
  - **网络设置**：TUN 状态、TUN 接口、局域网共享、本机绕过代理
  - **流量统计**：上下行速度、活跃连接、上传/下载总量、内核占用、趋势图
  - **IP 链路**：当前 IP、入口 IP、出口 IP
  - **常用站点延迟**：YouTube / Google / GitHub / Apple

### 2. 顶部 Tab 精简为三个分区

- **首页**：Dashboard 总览 + 连接与流量 + IP 与延迟
- **代理**：节点与分组 + 网络设置 + 规则推荐 + 出口网络
- **订阅**：订阅管理 + 代理来源 + 扩展模式 + 住宅代理 + 状态概览

### 3. 删除底部命令栏

- 移除了旧的 `命令栏` 面板（`renderInput`）和命令补全提示（`renderCommandSuggestions`）
- 确认操作（重启/停止/退出）和参数输入（修改 URL、名称等）改为居中覆盖弹窗
- 底部区域现在只显示当前焦点区的操作提示

### 4. 菜单预览区优化

- 左侧菜单预览卡片新增"**当前摘要**"和"**下一步**"区块
- 下一步区块直接告知"回车进入"和对应快捷命令（如 `Ctrl+P`、`/tun`、`/rule` 等）
- 按 R 刷新时，"功能导航"等信息页会原地刷新，不再跳回首页

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.8/gateway-darwin-arm64) / [gateway-darwin-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.8/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.8/gateway-darwin-amd64) / [gateway-darwin-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.8/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.8/gateway-linux-amd64) / [gateway-linux-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.8/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.8/gateway-linux-arm64) / [gateway-linux-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.8/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.8/gateway-windows-amd64.exe) / [gateway-windows-amd64.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.8/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.8/SHA256SUMS)

## 建议验证

升级到 `v2.2.8` 后，建议顺手确认这几处：

1. 执行 `gateway start`，确认首页自动展示订阅流量进度条、当前节点和延迟数据
2. 按 `←/→` 切换首页/代理/订阅三个 Tab，确认左侧菜单和右侧预览正常联动
3. 在左侧菜单预览卡片中确认能看到"当前摘要"和"下一步（回车/命令）"提示
4. 在任意信息页按 `R` 刷新，确认页面原地更新，不会跳回首页
