#!/usr/bin/env bash
# 设备流量排行：各设备的流量和连接数，可选展开服务明细
set -euo pipefail
BASE="${GATEWAY_API:-http://127.0.0.1:19090}"

DETAIL="${1:-}"  # 传入设备 IP 可查看该设备的服务明细

curl -sf "$BASE/api/stats" | python3 -c "
import sys, json
data = json.load(sys.stdin)['relay']
devices = sorted(data['devices'], key=lambda x: x['up']+x['down'], reverse=True)
device_services = {ds['device']: ds['services'] for ds in data.get('device_services', [])}
detail_ip = '$DETAIL'

print('═══ 设备流量排行 ═══')
print()
print(f'{\"设备 IP\":>15}  {\"总流量\":>10}  {\"连接数\":>6}  最后活跃')
print('─' * 60)
for d in devices:
    total = (d['up'] + d['down']) / (1 << 20)
    last = d['last_seen'][:19].replace('T', ' ')
    print(f'{d[\"name\"]:>15}  {total:>8.1f}MB  {d[\"connections\"]:>6}  {last}')

    if detail_ip and d['name'] == detail_ip:
        services = sorted(device_services.get(d['name'], []),
                          key=lambda x: x['up']+x['down'], reverse=True)
        for s in services:
            st = (s['up'] + s['down']) / (1 << 20)
            print(f'  └─ {s[\"name\"]:<20} {st:>6.1f}MB  {s[\"connections\"]:>4}连接')

if not devices:
    print('暂无设备流量记录')
" 
