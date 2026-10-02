# 配置文件参考

[← 返回 SKILL.md](SKILL.md)

配置文件位于 `~/.config/lan-proxy-gateway/gateway.yaml`。
通常通过 CLI 命令或 macOS App 修改。守护进程监听文件变更自动热加载，
也可在诊断确有需要时通过实际 loopback API 端口手动 reload。涉及接入方式和监听器变更时需核实是否重启才生效。

外部 Agent 应优先使用[官方 Skill 命令参考](../../internal/agentskill/content/lan-proxy-gateway/references/commands.md)，不要整份读取或打印可能含密码的配置文件。HTTP/PAC、热点和学习设置以 CLI/API 当前返回为准。

---

## 字段示例

```yaml
version: 4

# ── 局域网网关 ──
gateway:
  enabled: true              # 是否启用旁路由功能

# ── 出口 ──
egress:
  mode: proxy                # direct | proxy
  proxy:                     # mode=proxy 时使用
    type: socks5             # socks5 | http
    host: 127.0.0.1
    port: 7897
    username: ""             # 可选认证
    password: ""

# ── 分流规则 ──
# src-ip 是设备级前置策略，不受后续域名规则顺序影响。
routing:
  rules:
    - type: src-ip
      value: 192.168.1.50
      action: direct         # direct | proxy | reject
      group: "设备开关"       # 可选，仅用于 App 分组
    - type: domain-suffix
      value: youtube.com
      action: proxy
      group: "YouTube"
    - type: domain-suffix
      value: bilibili.com
      action: direct
      group: "国内"
    - type: ip-cidr
      value: 10.0.0.0/8
      action: direct

# ── DNS 服务 ──
dns:
  enabled: true
  port: 53
  upstreams:                 # 国内 DNS，用于解析国内域名
    - "223.5.5.5"
    - "119.29.29.29"
  fake_ip: true              # 代理模式返回 fake-IP，保留原始域名给上游代理
  hijack: false              # 不劫持设备发往其他 DNS 的查询
  fake_ip_filter: []         # 追加不做 fake-IP 的域名后缀
                             # 内置: lan/local/localhost/internal/home.arpa

# ── QUIC 拦截 ──
quic_block: true             # 代理模式拒绝 UDP/443，迫使浏览器回退 TCP

# ── Runtime（通常不用改）──
runtime:
  redir_port: 17892          # 透明 TCP 转发端口
  udp_redir_port: 17893      # UDP fake-IP 转发端口
  api_port: 19090            # 本机回环状态 API 端口
  log_level: info            # debug | info | warn | error
  fake_ip_range: "198.18.0.0/16"  # fake-IP 地址段
```

---

## 常见上游代理端口

| 上游软件 | 类型 | 地址 |
|----------|------|------|
| Clash Verge / Mihomo | HTTP 或 SOCKS5 | `127.0.0.1:7897` |
| sing-box | 按本地入站类型 | 看入站配置 |
| `ssh -D 1080 host` | SOCKS5 | `127.0.0.1:1080` |

---

## 重要文件路径

| 文件 | 说明 |
|------|------|
| `~/.config/lan-proxy-gateway/gateway.yaml` | 主配置 |
| `~/.config/lan-proxy-gateway/gateway.log` | 运行日志 |
| `~/.config/lan-proxy-gateway/gateway.pid` | 守护进程 PID |
| `~/.config/lan-proxy-gateway/fakeip-cache.json` | fake-IP 映射缓存（`0600` 权限，含域名，勿分享） |
| `~/.config/lan-proxy-gateway/fallback-learn.json` | 自动学习候选状态 |
| `~/.config/lan-proxy-gateway/runtime.state` | 防火墙/IP转发回滚状态 |

---

## 代理模式下的自动行为

| 行为 | 说明 |
|------|------|
| `fake_ip` 强制 true | 代理模式保留原始域名给上游解析，无法关闭 |
| `quic_block` 强制 true | 拒绝 QUIC 迫使浏览器回退 TCP |
| 自动学习 | 未匹配域名先直连，直连失败后代理收到响应才记录；24h 内达到用户阈值（默认 1 次）后按 auto_save 保存精确域名代理规则或等待确认 |
| 设备断路器 | 以运行遥测 device_adaptive 的 threshold、window_seconds、direct_seconds 为准；不可覆盖显式策略或禁止直连要求 |

学习设置不在上述路由数组中；通过 `gateway learning configure` 传入当前完整设置 JSON，保留其他字段。撤销学习规则会同时暂停学习，恢复只恢复观察。网络出口配置只有一个代理上游；出口卡片不是多个代理配置。完整字段定义见源码 `internal/config/schema.go`，不要把示例当作用户的实际设置。
