#!/usr/bin/env bash
# 网络概览：出口状态、活跃连接、流量和代理健康
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/api-base.sh"

gateway_stats | python3 -c "
import sys, json
d = json.load(sys.stdin)
r, h = d['relay'], d['health']
udp = d.get('udp_relay')

print('═══ 网关概览 ═══')
print(f'出口:     {d[\"egress\"]}' + (f'  ({d[\"proxy\"]})' if d.get('proxy') else ''))
print(f'运行:     {d[\"uptime_sec\"]//3600}h {d[\"uptime_sec\"]%3600//60}m')
print(f'活跃连接: {len(r[\"active\"])}')
print(f'活跃设备: {len(r[\"devices\"])}')
if udp:
    print(f'UDP 会话: {udp[\"sessions\"]}')
print(f'流量:     ↑ {r[\"up_total\"]/(1<<20):.1f} MB  ↓ {r[\"down_total\"]/(1<<20):.1f} MB')
print()

dns = d.get('dns')
if dns:
    fail_rate = dns['failures'] / max(dns['queries'], 1) * 100
    print(f'DNS:      {dns[\"queries\"]} 查询  {dns[\"failures\"]} 失败 ({fail_rate:.1f}%)')

status = '✓ 健康' if h['healthy'] else '✗ 异常'
print(f'代理:     {status}  延迟 {h[\"latency_ms\"]:.0f}ms  抖动 {h[\"jitter_ms\"]:.0f}ms  可用 {h[\"availability\"]*100:.1f}%')

ident = h.get('egress_identity')
if ident:
    loc = ' '.join(filter(None, [ident.get('country_code'), ident.get('city'), ident.get('isp')]))
    print(f'公网出口: {ident[\"ip\"]}  {loc}')

da = d.get('device_adaptive', {})
protected = [x for x in da.get('devices', []) if x['mode'] == 'direct']
if protected:
    print()
    for x in protected[:20]:
        print(f'⚠ 设备 {x[\"device\"]} 保护直连中 (失败{x[\"failure_count\"]}次)')
"
