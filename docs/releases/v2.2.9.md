# LAN Proxy Gateway v2.2.9

这一版先把默认启动路径收回到 simple 模式。TUI 工作台还会继续修，但日常使用先优先兼容性和稳定性，避免一上来就把用户送进还在持续打磨的界面里。

## 重点变更

### 1. `start / console / restart` 默认进入 simple 模式

- 交互终端里执行 `gateway start`，现在默认会进入纯命令控制台
- `gateway console` 和 `gateway restart` 也沿用同样的默认行为
- simple 模式继续保留同一批核心控制能力，适合日常切代理来源、TUN、规则和 chains 配置

### 2. TUI 改为显式进入

- 新增 `--tui`，需要时可以明确进入运行中 TUI 工作台
- 支持:
  - `gateway start --tui`
  - `gateway console --tui`
  - `gateway restart --tui`
- 原有的 `--simple` 仍然可用，方便脚本或习惯显式写法的用户继续使用

### 3. 模式切换逻辑统一收口

- `start`、`console`、`restart` 现在共享同一套模式解析逻辑
- 同时传 `--simple` 和 `--tui` 时，会明确报错，避免行为不一致
- 从 simple 模式输入 `tui` 切换时，终端提示也改成了更准确的“TUI 工作台”

### 4. README / 命令文档 / 安装提示同步更新

- README 中的默认启动说明已经改成 simple 模式
- 命令手册和 advanced 文档都补上了 `--tui` 的新入口
- `gateway install` 完成后的终端提示，也改成“默认模式 + TUI 工作台”的新文案

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.9/gateway-darwin-arm64) / [gateway-darwin-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.9/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.9/gateway-darwin-amd64) / [gateway-darwin-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.9/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.9/gateway-linux-amd64) / [gateway-linux-amd64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.9/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.9/gateway-linux-arm64) / [gateway-linux-arm64.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.9/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.9/gateway-windows-amd64.exe) / [gateway-windows-amd64.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.9/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v2.2.9/SHA256SUMS)

## 建议验证

升级到 `v2.2.9` 后，建议顺手确认这几处：

1. 执行 `gateway start`，确认默认进入的是 simple 模式命令控制台
2. 执行 `gateway start --tui`，确认仍然可以显式进入运行中 TUI 工作台
3. 执行 `gateway console` 和 `gateway restart`，确认默认行为也都保持 simple 模式
4. 执行 `gateway console --tui` 或 `gateway restart --tui`，确认显式 TUI 入口可用
5. 执行 `gateway install` 后查看完成提示，确认默认模式和 TUI 工作台入口文案正确
