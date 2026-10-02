#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
exec python3 "$ROOT/internal/agentskill/content/lan-proxy-gateway/scripts/gateway_inspect.py" \
    --gateway "${GATEWAY_BIN:-gateway}" --view learning "$@"
