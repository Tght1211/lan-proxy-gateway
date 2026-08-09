#!/usr/bin/env bash
# 观察自动学习：候选域名进度和已生成规则（按服务分组）
set -euo pipefail
BASE="${GATEWAY_API:-http://127.0.0.1:19090}"

curl -sf "$BASE/api/stats" | python3 -c "
import sys, json
d = json.load(sys.stdin).get('fallback')
if not d:
    print('自动学习未启用（需要代理模式）')
    sys.exit()

threshold = d['threshold']
window = d['window_hours']
candidates = d['candidates']
learned = d['learned']

print(f'═══ 自动学习 (阈值 {threshold}次/{window}h) ═══')
print()

if candidates:
    print(f'📖 学习中 ({len(candidates)} 个候选):')
    for c in candidates:
        bar = '█' * c['count'] + '░' * (threshold - c['count'])
        print(f'  [{bar}] {c[\"count\"]}/{threshold}  {c[\"host\"]}')
    print()

if learned:
    groups = {}
    for r in learned:
        g = r.get('group', '自动学习')
        groups.setdefault(g, []).append(r['value'])
    print(f'✅ 已生成直连规则 ({len(learned)} 条):')
    for g, domains in sorted(groups.items()):
        label = g.replace('自动学习 · ', '· ').replace('自动学习', '· 其他')
        print(f'  {label} ({len(domains)}):')
        for d in domains:
            print(f'    ✓ {d}')
    print()

if not candidates and not learned:
    print('正在守望：还没有域名触发回退')
"
