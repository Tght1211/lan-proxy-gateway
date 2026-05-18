#!/usr/bin/env bash
# Compatibility wrapper. The real script lives at scripts/install-mihomo.sh.
# Kept at repo root so any documentation or muscle-memory pointing here keeps working.
exec "$(dirname "$0")/scripts/$(basename "$0")" "$@"
