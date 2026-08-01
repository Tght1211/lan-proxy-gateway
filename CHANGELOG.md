# Changelog

## v4.2.0 - 2026-08-01

### 自学习 2.0（出口健康状态机）

- 域名级状态机：normal → directTest → directHold，代理连续失败 3 次（或快速域名 1 次）即切直连测试；直连有效后固定直连并每 3 分钟探测代理恢复。
- 代理恢复探测成功后切回代理并自动移除对应的学习直连规则，同时标记为"快速域名"——之后该域名代理只要 1 次失败立即切直连，响应更快。
- 探测连续 3 次失败则固定直连并标记告警。
- 代理"连上了但零返回数据"也计入失败，不再只认拨号失败。
- 学习规则统一归入"自动学习"分组，可在编辑器中单独编辑或删除。

### 全局出口异常告警

- 每 30 秒探测上游代理端口；不可用时全部设备立即强制直连（reject 规则仍生效），恢复后自动切回。
- App 在代理异常或存在告警域名时显示红色呼吸灯边框 + 顶部告警横幅，展示已采取措施时间线和"切换直连后仍访问异常"的设备/域名列表。
- stats API 新增 `egress_health` 字段（代理状态、措施、告警域名、直连后仍失败记录）。

### 设备级前置开关

- 新规则类型 `src-ip`：按设备 IP 强制直连/代理，优先级最高，命中即定出口并跳过所有域名规则。
- 设备与服务页每台设备加"默认/强制直连/强制代理"选择器，写入"设备开关"分组。

### 分流规则分组与编辑器重构

- 规则支持 `group` 元数据字段；编辑器改为左侧分组目录 + 右侧规则窗格，告别默认全展开。
- 内置预设分组：YouTube、Netflix、Disney+、AI 服务、Telegram；支持自定义分组、整组动作批量设置、跨组移动规则、跨组重复规则提醒。
- 文本模式用 `# == 分组: 名称 ==` 标记分组，往返不丢。
- 规则行视觉重做：动作改为彩色胶囊，移动/删除合并为溢出菜单，本地序号 + 全局优先级区间。

### 修复与体验

- 配置自愈：代理模式下 Normalize 强制 `fake_ip=true`、`quic_block=true`，修复 v4.0.0 写入的不安全默认值（YouTube 打不开的根因）。
- 运行日志显示横向/纵向滚动条，自动刷新时水平位置固定在最左。
- 开机自启开关旁增加"已开启/已关闭"文字状态，深色主题下可辨。
- 安装 CLI 后提示"新版核心需重启后生效"并提供一键重启核心按钮。
- 设备智能打标：基于流量域名自动识别 Switch/PlayStation/电脑/手机，设备 10 分钟无流量即失效，手动标签持久保留且始终优先。

### 文档

- README 重构为简短首页 + `docs/` 分层文档索引（设备接入、AI 配置提示词、App 说明、v3→v4 迁移、实机结果）。

## v4.1.0 - 2026-07-24

### macOS App

- Rebuilt the native App around four sections: network overview (interactive traffic topology, throughput and latency/jitter/availability), devices & services (usage ranking, ping-probed onboarding wizard), connection history (connection-level outcomes with filter chips and success rate), and settings.
- Added a routing rules editor with drag reordering and Clash-style text mode; learned rules round-trip a trailing `# 自动学习` comment.
- Added four built-in themes (暖沙米 default, 经典浅色, 石墨深色, 海雾蓝), each with its own palette, corner radius, typography, shadows and background texture.
- Added sheet-based proxy configuration with a non-privileged connectivity test, update check against GitHub releases, and a unified scrollable page scaffold.

### Gateway core

- Added ordered first-match routing rules (`domain`, `domain-suffix`, `ip-cidr` → `proxy`/`direct`/`reject`) with hot reload.
- Recorded connection outcomes in history: rejected, dial-failed with classified reasons, and no-data connections.
- Added proxy→direct fallback with conservative auto-learning: a host whose proxy dial fails but direct retry succeeds three times within 24 hours is promoted to a deletable learned direct rule, persisted across restarts and surfaced via `/api/stats`.
- Bounded in-memory connection history to 2000 records and 72 hours; nothing is written to disk.

### CLI

- Added `gateway routing list [--json]` showing rule priority and the learned marker.
- Added `gateway system-proxy test` to probe a candidate proxy without saving it.

### Fixed

- Persist fake-IP-to-domain mappings across gateway restarts instead of losing every active device mapping with the daemon process.
- Extend the default idle retention from 10 minutes to 7 days and raise the bounded LRU capacity for long-lived phone, TV, and console caches.
- Rate-limit repeated missing fake-IP warnings per address while continuing to reject unknown mappings safely.
- Save cache snapshots atomically with mode `0600`, validate restored entries, and ignore corrupt, expired, duplicate, or out-of-range data without blocking DNS startup.

## v4.0.4 - 2026-07-21

### Device onboarding

- Added a same-subnet device IP recommendation to `gateway status`, preferring `.112` while avoiding the gateway host and router addresses.
- Changed terminal guidance to mirror device network fields: manual/static IP, subnet mask, Android prefix length, gateway/router, both DNS fields, device proxy, and Android Private DNS.
- Clarified that the device IP is unique per device while gateway and DNS all use the gateway host address.

### Documentation

- Reorganized the README around prerequisites, installation, first-time setup, proxy configuration, and verification.
- Added annotated Nintendo Switch and Android network-setting screenshots.
- Added a copy-ready prompt for terminal-capable AI tools to install, configure, verify, and print device settings safely.
- Unified DNS guidance across Switch, PS5, phones, TVs, Apple TV, and the FAQ.
- Replaced the failing dynamic Star History image with a repository-local chart generated from GitHub Stargazer data.

## v4.0.3 - 2026-07-21

### Fixed

- Fixed HTTP CONNECT requests containing duplicate `Host` headers, which caused some mixed HTTP/SOCKS proxy endpoints to close every LAN connection with `unexpected EOF`.
- Fixed successful HTTP CONNECT tunnels being closed through the response body lifecycle when the upstream returned a standard bodyless `200 Connection established` response.
- Added regression coverage for strict CONNECT header handling, bodyless successful responses, and safe loading of configurations left by pre-release UDP experiments.

### Documentation

- Added real-device Fast.com and Nintendo Switch results, including YouTube access through the LAN gateway.
- Clarified that gateway and the external proxy do not require TUN, that QUIC falls back to TCP in proxy mode, and that node selection and routing remain the external proxy software's responsibility.
- Corrected phone and PlayStation setup guidance for DNS, NAT expectations, and external routing rules.

## v4.0.2 - 2026-07-20

### Fixed

- Fixed the relay incorrectly terminating every long-lived TCP connection after two minutes. The drain timeout now starts only after one direction reaches EOF, preventing video streams such as YouTube from being interrupted and reconnected.
- Preserved Go's optimized TCP copy path, including zero-copy `splice` on Linux, by recording transfer totals after each copy direction completes instead of wrapping every write.

## v4.0.1 - 2026-07-20

### Fixed

- Enabled fake-IP by default in proxy mode while keeping DNS hijacking disabled. The relay now passes original domains to Clash/sing-box instead of forwarding potentially polluted local DNS results as destination IPs.
- Fixed sites such as YouTube failing when the external proxy depends on domain-based rules.

## v4.0.0 - 2026-07-20

### Breaking changes

- Rebuilt the project as a focused LAN bypass gateway. This is not an in-place compatible v3 upgrade.
- Removed the bundled mihomo engine, subscriptions, nodes, rule sets, GeoIP, scripts, WebUI, live traffic dashboard, device naming, and Windows support.
- Legacy configuration is backed up and requires v4 onboarding again.

### Gateway

- Added an in-process transparent TCP relay with direct, SOCKS5, and HTTP CONNECT egress.
- Added native macOS pf and Linux iptables rule management.
- Added an IPv4 DNS forwarder. v4.0.1 corrects its proxy-mode defaults so external proxy software receives domains without globally intercepting DNS.
- Proxy mode blocks QUIC so browsers fall back to TCP; other UDP remains direct.
- Added detached daemon lifecycle, hot configuration reload, status API, and precise firewall cleanup.

### Proxy configuration

- Added `gateway system-proxy status|on|off`.
- On macOS, the command updates HTTP/HTTPS or SOCKS5 settings through `networksetup` and uses the same endpoint for LAN gateway traffic.
- On Linux, the endpoint configures LAN gateway egress without modifying host desktop proxy settings.
- Proxy nodes and traffic rules remain the responsibility of external Clash, Mihomo, or sing-box software.

### Interface and distribution

- Replaced the live dashboard with a one-level terminal control panel: start/stop, proxy, device settings, and recent logs.
- Kept `gateway update` with a mandatory migration notice and explicit confirmation.
- Limited builds to macOS and Linux on amd64/arm64.
- Rewrote README and operational documentation around low-power Mac mini and small Linux hosts.
