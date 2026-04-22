# 典型场景

## 场景一：本机已经装着 Clash Verge，想让手机也用这份代理

安装 → 选 `1) 单点代理` → 填 `127.0.0.1` + Clash 的端口（通常 `7890` 或 `7897`）→ 启动。

- 手机按「方式 2 填代理」填 `电脑IP:17890`
- macOS / Linux 上 Switch / PS5 可按「方式 1 改网关」

---

## 场景二：纯机场订阅用户

安装 → 选 `2) 机场订阅` → 粘订阅 URL → 启动。

所有订阅规则、分组、节点**全部 inline** 到 mihomo（v3.1+ 改了渲染方式，不再走 proxy-provider，机场自定义分组完整生效）。

---

## 🌟 场景三：AI 服务走纯净住宅 IP 链式代理（招牌玩法）

**用机场节点访问 Claude / OpenAI 总是被风控？换这招。**

### 问题

- 机场的出口 IP 都是**机房 ASN**（Digital Ocean / Vultr / AWS 之类），OpenAI / Anthropic 的风控系统识别度极高 —— 轻则要求验证、重则直接封号
- 但机场节点**带宽好、跨境路径稳**，直接扔掉换住宅 IP 又会卡到天荒地老
- 单独跑一个住宅 IP 落地节点？流量**全**走住宅，YouTube / Google Drive 跟着一起 —— 家庭宽带跑流量扛不住

### 这个项目的解法

内置链式代理预设（代码在 `internal/script/presets/RenderResidentialChain`）：

```
你的浏览器 / Cursor / Claude Code
          │
          ↓
    本机 gateway (TUN 抓取)
          │
          ↓
  🛫 AI起飞节点 (机场, 带宽好)  ← 跨境这一段走这里，快
          │
          ↓
  🛬 AI落地节点 (住宅 IP)       ← 最后一跳是家庭宽带，AI 网站看到的是"普通家用 IP"
          │
          ↓
    Claude / OpenAI / Cursor
```

### 操作步骤

1. 先配好订阅源（场景二）
2. 主菜单 → `3 代理 & 订阅 → S 全局扩展脚本 → 1 预设 · 链式代理`
3. 填住宅 IP 信息（服务器 / 端口 / 用户名 / 密码，SOCKS5 或 HTTP 都支持）

### 自动生成的内容

- `🛫 AI起飞节点` 组 = 订阅的所有机场节点（可按需切换）
- `🛬 AI落地节点` 组 = 住宅 IP 节点，`dialer-proxy` 指向 🛫 AI起飞节点（先机场后住宅的链式连接）
- **规则**：Claude / OpenAI / Anthropic / Cursor / Termius / ping0.cc / openai.com / anthropic.com / generativelanguage.googleapis.com 等 AI 相关域名 → `🛬 AI落地节点`
- **不误伤**：YouTube / Google Drive / GitHub 等走 `Proxy`（机场）；国内流量走 `DIRECT`；住宅 IP 带宽只给 AI 独享

### 验证出口 IP

- 本机跑 AI CLI / Claude Code / Cursor —— 切到方式 3（macOS 菜单按 `L`，Windows TUN 开着即可），访问 `https://ping0.cc` 看到的就是住宅 IP
- 服务端验证：进 `https://whoer.net` 看 ASN 类型是 "Residential" 就对了

---

## 场景四：本机浏览器直接验证出口 IP

**macOS**：按「方式 3」切本机 DNS → 浏览器开 `https://ping0.cc`，应该显示住宅 IP（不是机场 IP、不是你家宽带 IP）。

**Windows**：TUN 开着时（默认）浏览器自动走 mihomo；开 `https://ping0.cc` 看到的也应该是住宅 IP。如果想让 `DOMAIN-SUFFIX` 规则精确命中（只让 `ping0.cc` 走住宅，其它走机场）而不是靠 GeoIP 兜底，浏览器装个 SwitchyOmega 指 `127.0.0.1:17890`。
