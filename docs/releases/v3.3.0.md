# LAN Proxy Gateway v3.3.0

**首页升级版**。首页仪表盘新增「真实出口 IP / 位置 / ISP」一行（通过 ipinfo.io 查），更准确反映链式代理 / 住宅 IP 的实际落地；切节点列表改成 Linux `ls` 风格多列网格 + `ll` 单列详细表；日志 tail 自动折叠连续重复的告警。同时修复 systemd service 在非 root 配置路径下找不到 `gateway.yaml` 的问题（issue #2）。

## ✨ 首页 & 控制台

- **🛬 落地下面新增「真实出口」行**：通过本机 mihomo mixed port 向 `https://ipinfo.io/json` 取真实落地 IP / 城市 / ISP，比「节点名里的国旗 emoji」更可靠。链式代理、IP 轮换、多跳出口场景下这行能反映真实情况。
  - 异步缓存 30 秒 TTL；首次显示「查询中…」，后续刷新瞬间命中缓存。
  - 后台 goroutine 刷新，不阻塞首页 `fetchCtx`，保持手动刷新 = 看上次快照的静态 dashboard 哲学。
  - 免费版 ipinfo.io 有 1000 次/天上限，30s 缓存 + 只在 mihomo 运行时查，不会打爆额度。

- **切节点列表改 Linux 风格**：
  - `ls` → 多列紧凑网格（列优先，第一列是延迟最低的几个节点），CJK / emoji 按显示宽度对齐
  - `ll` → 单列详细表格（`# / 当前 / 延迟 / 节点`）
  - 默认进入分组只显示前 20 个，满 20 提示输入 `ls` / `ll` 看全
  - 终端宽度按 `COLUMNS` 环境变量动态适配，窄终端自动退化为单列

- **日志 tail 折叠刷屏**：同一条超时 / 拒绝告警每 15 秒重复一次的场景下，折叠成：
  ```
  🟡 02:37:57  TCP 直连 115.190.130.195:21114 ... → 目标无响应（超时）
     ⋮ 上面那一行又重复 26 次（最近 02:44:51）
  🟡 02:44:20  TCP 直连 pantanal-intelli-cn.allawntech.com:443 ... → 目标无响应（超时）
  ```
  `renderTail` 和 `tailFollow` 都套了 `lineDeduper`，原始模式（按 `r` 切换）不折叠。

- **启动/重启/停止菜单的 sudo 提示精修**：运行中时给温和提示「重启/停止通常不需要 sudo；切 TUN 或绑 53 端口才需要」，未运行时才给强警示「启动需要 sudo」。与代码行为一致，不再对已在跑的场景过度告警。

- **代理 & 订阅界面 no-op 编辑不再误触发 reload**：订阅 URL / 名称 / 文件路径 / 脚本界面全按回车接受默认值时，比较前后 `SourceConfig` 发现没变就静默返回，不再 `Save` / `Reload`。

## 🐛 修复

- **subscription 重载在 TUN/fake-ip 下不再自咬**（Codex 完成）：`engine.Reload` 改为"先通过**老的** mixed port 拉订阅，再 Stop + Start"，避免 198.18.0.0/15 fake-ip 回环导致 `read: can't assign requested address`。新增 `source.MaterializeWithOptions` / `source.TestWithOptions` + `source.LocalMixedProxyURL(port)` 辅助。

- **订阅 / 文件内容兼容性**（Codex 完成）：
  - 去 UTF-8 BOM
  - 容忍 `#!MANAGED-CONFIG` 风格注释头
  - 看起来是 Clash YAML 的 base64 内容自动解码
  - 兜底 `Proxy` 组生成改为 structured yaml，避免后续 `yaml.Unmarshal` 爆错

- **systemd service 缺 HOME 环境变量** ([issue #2](https://github.com/Tght1211/lan-proxy-gateway/issues/2))：`gateway install` 生成的 `lan-proxy-gateway.service` 现在会把调用 `install` 时的用户 HOME（优先 `SUDO_USER` 的 home）写入 `Environment=HOME=...` 和 `Environment=XDG_CONFIG_HOME=.../.config`。之前 systemd 以 root 跑 service 时 HOME 默认 `/root`，读不到用户之前以 `sudo gateway install` 放在 `/home/user/.config/` 的 `gateway.yaml`。

- **macOS 切本机 DNS 到 loopback 时顺便关系统 HTTP/SOCKS 代理**：方式 3（DNS 劫持）和系统代理并存时系统代理会抢走流量，DNS fake-ip 这条路被架空。`SetLocalDNSToLoopback` 现在同时把 `webproxystate` / `securewebproxystate` / `socksfirewallproxystate` 关掉（只关 state 不动 server/port，用户下次自己再开）。

## 🏗 基础设施

新增 3 个 internal 包，全部带测试：

- `internal/ipinfo/` — ipinfo.io 客户端（Fetch + Info.ISP() 智能截断 + 单测）
- `internal/geoip/` — maxmind mmdb 读取 + 国旗 emoji 映射（配合链式代理落地显示）
- `internal/devices/` — LAN 设备 IP → 名字映射：先查用户打的手动标签，再反向 DNS，最后回退 IP

配套功能：

- `gateway.yaml` 新增 `gateway.device_labels: {ip: name}`（菜单里「给 IP 起名字」写入），仪表盘设备表按此优先显示
- `internal/engine/api.go` 新增 `GetConnections` + `ConnectionsSnapshot` + `Connection.Metadata`，仪表盘实时速率/连接数/设备聚合都走它

## ✅ 测试

新增或更新：

- `internal/console/dashboard_test.go` — 5 个 `drawEgressLine` 分支 + 位置字段去重
- `internal/console/node_list_test.go` — 节点排序 + preview/compact/verbose 三种模式 + 列优先布局 + 窄终端降级单列
- `internal/console/logview_test.go` — 重复行折叠 + Flush 末尾累计摘要
- `internal/source/source_proxy_test.go` — 订阅 fetch 通过 proxy URL 路由
- `internal/source/source_compat_test.go` — BOM / base64 / `#!MANAGED-CONFIG` 兼容
- `internal/engine/render_chain_test.go` — 链式代理渲染
- `internal/ipinfo/ipinfo_test.go` — ISP 剥 AS 号 + 超长截断
- `internal/devices/devices_test.go` — 标签优先级 + 回退
- `internal/geoip/geoip_test.go` — ISO 码 → 国旗 emoji

全仓 `go test ./...` + `-race` 全过；`GOOS=linux` 交叉编译通过。

## 🚧 已知未修（接下来版本处理）

- **节点/配置分离，支持 v2rayN base64 节点列表**：当前只能导入 Clash YAML 订阅；有用户希望直接导入 base64 节点链接再由 gateway 生成配置（[issue #2](https://github.com/Tght1211/lan-proxy-gateway/issues/2) 追加需求）。计划在后续小版本实现。
- **Linux 开启 gateway 后本机 IPv6 从外部 ping 不通**（[issue #2](https://github.com/Tght1211/lan-proxy-gateway/issues/2) 追加反馈）：需要 Debian 真机现场定位 `ip -6 route` 和 mihomo fake-ip 对 IPv6 的劫持范围，暂留 issue。

## 下载

| 系统 | 直接下载 |
|---|---|
| macOS Apple Silicon | [gateway-darwin-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.3.0/gateway-darwin-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.3.0/gateway-darwin-arm64.tar.gz) |
| macOS Intel | [gateway-darwin-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.3.0/gateway-darwin-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.3.0/gateway-darwin-amd64.tar.gz) |
| Linux x86_64 | [gateway-linux-amd64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.3.0/gateway-linux-amd64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.3.0/gateway-linux-amd64.tar.gz) |
| Linux ARM64 | [gateway-linux-arm64](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.3.0/gateway-linux-arm64) / [.tar.gz](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.3.0/gateway-linux-arm64.tar.gz) |
| Windows x86_64 | [gateway-windows-amd64.exe](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.3.0/gateway-windows-amd64.exe) / [.zip](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.3.0/gateway-windows-amd64.zip) |

校验文件: [SHA256SUMS](https://github.com/Tght1211/lan-proxy-gateway/releases/download/v3.3.0/SHA256SUMS)

## 升级

**macOS / Linux**：

```bash
sudo gateway stop
curl -fsSL https://raw.githubusercontent.com/Tght1211/lan-proxy-gateway/main/install.sh | sudo bash
sudo gateway install   # 重新生成 systemd service（Linux 会带上 HOME 环境变量）
```

**Windows**：

```powershell
gateway stop
irm https://raw.githubusercontent.com/Tght1211/lan-proxy-gateway/main/install.ps1 | iex
gateway install
```

**Linux 旧版用户注意**：如果之前手动改过 `/etc/systemd/system/lan-proxy-gateway.service`，本版 `gateway install` 会覆盖它。重装后 service 会带上你调用 install 时的用户 HOME（通常通过 `SUDO_USER` 自动拿到），用户放在 `~/.config/lan-proxy-gateway/` 的配置从此被正确读到。
