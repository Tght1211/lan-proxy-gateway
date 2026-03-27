#!/usr/bin/env bash
set -euo pipefail

REPO="Tght1211/lan-proxy-gateway"
BINARY="gateway"
GITHUB_MIRROR="${GITHUB_MIRROR:-}"

MIRRORS=(
  "https://hub.gitmirror.com/"
  "https://mirror.ghproxy.com/"
  "https://github.moeyy.xyz/"
  "https://gh.ddlc.top/"
)

info()  { printf "\033[1;32m%s\033[0m\n" "$*"; }
warn()  { printf "\033[1;33m%s\033[0m\n" "$*"; }
error() { printf "\033[1;31m%s\033[0m\n" "$*" >&2; exit 1; }

# download with automatic mirror fallback
# usage: gh_download URL OUTPUT_FILE [--progress]
gh_download() {
  local url="$1" output="$2" show_progress="${3:-}"
  local curl_opts="-fSL --connect-timeout 10 --max-time 90"
  [ "$show_progress" = "--progress" ] && curl_opts="$curl_opts --progress-bar" || curl_opts="$curl_opts -s"

  if [ -n "$GITHUB_MIRROR" ]; then
    curl $curl_opts -o "$output" "${GITHUB_MIRROR}${url}" && return 0
    error "下载失败: ${GITHUB_MIRROR}${url}"
  fi

  if curl $curl_opts -o "$output" "$url" 2>/dev/null; then
    return 0
  fi

  warn "直连 GitHub 失败，尝试镜像加速..."
  for m in "${MIRRORS[@]}"; do
    info "尝试镜像: ${m}"
    if curl $curl_opts -o "$output" "${m}${url}" 2>/dev/null; then
      info "镜像下载成功"
      return 0
    fi
  done

  error "所有下载方式均失败。请手动设置: GITHUB_MIRROR=https://你的镜像/ bash install.sh"
}

OS="$(uname -s)"
case "$OS" in
  Darwin) OS="darwin" ;;
  Linux)  OS="linux" ;;
  *) error "不支持的系统: $OS (Windows 请使用 PowerShell 安装脚本)" ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) error "不支持的架构: $ARCH" ;;
esac

if [ "$OS" = "darwin" ]; then
  INSTALL_DIR="/usr/local/bin"
  mkdir -p "$INSTALL_DIR" 2>/dev/null || true
else
  if [ -d "/usr/local/bin" ] && ([ -w "/usr/local/bin" ] || command -v sudo &>/dev/null); then
    INSTALL_DIR="/usr/local/bin"
  else
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
  fi
fi

ASSET="${BINARY}-${OS}-${ARCH}"

info "检测到系统: ${OS}/${ARCH}"
info "安装目录: ${INSTALL_DIR}"
info "正在获取最新版本..."

API_TMPFILE=$(mktemp)
gh_download "https://api.github.com/repos/${REPO}/releases/latest" "$API_TMPFILE"
TAG=$(grep '"tag_name"' "$API_TMPFILE" | head -1 | cut -d'"' -f4)
[ -z "$TAG" ] && rm -f "$API_TMPFILE" && error "无法获取最新版本号"

if ! grep -q "\"name\": \"${ASSET}\"" "$API_TMPFILE"; then
  rm -f "$API_TMPFILE"
  error "最新版本缺少当前平台资产: ${ASSET}"
fi
rm -f "$API_TMPFILE"

info "最新版本: ${TAG}"

TMPFILE=$(mktemp)
trap 'rm -f "$TMPFILE"' EXIT

info "下载 ${ASSET}..."
gh_download "https://github.com/${REPO}/releases/download/${TAG}/${ASSET}" "$TMPFILE" --progress
chmod +x "$TMPFILE"

TARGET="${INSTALL_DIR}/${BINARY}"
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMPFILE" "$TARGET"
else
  info "需要 sudo 权限安装到 ${INSTALL_DIR}"
  sudo mv "$TMPFILE" "$TARGET"
fi

case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    warn "注意: ${INSTALL_DIR} 不在 PATH 中"
    warn "请将以下内容添加到 ~/.bashrc 或 ~/.zshrc:"
    warn "  export PATH=\"${INSTALL_DIR}:\$PATH\""
    ;;
esac

ACTUAL_PATH="$(command -v gateway || true)"
INSTALLED_VERSION="$($TARGET version 2>/dev/null || $TARGET --version 2>/dev/null || true)"

if [ -z "$INSTALLED_VERSION" ]; then
  error "安装后版本校验失败：无法执行 gateway version"
fi

info ""
info "安装成功! 🎉"
info "版本: ${INSTALLED_VERSION}"
[ -n "$ACTUAL_PATH" ] && info "当前命中: ${ACTUAL_PATH}"
info ""
info "安装后验证（必做）:"
info "  gateway version"
info "  gateway install"
info "  gateway status"
