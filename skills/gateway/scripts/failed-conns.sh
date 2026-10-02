#!/usr/bin/env bash
# 连接失败诊断
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/api-base.sh"

gateway_stats | python3 -c "
import sys, json
recent = json.load(sys.stdin)['relay']['recent']

failed = [c for c in recent if c.get('status') == 'dial_failed']
rejected = [c for c in recent if c.get('status') == 'rejected']

print('═══ 连接诊断 ═══')
print()

if failed:
    print(f'❌ 拨号失败 ({len(failed)}):')
    for c in failed[:15]:
        proto = 'UDP ' if c.get('proto') == 'udp' else ''
        via = 'PROXY' if c.get('via_proxy') else 'DIRECT'
        print(f'  {c[\"src_ip\"]:>15} → {proto}{c[\"dst_host\"]}:{c[\"dst_port\"]}  [{via}] {c[\"failure\"]}')
    if len(failed) > 15:
        print(f'  ... 还有 {len(failed) - 15} 条')
    print()

if rejected:
    print(f'🚫 规则拒绝 ({len(rejected)}):')
    for c in rejected[:10]:
        proto = 'UDP ' if c.get('proto') == 'udp' else ''
        print(f'  {c[\"src_ip\"]:>15} → {proto}{c[\"dst_host\"]}:{c[\"dst_port\"]}')
    if len(rejected) > 10:
        print(f'  ... 还有 {len(rejected) - 10} 条')
    print()

if not failed and not rejected:
    print('✓ 最近无失败或被拒连接')
"
