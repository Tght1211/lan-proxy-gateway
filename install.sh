#!/usr/bin/env bash
set -euo pipefail

REPO="Tght1211/lan-proxy-gateway"
BINARY="gateway"
INSTALL_DIR="/usr/local/bin"

info()  { printf "\033[1;32m%s\033[0m\n" "$*"; }
warn()  { printf "\033[1;33m%s\033[0m\n" "$*"; }
error() { printf "\033[1;31m%s\033[0m\n" "$*" >&2; exit 1; }

# --- detect OS ---
OS="$(uname -s)"
case "$OS" in
  Darwin)  OS="darwin" ;;
  Linux)   OS="linux" ;;
  MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
  *) error "不支持的系统: $OS" ;;
esac

# --- detect arch ---
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  arm64|aarch64)  ARCH="arm64" ;;
  *) error "不支持的架构: $ARCH" ;;
esac

SUFFIX=""
[ "$OS" = "windows" ] && SUFFIX=".exe"
ASSET="${BINARY}-${OS}-${ARCH}${SUFFIX}"

info "检测到系统: ${OS}/${ARCH}"
info "正在获取最新版本..."

# --- get latest release tag ---
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | head -1 | cut -d'"' -f4)

[ -z "$TAG" ] && error "无法获取最新版本号，请检查网络连接"

info "最新版本: ${TAG}"

# --- download ---
URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"
TMPFILE=$(mktemp)
trap 'rm -f "$TMPFILE"' EXIT

info "下载 ${ASSET}..."
curl -fSL --progress-bar -o "$TMPFILE" "$URL" \
  || error "下载失败: ${URL}"

chmod +x "$TMPFILE"

# --- install ---
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMPFILE" "${INSTALL_DIR}/${BINARY}${SUFFIX}"
else
  info "需要 sudo 权限安装到 ${INSTALL_DIR}"
  sudo mv "$TMPFILE" "${INSTALL_DIR}/${BINARY}${SUFFIX}"
fi

info "安装成功! 🎉"
info "版本: $(${BINARY} --version 2>/dev/null || echo "${TAG}")"
info ""
info "快速开始:"
info "  gateway install    # 安装向导"
info "  sudo gateway start # 启动网关"
info "  gateway status     # 查看状态"
