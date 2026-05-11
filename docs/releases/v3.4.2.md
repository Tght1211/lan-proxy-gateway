# LAN Proxy Gateway v3.4.2

修两个 issue：

- [#4](https://github.com/Tght1211/lan-proxy-gateway/issues/4) — 订阅里 `type: anytls` 节点报 `unsupport proxy type: anytls`
- [#5](https://github.com/Tght1211/lan-proxy-gateway/issues/5) — 严重 bug：`gateway stop` 后局域网访问 docker 容器超时（仅当 docker 端口映射 `host:container` 不一致时复现，例如 `2228:2283`）

## 🐛 修复

### 1. anytls 协议不支持 — 升 mihomo

mihomo 一直 pin 在 `v1.18.6`（2025 年初版本）。`anytls` 协议是 mihomo `v1.19.3` 才加入的，所以订阅里只要有 `type: anytls` 的节点 mihomo 直接拒绝启动。

v3.4.2 升到 **mihomo v1.19.24**（2026-04-20 最新稳定版），同时带来：

- ✅ `anytls` 协议支持（issue #4）
- ✅ vless reality / hysteria2 / trusttunnel / xhttp 一票稳定性 fix
- ✅ `CVE-2026-33814` (`net/http`) 等多个安全补丁
- ✅ vmess http obfs / ss v2ray-plugin 别名兼容

#### ⚠️ 升级方法（必须主动触发）

`gateway install` **不会**重新下载 mihomo —— 它检查到 `/usr/local/bin/mihomo` 已存在就直接跳过这一步（避免每次重装都拉一遍内核）。所以从 v3.4.1 升到 v3.4.2 的存量用户必须用新加的 `--reinstall-mihomo` 强制覆盖：

```bash
sudo gateway stop
curl -fsSL https://raw.githubusercontent.com/Tght1211/lan-proxy-gateway/main/install.sh | sudo bash
sudo gateway install --reinstall-mihomo   # ← 关键：强制覆盖到 v1.19.24
sudo gateway start
```

升级完跑 `mihomo -v` 应当看到 `Mihomo Meta v1.19.24`。

不想升内核（继续 v1.18.6）的用户可以忽略 `--reinstall-mihomo`，gateway 二进制本身的修复（issue #5、SIGTERM grace）仍然生效。

### 2. `gateway stop` 后局域网访问 docker 容器超时

#### 病因

mihomo TUN 模式开启 `strict-route: true`（gateway 默认配置）时，会向 Linux 内核加几条高优先级 ip rule：

```
9000:	from all unreachable
9001:	from all lookup main suppress_prefixlength 0
```

`pref 9000` 那条 `unreachable` 规则的作用是兜底封掉所有非 TUN 出口。mihomo **正常 SIGTERM 退出时**会自己清这些 rule 出来。但 v3.4.1 的 SIGTERM grace 只有 **3 秒**，遇到 mihomo TUN 拆解动作多 / 路由表复杂 / 系统繁忙时来不及清完就被 SIGKILL，于是 rule 留在那里。

`pref 9000 from all unreachable` 留下后，新进 SYN 包的命中顺序是：

1. PREROUTING（内含 docker DNAT `2228 → container_ip:2283`）—— 没问题
2. **路由决策时撞 `pref 9000 unreachable`** —— 被打回 ICMP unreachable
3. 客户端看到的现象：超时

**为什么 host_port == container_port 不复现？** 那种情况下 docker DNAT 不改端口（5001→5001 还是 5001），conntrack 命中既有条目，不走完整路由决策；而端口不一致时（2228→2283）每个新连接都要重新走路由决策，撞 `unreachable` 概率大得多。

`docker compose down/up` 之所以能临时修好，是因为它会重建 docker 网络，间接刷新 conntrack 状态、绕开缓存里那条已经被撞死的路径。

#### 修法（两层防御）

**第一层：SIGTERM grace 3s → 8s**（`internal/engine/process.go`）

让 mihomo 有充足时间自己清掉 ip rule 再退出。正常情况下 mihomo 1-2s 就清完，8s 不会让用户等更久；只有 mihomo 真的卡住时才到 8s 兜底 SIGKILL。

**第二层：兜底主动清理**（`internal/platform/cleanup_linux.go`）

新增 `Platform.PostStopCleanup()` 接口方法，darwin / windows 是 no-op，Linux 上扫描 `ip rule list` + `ip -6 rule list`，把 `pref 9000-9999 + unreachable` 的条目（mihomo 的签名）删掉。`gateway.Disable()` 在 `UnconfigureNAT` + `DisableIPForward` 之后调它，所以**每次 `gateway stop` 都会做这次保险**。

只删 `pref 9000-9999` + `unreachable` 双重特征的，管理员自己加的高 pref 普通规则（比如 `9500: from 10.0.0.0/8 lookup admin_isolation`）一律不动 —— 单元测试有 7 条断言专门盯这条不变量。

## ✅ 测试

| 测试文件 | 覆盖什么 |
|---|---|
| `internal/platform/cleanup_parser_test.go` (新增) | 7 个场景：检测 mihomo 签名 / 干净系统返回空 / 不动管理员高 pref 规则 / 拒绝 pref 范围外的 unreachable / 多条 mihomo 残留全报上来 / 空和垃圾输入 |
| `internal/gateway/gateway_test.go` (新增) | `Gateway.Disable()` 调用顺序 = `UnconfigureNAT` → `DisableIPForward` → `PostStopCleanup`；接口为空时 cleanup 仍执行 |
| `internal/mihomo/download_test.go::TestPinnedMihomoVersionSatisfiesAnytls` (新增) | 版本回归栏：禁止把 mihomo pin 降回 v1.19 以下，免得 anytls 又挂 |

`go test -race ./...` 全模块过 + `GOOS={linux,darwin,windows}` 三平台 vet+build 全过。

## 📦 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.2/gateway-darwin-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.2/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.2/gateway-darwin-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.2/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.2/gateway-linux-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.2/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.2/gateway-linux-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.2/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.2/gateway-windows-amd64.exe) / [.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.2/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.4.2/SHA256SUMS)

## 🚀 升级

```bash
sudo gateway stop
curl -fsSL https://raw.githubusercontent.com/Tght1211/lan-proxy-gateway/main/install.sh | sudo bash

# 订阅里有 anytls / vless reality 等新协议节点的话，必须强制重装 mihomo 内核到 v1.19.24
sudo gateway install --reinstall-mihomo

# 没有上述新协议的话也可以省掉 --reinstall-mihomo，只装 gateway 二进制就行（issue #5 修复仍然生效）
# sudo gateway install

sudo gateway start
```

## 📝 后续

- 节点和策略分开（订阅只供节点 + gateway 自己生成完整策略层）仍然规划在 v3.5
- issue #5 的根因如果不止 ip rule 残留（比如 iptables FORWARD 链 / conntrack 缓存也有副作用），需要用户在 v3.4.2 上验证后回报 —— 拿到 `iptables-save` + `conntrack -L` 的对比就能定位下一步
