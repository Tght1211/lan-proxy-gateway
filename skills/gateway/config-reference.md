# 配置文件参考

[← 返回 SKILL.md](SKILL.md)

配置文件位于 `~/.config/lan-proxy-gateway/gateway.yaml`。
通常通过 CLI 命令或 macOS App 修改。守护进程监听文件变更自动热加载，
也可 `POST http://127.0.0.1:19090/api/reload` 手动触发。

---

## 完整字段

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
| 自动学习 | 代理失败→直连成功→24h 内 3 次→生成直连规则 |
| 设备断路器 | 2 分钟内 5 个不同目标代理失败→临时直连 15 分钟 |
