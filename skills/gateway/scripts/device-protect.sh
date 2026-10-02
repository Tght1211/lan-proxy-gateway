#!/usr/bin/env bash
# 设备断路器状态：显示触发保护直连的设备及其失败详情
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/api-base.sh"

gateway_stats | python3 -c "
import sys, json
data = json.load(sys.stdin)
da = data.get('device_adaptive', {})
devices = da.get('devices', [])
threshold = da.get('threshold', 5)
window = da.get('window_seconds', 60)
cooldown = da.get('direct_seconds', 300)

print(f'═══ 设备断路器 (阈值 {threshold}次/{window}s，保护 {cooldown//60}min) ═══')
print()

protected = [d for d in devices if d['mode'] == 'direct']
observing = [d for d in devices if d['mode'] == 'observing' and d.get('failure_count', 0) > 0]

if protected:
    print(f'⚠ 保护直连中 ({len(protected)}):')
    for d in protected[:20]:
        until = d.get('until', '')[:19].replace('T', ' ') if d.get('until') else '未知'
        print(f'  {d[\"device\"]:>15}  失败 {d[\"failure_count\"]} 次  恢复时间: {until}')
        if d.get('hosts'):
            print(f'                   涉及: {\" \".join(d[\"hosts\"][:5])}')
    print()

if observing:
    print(f'👀 观察中 ({len(observing)}):')
    for d in observing[:20]:
        print(f'  {d[\"device\"]:>15}  失败 {d[\"failure_count\"]}/{threshold}')
    print()

if not protected and not observing:
    print('✓ 所有设备正常，无断路器触发')

# 补充：看看 egress_health 中是否有直连后仍失败的记录
eh = data.get('egress_health', {})
df = eh.get('direct_failures', [])
if df:
    print(f'📌 直连后仍失败 ({len(df)}):')
    for f in df[:20]:
        print(f'  {f[\"device\"]:>15} → {f[\"host\"]}  [{f[\"reason\"]}]')
"
