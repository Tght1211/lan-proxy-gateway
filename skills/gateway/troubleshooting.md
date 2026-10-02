# 故障诊断

[← 返回 SKILL.md](SKILL.md)

诊断与修改边界以[官方 Skill 命令参考](../../internal/agentskill/content/lan-proxy-gateway/references/commands.md)为准。先确认接入模式，不要把透明网关设置应用到 HTTP/PAC 客户端；未经授权不要重启、停用认证或改系统网络。

## 快速诊断流程

```
设备无法上网？
  ├─ gateway status --json → running=false? → 确认用户是否需要启动
  ├─ 手动网关？→ 检查设备网关/DNS；HTTP/PAC？→ 检查代理地址、端口、认证
  ├─ 局域网地址不通？→ 根据地址和子网掩码检查，不假定固定 /24
  ├─ 日志报端口占用？→ 确认监听者与接入模式，再提出最小修改
  └─ 其他 → 看下面的对照表
```

## 症状对照表

| 现象 | 原因 | 解决 |
|------|------|------|
| 设备完全无法上网 | 网关未运行 / 接入配置不完整 | 检查 running 与当前接入模式；网关/DNS 和 HTTP/PAC 是不同接入配置 |
| 国外网站慢 | 上游代理节点质量差 | 在 Clash/sing-box 中换节点，不是 gateway 的问题 |
| 国内 App 变慢 | 国内域名走了代理 | 把对应域名加 direct 规则；或检查上游是否全局代理 |
| 部分网站打不开 | 客户端、直连、上游或目标服务失败 | 对比相同目标与协议的直连/代理证据；不要自动改为直连或扩大域名后缀 |
| `fake-ip 映射缺失` | 升级后设备仍连旧的虚拟地址 | 设备重连 Wi-Fi；后续自动持久化不再出现 |
| 代理健康异常 | 上游代理端口不通 | 检查 Clash/sing-box 是否运行；`health.availability < 0.95` 即告警 |
| 设备触发断路器 | 运行窗口内多目标代理失败 | 读取 `device_adaptive` 的实际阈值、窗口、保护时长与设备 until；保留显式策略 |
| DNS 失败率高 | 上游 DNS 不通 | `dns.failures / dns.queries > 5%` → 检查 223.5.5.5 等连通性 |
| 端口冲突启动失败 | 53/17892/19090 被占 | 日志会报占用进程名；关掉它或改 runtime 端口 |
| Apple TV 地区不对 | 代理节点不支持解锁 | 换支持流媒体解锁的节点 |
| 游戏 NAT 类型差 | 接入方式、系统共享或上游 NAT 限制 | HTTP 代理不承载任意 UDP；确需 NAT 诊断时检查 `/api/nat-diag`，不能仅凭 UDP 直连推断 NAT 类型 |

## 用 API 诊断

优先使用 Skill 的只读摘要工具。以下源码仓库诊断示例共用 `gateway_stats`：从指定 CLI 读取实际端口和配置目录，以内存中的管理令牌认证并绕过环境代理；不要打印令牌或把它放入 curl 参数。先在仓库根目录执行：

```bash
GATEWAY_BIN="/实际路径/gateway"
source skills/gateway/scripts/api-base.sh
```

### 检查代理健康

```bash
gateway_stats | python3 -c "
import sys, json; h = json.load(sys.stdin)['health']
if not h['healthy']:
    print(f'✗ 代理不健康！失败 {h[\"fail_count\"]} 次')
    if h.get('last_error'): print(f'  错误: {h[\"last_error\"]}')
else:
    print(f'✓ 代理健康  延迟 {h[\"latency_ms\"]:.0f}ms  可用 {h[\"availability\"]*100:.1f}%')
"
```

### 检查 DNS 健康

```bash
gateway_stats | python3 -c "
import sys, json; d = json.load(sys.stdin).get('dns')
if not d: print('DNS 未启用'); sys.exit()
rate = d['failures'] / max(d['queries'], 1) * 100
status = '✓' if rate < 5 else '✗'
print(f'{status} DNS: {d[\"queries\"]}查询  {d[\"failures\"]}失败  失败率{rate:.1f}%')
"
```

### 查看最近失败连接

```bash
gateway_stats | python3 -c "
import sys, json
recent = json.load(sys.stdin)['relay']['recent']
failed = [c for c in recent if c.get('status') == 'dial_failed']
if not failed: print('最近无失败连接'); sys.exit()
for c in failed[:10]:
    proto = f'[{c.get(\"proto\",\"tcp\")}]' if c.get('proto') == 'udp' else ''
    print(f'  {c[\"src_ip\"]} → {c[\"dst_host\"]}:{c[\"dst_port\"]} {proto} [{c[\"failure\"]}]')
"
```

### 查看被断路器保护的设备

```bash
gateway_stats | python3 -c "
import sys, json; da = json.load(sys.stdin).get('device_adaptive', {})
devices = [d for d in da.get('devices', []) if d['mode'] == 'direct']
if not devices: print('无设备处于保护状态'); sys.exit()
for d in devices[:20]:
    print(f'  ⚠ {d[\"device\"]} 保护直连中 (失败{d[\"failure_count\"]}次)')
    print(f'    涉及: {\" \".join(d[\"hosts\"][:5])}')
"
```

### 查看日志尾部

```bash
tail -50 ~/.config/lan-proxy-gateway/gateway.log
```
