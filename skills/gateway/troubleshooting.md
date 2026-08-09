# 故障诊断

[← 返回 SKILL.md](SKILL.md)

## 快速诊断流程

```
设备无法上网？
  ├─ gateway status → running=false? → sudo gateway start
  ├─ 设备网关/DNS 没有都填本机 IP? → 两个都要填
  ├─ 不在同一网段? → 确保前三段相同（如都是 192.168.1.x）
  ├─ 日志报 53 端口被占? → 关掉占用程序或改 dns.port
  └─ 其他 → 看下面的对照表
```

## 症状对照表

| 现象 | 原因 | 解决 |
|------|------|------|
| 设备完全无法上网 | 网关未运行 / 设备配置不完整 | `gateway status` 确认 running；网关+DNS 都填本机 IP；同网段 |
| 国外网站慢 | 上游代理节点质量差 | 在 Clash/sing-box 中换节点，不是 gateway 的问题 |
| 国内 App 变慢 | 国内域名走了代理 | 把对应域名加 direct 规则；或检查上游是否全局代理 |
| 部分网站打不开 | 上游代理不支持该站 | 换代理节点；或加 direct 规则直连 |
| `fake-ip 映射缺失` | 升级后设备仍连旧的虚拟地址 | 设备重连 Wi-Fi；后续自动持久化不再出现 |
| 代理健康异常 | 上游代理端口不通 | 检查 Clash/sing-box 是否运行；`health.availability < 0.95` 即告警 |
| 设备触发断路器 | 2 分钟内多目标代理失败 | 15 分钟后自动恢复；检查 `device_adaptive.devices` |
| DNS 失败率高 | 上游 DNS 不通 | `dns.failures / dns.queries > 5%` → 检查 223.5.5.5 等连通性 |
| 端口冲突启动失败 | 53/17892/19090 被占 | 日志会报占用进程名；关掉它或改 runtime 端口 |
| Apple TV 地区不对 | 代理节点不支持解锁 | 换支持流媒体解锁的节点 |
| 游戏 NAT 类型差 | UDP 未被代理 | 正常行为——UDP 直连，NAT 与直接连路由器一致 |

## 用 API 诊断

### 检查代理健康

```bash
curl -s http://127.0.0.1:19090/api/stats | python3 -c "
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
curl -s http://127.0.0.1:19090/api/stats | python3 -c "
import sys, json; d = json.load(sys.stdin).get('dns')
if not d: print('DNS 未启用'); sys.exit()
rate = d['failures'] / max(d['queries'], 1) * 100
status = '✓' if rate < 5 else '✗'
print(f'{status} DNS: {d[\"queries\"]}查询  {d[\"failures\"]}失败  失败率{rate:.1f}%')
"
```

### 查看最近失败连接

```bash
curl -s http://127.0.0.1:19090/api/stats | python3 -c "
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
curl -s http://127.0.0.1:19090/api/stats | python3 -c "
import sys, json; da = json.load(sys.stdin).get('device_adaptive', {})
devices = [d for d in da.get('devices', []) if d['mode'] == 'direct']
if not devices: print('无设备处于保护状态'); sys.exit()
for d in devices:
    print(f'  ⚠ {d[\"device\"]} 保护直连中 (失败{d[\"failure_count\"]}次)')
    print(f'    涉及: {\" \".join(d[\"hosts\"][:5])}')
"
```

### 查看日志尾部

```bash
tail -50 ~/.config/lan-proxy-gateway/gateway.log
```
