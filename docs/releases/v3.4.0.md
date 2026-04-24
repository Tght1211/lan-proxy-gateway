# LAN Proxy Gateway v3.4.0

**策略组自动补全**。用户订阅里只有 `select` 类型（直选节点）没有自动切换策略时，gateway 现在可以按需自动追加 `Auto` (`url-test`) + `Fallback` (`fallback`) 两个兜底组。这恢复了 v2.x 模板默认提供的能力（v3.0 重写时把它跟 IPv6 一起丢了），解决 [issue #2](https://github.com/Tght1211/lan-proxy-gateway/issues/2) 里 @lingbaoboy 报的「直选节点挂了不能自动切换」。

## ✨ 新增

- **`traffic.auto_groups` 配置项 + 菜单开关**（默认 `false`，不破坏升级用户现有 config.yaml 行为）
  - 在 `[M] → 2 分流 & 规则` 里新增第 5 项 `策略组自动补全`
  - 开启后，subscription / file 源在渲染 mihomo config 时智能判定：
    - 用户订阅里**没有 `type: url-test` 的组** → 追加 `Auto` 组，引用订阅里全部节点，300s 健康检查
    - 用户订阅里**没有 `type: fallback` 的组** → 追加 `Fallback` 组，同上
    - 两种类型都已经存在（例如订阅里有 `🚀 自动` (`url-test`) + `🛡 手动` (`fallback`)）→ 不追加，避免重复
  - 判定**按类型而不是按名字**，所以用户订阅里自定义组名（`智能选择` / `🚀 节点选择` 等）也能被正确识别
  - 组名冲突（用户已有一个叫 "Auto" 的 select 组）时自动后缀编号成 `Auto2`，避免 mihomo 报 duplicate

## ✅ 测试

新增 `internal/source/source_autogroups_test.go`，覆盖 5 种场景：

- `AutoGroupsOff` — 关闭时不追加任何组
- `AutoGroupsAppendsBoth` — 订阅只有 select 时两组都追加
- `AutoGroupsSkipsWhenUrlTestExists` — 已有 url-test 只补 Fallback
- `AutoGroupsSkipsWhenBothExist` — 两种类型都有则什么都不追加
- `AutoGroupsRenameOnNameClash` — 已有同名的别类型组时自动后缀避冲突

`go test -race ./...` + `GOOS=linux` 交叉编译全过。

## 🧭 怎么用

```text
sudo gateway
[M] → 2 分流 & 规则
5  策略组自动补全 开    ← 切到开
[Q] 返回主页 / 按下一次刷新
```

切换开关会走 `saveAndMaybeReload`：mihomo 运行中就自动热重载让新组立即生效，关着就下次 start 生效。

切到开之后，在 mihomo Web UI (`http://<本机 IP>:9090/ui/`) 的 Proxies 页或者 gateway 菜单 `[N] 切换节点` 里就能看到新的 `Auto` 和 `Fallback` 组，可以把它们当作用户 Proxy 组的选项（如果要更进一步把它们自动纳入 MATCH 兜底，等后续版本做「节点/策略分开」重构）。

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.0/gateway-darwin-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.0/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.0/gateway-darwin-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.0/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.0/gateway-linux-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.0/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.0/gateway-linux-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.0/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.0/gateway-windows-amd64.exe) / [.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.0/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.0/SHA256SUMS)

## 升级

```bash
sudo gateway stop
curl -fsSL https://raw.githubusercontent.com/Tght1211/lan-proxy-gateway/main/install.sh | sudo bash
sudo gateway start
```

升级后原有 config.yaml 里没有 `traffic.auto_groups` 字段（因为它 `omitempty`），默认即 `false`，**行为零变化**。要开的话走菜单 `[M] → 2 → 5`。

## 📝 后续路线

@lingbaoboy 在 issue #2 里提到的更大方向「节点和策略分开，便于维护更改」属于更彻底的重构（gateway 自己生成一套带 emoji 前缀的策略层，订阅只做节点供应），计划在后续 3.5 考虑。本版是最小侵入的实现：订阅的策略结构保留，gateway 只在明显缺失时补齐。
