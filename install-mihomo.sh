#!/usr/bin/env bash
set -euo pipefail

MIRROR="${MIRROR:-https://mirror.ghproxy.com/}"
VERSION="${VERSION:-v1.19.8}"

info()  { printf "\033[1;32m%s\033[0m\n" "$*"; }
warn()  { printf "\033[1;33m%s\033[0m\n" "$*"; }
error() { printf "\033[1;31m%s\033[0m\n" "$*" >&2; exit 1; }

# --- detect arch ---
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)  MIRROR_ARCH="linux-amd64" ;;
  arm64|aarch64) MIRROR_ARCH="linux-arm64" ;;
  *) error "不支持的架构: $ARCH" ;;
esac

info "正在下载 mihomo ${VERSION} (${MIRROR_ARCH})..."
info "使用镜像: ${MIRROR}"

# --- download mihomo ---
URL="${MIRROR}https://github.com/MetaCubeX/mihomo/releases/download/${VERSION}/mihomo-${MIRROR_ARCH}"
TMPFILE=$(mktemp)
trap 'rm -f "$TMPFILE"' EXIT

curl -fsSL --progress-bar -o "$TMPFILE" "$URL" || error "下载失败"

# --- install ---
if command -v sudo &>/dev/null && [ -w "/usr/local/bin" ]; then
  sudo mv "$TMPFILE" /usr/local/bin/mihomo
else
  warn "需要 sudo 权限或 /usr/local/bin 可写"
  sudo mv "$TMPFILE" /usr/local/bin/mihomo || {
    mkdir -p "$HOME/.local/bin"
    mv "$TMPFILE" "$HOME/.local/bin/mihomo"
    warn "已安装到 ~/.local/bin，请确保它在 PATH 中"
    warn "运行: export PATH=\"\$HOME/.local/bin:\$PATH\""
  }
fi

chmod +x /usr/local/bin/mihomo

info ""
info "mihomo 安装成功! 🎉"
info "运行 mihomo --version 检查:"
info "  mihomo --version"
info ""
info "现在可以运行 gateway install 了:"
info "  gateway install"
info ""