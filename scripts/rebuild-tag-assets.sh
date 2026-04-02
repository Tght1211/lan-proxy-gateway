#!/usr/bin/env bash
set -euo pipefail

if [ $# -lt 1 ] || [ $# -gt 2 ]; then
  echo "usage: $0 <tag> [output-dir]" >&2
  exit 1
fi

TAG="$1"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${2:-$ROOT_DIR/dist/$TAG}"
TMP_DIR="$(mktemp -d "/tmp/lan-proxy-${TAG}-XXXXXX")"

cleanup() {
  git -C "$ROOT_DIR" worktree remove --force "$TMP_DIR" >/dev/null 2>&1 || true
}
trap cleanup EXIT

mkdir -p "$OUT_DIR"
git -C "$ROOT_DIR" worktree add --detach "$TMP_DIR" "$TAG" >/dev/null

build_one() {
  local goos="$1"
  local goarch="$2"
  local output="$3"
  (
    cd "$TMP_DIR"
    GOOS="$goos" GOARCH="$goarch" go build \
      -ldflags "-s -w -X main.version=$TAG" \
      -o "$OUT_DIR/$output" .
  )
}

build_one darwin amd64 gateway-darwin-amd64
build_one darwin arm64 gateway-darwin-arm64
build_one linux amd64 gateway-linux-amd64
build_one linux arm64 gateway-linux-arm64
build_one windows amd64 gateway-windows-amd64.exe

if command -v shasum >/dev/null 2>&1; then
  (
    cd "$OUT_DIR"
    shasum -a 256 gateway-* > SHA256SUMS
  )
else
  (
    cd "$OUT_DIR"
    sha256sum gateway-* > SHA256SUMS
  )
fi

echo "assets written to $OUT_DIR"
