#!/usr/bin/env bash
set -euo pipefail

REPO="Tght1211/lan-proxy-gateway"
BINARY="gateway"
# 可通过环境变量指定镜像前缀，如 GITHUB_MIRROR=https://hub.gitmirror.com/
GITHUB_MIRROR="${GITHUB_MIRROR:-}"
GITHUB_API_MAX_TIME="${GITHUB_API_MAX_TIME:-30}"
GITHUB_ASSET_MAX_TIME="${GITHUB_ASSET_MAX_TIME:-600}"
GITHUB_CONNECT_TIMEOUT="${GITHUB_CONNECT_TIMEOUT:-10}"
GITHUB_DOWNLOAD_RETRIES="${GITHUB_DOWNLOAD_RETRIES:-3}"

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
# usage: gh_download URL OUTPUT_FILE [--progress] [MAX_TIME]
gh_download() {
  local url="$1" output="$2" show_progress="${3:-}" max_time="${4:-$GITHUB_ASSET_MAX_TIME}"
  local curl_opts="-fSL --connect-timeout ${GITHUB_CONNECT_TIMEOUT} --max-time ${max_time} --retry ${GITHUB_DOWNLOAD_RETRIES} --retry-delay 2 --retry-all-errors"
  [ "$show_progress" = "--progress" ] && curl_opts="$curl_opts --progress-bar" || curl_opts="$curl_opts -s"

  # if user specified a mirror, use it directly
  if [ -n "$GITHUB_MIRROR" ]; then
    curl $curl_opts -o "$output" "${GITHUB_MIRROR}${url}" && return 0
    error "下载失败: ${GITHUB_MIRROR}${url}"
  fi

  # try direct first
  if curl $curl_opts -o "$output" "$url" 2>/dev/null; then
    return 0
  fi

  # direct failed, try mirrors
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

# --- detect OS ---
OS="$(uname -s)"
case "$OS" in
  Darwin)  OS="darwin" ;;
  Linux)   OS="linux" ;;
  *) error "不支持的系统: $OS (Windows 请使用 PowerShell 安装脚本)" ;;
esac

# --- detect arch ---
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  arm64|aarch64)  ARCH="arm64" ;;
  *) error "不支持的架构: $ARCH" ;;
esac

# --- pick install dir ---
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

# --- get latest release tag ---
API_TMPFILE=$(mktemp)
gh_download "https://api.github.com/repos/${REPO}/releases/latest" "$API_TMPFILE" "" "$GITHUB_API_MAX_TIME"
TAG=$(grep '"tag_name"' "$API_TMPFILE" | head -1 | cut -d'"' -f4)
rm -f "$API_TMPFILE"

[ -z "$TAG" ] && error "无法获取最新版本号"

info "最新版本: ${TAG}"

# --- download binary ---
TMPFILE=$(mktemp)
trap 'rm -f "$TMPFILE"' EXIT

info "下载 ${ASSET}..."
gh_download "https://github.com/${REPO}/releases/download/${TAG}/${ASSET}" "$TMPFILE" --progress

chmod +x "$TMPFILE"

# --- install ---
TARGET="${INSTALL_DIR}/${BINARY}"
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMPFILE" "$TARGET"
else
  info "需要 sudo 权限安装到 ${INSTALL_DIR}"
  sudo mv "$TMPFILE" "$TARGET"
fi

# --- check PATH ---
case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    warn "注意: ${INSTALL_DIR} 不在 PATH 中"
    warn "请将以下内容添加到 ~/.bashrc 或 ~/.zshrc:"
    warn "  export PATH=\"${INSTALL_DIR}:\$PATH\""
    ;;
esac

info ""
info "安装成功! 🎉"
info "版本: $("$TARGET" --version 2>/dev/null || echo "${TAG}")"
info ""
info "快速开始:"
info "  gateway install             # 安装向导"
info "  gateway config              # 打开配置中心"
info "  sudo gateway start          # 启动网关并进入运行中控制台"
info "  gateway status              # 查看状态和出口网络"
info "  sudo gateway permission install  # 可选: 配置免密控制"
