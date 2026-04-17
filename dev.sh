#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

GO_BIN="${GO_BIN:-go}"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev-local)}"
LDFLAGS="${LDFLAGS:--X main.version=${VERSION}}"

CACHE_DIR="${CACHE_DIR:-$ROOT_DIR/.cache}"
BUILD_DIR="${BUILD_DIR:-$ROOT_DIR/.tmp}"
BINARY_NAME="${BINARY_NAME:-gateway-dev}"
BINARY_PATH="${BINARY_PATH:-$BUILD_DIR/$BINARY_NAME}"

export GOCACHE="${GOCACHE:-$CACHE_DIR/go-build}"

if [ "${USE_LOCAL_GOMODCACHE:-0}" = "1" ]; then
  export GOMODCACHE="${GOMODCACHE:-$CACHE_DIR/go-mod}"
fi

info()  { printf "\033[1;32m%s\033[0m\n" "$*"; }
warn()  { printf "\033[1;33m%s\033[0m\n" "$*"; }
error() { printf "\033[1;31m%s\033[0m\n" "$*" >&2; exit 1; }

usage() {
  cat <<EOF
用法:
  ./dev.sh build
  ./dev.sh test
  ./dev.sh test-core
  ./dev.sh run -- <gateway 参数>
  ./dev.sh start [gateway start 参数]
  ./dev.sh console
  ./dev.sh stop
  ./dev.sh restart
  ./dev.sh status
  ./dev.sh clean

常用例子:
  ./dev.sh build
  ./dev.sh test
  ./dev.sh test-core
  ./dev.sh run -- --version
  ./dev.sh run -- config show
  ./dev.sh start
  ./dev.sh console

环境变量:
  GO_BIN=go                     指定 go 可执行文件
  VERSION=dev-local             覆盖构建版本
  BINARY_PATH=.tmp/gateway-dev  覆盖输出二进制路径
  USE_LOCAL_GOMODCACHE=1        让模块缓存也落到仓库内 .cache/
EOF
}

ensure_go() {
  command -v "$GO_BIN" >/dev/null 2>&1 || error "未找到 go，请先安装 Go 1.25+"
}

prepare_dirs() {
  mkdir -p "$BUILD_DIR" "$GOCACHE"
  if [ -n "${GOMODCACHE:-}" ]; then
    mkdir -p "$GOMODCACHE"
  fi
}

build_binary() {
  ensure_go
  prepare_dirs
  info "编译本地开发二进制..."
  info "输出路径: $BINARY_PATH"
  "$GO_BIN" build -ldflags "$LDFLAGS" -o "$BINARY_PATH" .
}

run_tests() {
  ensure_go
  prepare_dirs
  info "运行全量测试..."
  "$GO_BIN" test ./...
}

run_core_tests() {
  ensure_go
  prepare_dirs
  info "运行核心包测试..."
  "$GO_BIN" test ./internal/config/... ./internal/traffic/... ./internal/source/... ./internal/engine/...
}

run_binary() {
  build_binary
  info "运行: $BINARY_PATH $*"
  "$BINARY_PATH" "$@"
}

run_with_sudo_if_needed() {
  local subcmd="$1"
  shift

  build_binary

  if [ "$(uname -s)" = "Darwin" ] || [ "$(uname -s)" = "Linux" ]; then
    if [ "${EUID:-$(id -u)}" -ne 0 ]; then
      warn "命令 $subcmd 需要管理员权限，将使用 sudo 启动已编译的二进制。"
      exec sudo "$BINARY_PATH" "$subcmd" "$@"
    fi
  fi

  exec "$BINARY_PATH" "$subcmd" "$@"
}

clean_artifacts() {
  info "清理本地开发产物..."
  rm -rf "$BUILD_DIR" "$CACHE_DIR"
}

cmd="${1:-help}"
shift || true

case "$cmd" in
  build)
    build_binary
    ;;
  test)
    run_tests
    ;;
  test-core)
    run_core_tests
    ;;
  check)
    build_binary
    run_core_tests
    ;;
  run)
    if [ "${1:-}" = "--" ]; then
      shift
    fi
    run_binary "$@"
    ;;
  start|stop)
    run_with_sudo_if_needed "$cmd" "$@"
    ;;
  console)
    # v2: 直接运行二进制（不带子命令）即进入菜单。
    build_binary
    if [ "$(uname -s)" = "Darwin" ] || [ "$(uname -s)" = "Linux" ]; then
      if [ "${EUID:-$(id -u)}" -ne 0 ]; then
        warn "进入菜单需要管理员权限，将使用 sudo。"
        exec sudo "$BINARY_PATH"
      fi
    fi
    exec "$BINARY_PATH"
    ;;
  status)
    run_binary status "$@"
    ;;
  clean)
    clean_artifacts
    ;;
  help|-h|--help)
    usage
    ;;
  *)
    usage
    error "未知命令: $cmd"
    ;;
esac
