# LAN Proxy Gateway v3.4.1

修两个 [issue #2](https://github.com/Tght1211/lan-proxy-gateway/issues/2) 里 @lingbaoboy 在 v3.4.0 升级后报的 bug：systemd 启动时机过早导致 `no IPv4 address on enp0s1`，以及 `auto_groups` 开启后 `Auto`/`Fallback` 组在 mihomo UI 看得见但 Proxy 组里选不到。

## 🐛 修复

### 1. systemd unit 启动时机

`gateway install` 生成的 `lan-proxy-gateway.service` 改用 network-online.target：

```ini
[Unit]
Description=LAN Proxy Gateway
After=network-online.target
Wants=network-online.target
```

之前是 `After=network.target`，这只表示"网络栈起来了"，但接口可能还在 DHCP 拿地址。Debian 13 + 物理 / 虚拟机 DHCP 场景下，`gateway start` 跑在拿到 IP 之前，`detectNetwork()` 直接报错退出，systemd 按 `RestartSec=5` 重试 1～2 次才能成功，期间 journalctl 全是 `no IPv4 address on enp0s1` 噪音。

**升级方法（重新生成 service 文件即可）：**

```bash
sudo gateway stop
curl -fsSL https://raw.githubusercontent.com/Tght1211/lan-proxy-gateway/main/install.sh | sudo bash
sudo gateway install   # 重写 /etc/systemd/system/lan-proxy-gateway.service
sudo systemctl daemon-reload
sudo systemctl restart lan-proxy-gateway
```

### 2. `auto_groups` 开启后 Proxy 组没引用新加的自动组

v3.4.0 引入的 `traffic.auto_groups: true` 会向 `proxy-groups:` 追加 `Auto` (url-test) 和 `Fallback` (fallback) 两个兜底组。但用户的 **Proxy** 组（`MATCH,Proxy` 的目标）`proxies` 列表里没有引用它们，导致这两个组在 mihomo UI 列表里能看到，可在 Proxy 组的下拉里选不到——和 @lingbaoboy 反馈的"看得见用不着"对应。

v3.4.1 起，开启 `auto_groups` 时会**额外**把所有 `type: url-test` / `type: fallback` 的组名 + `DIRECT` 追加到 Proxy 组的 `proxies` 列表（已存在则跳过）。

**几条设计取舍：**

- **追加到尾部，不动头部**：mihomo 的 `select` 组第一项是初始默认选择。插队会偷偷改用户当前默认走的节点，所以新项一律 append 到末尾，用户原有的默认不变
- **DIRECT 也注入**：节点全挂时给本机一个直连兜底，避免基础联网（DNS / 局域网管理 / 自身 SSH）跟着挂掉。用户原本就在 Proxy 里挂了 DIRECT 的不会重复加
- **只动名为 "Proxy" 的 select 组**：base rules 的 `MATCH,Proxy` 指向它，是 gateway 的"主入口"。订阅里别的自定义分组（区域 / 协议 / 媒体）不动，避免改动用户精心设计的策略结构
- **看类型不看名字**：用户订阅原本就有 `🚀 自动` (url-test) + `🛡 兜底` (fallback)，开启 `auto_groups` 时不会重复追加，但**会**把这两个已存在的智能组注入 Proxy 组——之前手动加这俩组的人也能享受 v3.4.1 的注入

**升级行为：** 没开 `auto_groups` 的用户行为零变化。开了的用户重启 / 热重载后，`[N] 切换节点` 在 Proxy 组下能看到 Auto / Fallback / DIRECT 多了几个候选项，可以切过去。

## ✅ 测试

`internal/source/source_autogroups_test.go` 新增 3 个场景：

- `AutoGroupsInjectsExistingSmartGroupsIntoProxy` — 订阅已有 `🚀 自动` (url-test) + `🛡 兜底` (fallback) 但 Proxy 没引用，开启后这两个 emoji 名字会被注入 Proxy
- `AutoGroupsAugmentsFallbackProxyGroup` — 订阅压根没 Proxy 组（gateway 自动补了一个），增强逻辑也覆盖到这个合成出的 Proxy 组
- `AutoGroupsOff_ProxyGroupUntouched` — `auto_groups=false` 时 Proxy 组的 `proxies` 列表必须与订阅原值一字不差（升级零侵入）

`AutoGroupsAppendsBoth` 改为同时断言 Proxy 组 `proxies` 含 `n1, n2, n3, Auto, Fallback, DIRECT` 且头部仍是 `n1`。

`go test ./...` + `GOOS=linux go build` 全过。

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.1/gateway-darwin-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.1/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.1/gateway-darwin-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.1/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.1/gateway-linux-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.1/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.1/gateway-linux-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.1/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.1/gateway-windows-amd64.exe) / [.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.1/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.1/SHA256SUMS)

## 升级

```bash
sudo gateway stop
curl -fsSL https://raw.githubusercontent.com/Tght1211/lan-proxy-gateway/main/install.sh | sudo bash
# Linux 用户：systemd unit 这版变了，重装一次 service 文件
sudo gateway install
sudo gateway start
```

## 📝 后续

@lingbaoboy 提的「节点和策略分开，订阅只供节点 / gateway 自己生成完整策略层」仍然是 3.5 的方向。本版继续走最小侵入：保留订阅原结构，只在用户**主动开** `auto_groups` 时补齐 Proxy 组里的可选项。
