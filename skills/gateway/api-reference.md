# API 响应结构参考

[← 返回 SKILL.md](SKILL.md)

外部 Agent 的操作、安全边界和只读分类分页工具以[官方命令参考](../../internal/agentskill/content/lan-proxy-gateway/references/commands.md)为准。API 地址使用 `gateway status --json` 的 `ports.api`，禁用环境代理，不要固定端口。以下是字段示例，不是当前用户数据；缺失遥测不可当作空记录。

管理 API 需要独立 Bearer 令牌；CLI 自动从当前配置目录的 `api-token` 读取。脚本通过 HTTP 库在内存中使用令牌，不输出令牌或放入进程参数。没有认证的 401 不代表核心离线；局域网代理密码不能替代管理令牌。

`GET /api/stats` 返回的完整 JSON 结构。

---

## 顶层字段

```jsonc
{
  "schema_version": 3,
  "egress": "proxy",                // "proxy" | "direct"
  "proxy": "socks5 127.0.0.1:7897", // 当前代理地址（egress=proxy 时）
  "uptime_sec": 3600,
  "relay": { /* ... */ },           // TCP 连接遥测
  "udp_relay": { /* ... */ },       // UDP 中继（fake-IP 启用时）
  "dns": { /* ... */ },             // DNS 统计
  "health": { /* ... */ },          // 代理健康
  "fallback": { /* ... */ },        // 自动学习
  "egress_health": { /* ... */ },   // 出口健康告警
  "device_adaptive": { /* ... */ }  // 设备断路器
}
```

---

## relay — TCP 连接遥测

```jsonc
{
  "up_total": 102400,               // 累计上传字节
  "down_total": 5242880,            // 累计下载字节
  "active": [/* ConnectionInfo */], // 当前活跃连接
  "recent": [/* ConnectionInfo */], // 内存中最近 72 小时、最多 2000 条已关闭连接
  "traffic": [{                     // 5 秒采样吞吐序列
    "at": "2026-08-09T12:00:00Z",
    "up": 1024, "down": 8192
  }],
  "devices": [/* UsageAggregate */],         // 按设备聚合
  "services": [/* UsageAggregate */],        // 按服务聚合
  "device_services": [/* DeviceServiceAggregate */] // 设备×服务矩阵
}
```

### ConnectionInfo

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | int | 连接唯一 ID |
| `src_ip` | string | 来源设备 IP |
| `dst_host` | string | 目标域名或 IP |
| `dst_port` | int | 目标端口 |
| `proto` | string | `"tcp"` 或 `"udp"` |
| `service` | string | 自动识别服务名（YouTube、Netflix、Steam…） |
| `up` / `down` | int | 上传/下载字节数 |
| `started_at` | datetime | 连接开始时间 |
| `ended_at` | datetime? | 连接结束时间（null = 仍活跃） |
| `via_proxy` | bool | 是否经代理 |
| `rejected` | bool | 是否被规则拒绝 |
| `status` | string | `""` / `"rejected"` / `"dial_failed"` |
| `failure` | string | 失败原因（仅 dial_failed） |
| `fallback` | bool | 连接经历了备用路径切换；需结合路由、状态、失败与响应信息判断 |

### UsageAggregate

| 字段 | 说明 |
|------|------|
| `name` | 设备 IP 或服务名 |
| `up` / `down` | 字节数 |
| `connections` | 连接数 |
| `last_seen` | 最后活跃时间 |

### DeviceServiceAggregate

```jsonc
{ "device": "192.168.1.20", "services": [/* UsageAggregate */] }
```

---

## udp_relay — UDP 中继

仅在代理模式 + fake-IP 启用时出现。

```jsonc
{ "sessions": 3, "listen": "0.0.0.0:17893" }
```

---

## dns — DNS 统计

```jsonc
{
  "queries": 1200,         // 总查询数
  "fake_answered": 980,    // fake-IP 应答数
  "forwarded": 200,        // 转发到上游的查询数
  "failures": 5,           // 失败数
  "pool_size": 980         // fake-IP 池使用量
}
```

---

## health — 代理健康

```jsonc
{
  "healthy": true,
  "latency_ms": 42.5,
  "jitter_ms": 3.2,
  "availability": 0.997,   // 0.0 ~ 1.0
  "fail_count": 0,
  "history": [{             // 探测历史
    "at": "2026-08-09T12:00:00Z",
    "latency_ms": 40.0,
    "ok": true
  }],
  "egress_identity": {      // 公网出口 IP 信息
    "ip": "1.2.3.4",
    "country_code": "US",
    "region": "California",
    "city": "Los Angeles",
    "isp": "Example ISP",
    "checked_at": "2026-08-09T12:00:00Z"
  }
}
```

---

## fallback — 自动学习

```jsonc
{
  "strategy": "direct-first",
  "settings": {
    "enabled": true,
    "confirmations": 1,
    "auto_save": true,
    "direct_wait_seconds": 5,
    "proxy_wait_seconds": 5,
    "max_direct_wait_seconds": 30,
    "cooldown_seconds": 30,
    "memory_minutes": 10
  },
  "ignored": [],
  "threshold": 1,
  "window_hours": 24,                    // 滑动窗口
  "candidates": [{                       // 学习中的候选域名
    "host": "cdn.example.com",
    "count": 2,
    "last_at": "2026-08-09T12:00:00Z"
  }],
  "learned": [{
    "type": "domain",
    "value": "cdn.googlevideo.com",
    "action": "proxy",
    "group": "自动学习 · YouTube",        // 按服务自动分组
    "learned": true
  }]
}
```

学习到的规则按域名所属服务自动分组：
- 已知服务 → `"自动学习 · YouTube"`、`"自动学习 · Google"` 等
- 未识别域名 → `"自动学习"`（通用分组）

当前仅对未匹配规则的域名先尝试直连；直连失败或满足无响应重试条件后，代理收到响应数据才计入证据。默认 1 次，阈值可配置为 1–10 次，观察窗口为 24 小时。显式规则和禁止直连策略优先。新保存的是精确域名代理规则；`learned` 数组可能同时保留旧版直连/后缀规则，必须按 `action` 和 `type` 区分。`ignored` 仅表示暂停学习，不是拒绝访问。数千条记录先在本地聚合分类，再分页输出，不能把分页列表用于替换全量路由。

---

## egress_health — 出口健康告警

```jsonc
{
  "proxy_down": false,                   // 代理端口是否被判定不可用
  "since": "2026-08-09T12:00:00Z",      // proxy_down 开始时间
  "actions": [{ "at": "...", "text": "..." }], // 自动执行的动作
  "alerts": ["..."],                     // 告警消息
  "direct_failures": [{                  // 切直连后仍失败的域名
    "device": "192.168.1.20",
    "host": "restricted.example.com",
    "reason": "connection refused"
  }]
}
```

---

## device_adaptive — 设备断路器

```jsonc
{
  "threshold": 5,                        // 触发阈值（不同目标数）
  "window_seconds": 60,                  // 观察窗口
  "direct_seconds": 300,                 // 保护直连持续时间
  "devices": [{
    "device": "192.168.1.50",
    "mode": "direct",                    // "observing" | "direct"
    "failure_count": 6,
    "hosts": ["api.example.com", "cdn.example.com"],
    "since": "2026-08-09T12:00:00Z",
    "until": "2026-08-09T12:05:00Z"
  }]
}
```
