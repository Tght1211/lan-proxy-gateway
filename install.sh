#!/usr/bin/env bash
set -euo pipefail

REPO="Tght1211/lan-proxy-gateway"
BINARY="gateway"
# 可通过环境变量指定镜像前缀，如 GITHUB_MIRROR=https://hub.gitmirror.com/
GITHUB_MIRROR="${GITHUB_MIRROR:-}"

info()  { printf "\033[1;32m%s\033[0m\n" "$*"; }
warn()  { printf "\033[1;33m%s\033[0m\n" "$*"; }
error() { printf "\033[1;31m%s\033[0m\n" "$*" >&2; exit 1; }

# try curl with timeout, return 0 on success
try_curl() {
  curl -fsSL --connect-timeout 5 -o /dev/null "$1" 2>/dev/null
}

# --- auto-detect mirror ---
detect_mirror() {
  if [ -n "$GITHUB_MIRROR" ]; then
    info "使用指定镜像: ${GITHUB_MIRROR}"
    return
  fi
  # test direct GitHub
  if try_curl "https://api.github.com"; then
    GITHUB_MIRROR=""
    return
  fi

  warn "直连 GitHub 超时，尝试镜像加速..."
  local mirrors=(
    "https://hub.gitmirror.com/"
    "https://mirror.ghproxy.com/"
    "https://github.moeyy.xyz/"
    "https://gh.ddlc.top/"
  )
  for m in "${mirrors[@]}"; do
    if try_curl "${m}https://api.github.com"; then
      GITHUB_MIRROR="$m"
      info "使用镜像: ${m}"
      return
    fi
  done
  error "无法连接 GitHub 或任何镜像站。请手动设置: GITHUB_MIRROR=https://你的镜像/ bash install.sh"
}

# prefix a URL with mirror if needed
mirror() {
  echo "${GITHUB_MIRROR}${1}"
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

detect_mirror

info "正在获取最新版本..."

# --- get latest release tag ---
API_URL=$(mirror "https://api.github.com/repos/${REPO}/releases/latest")
TAG=$(curl -fsSL "$API_URL" \
  | grep '"tag_name"' | head -1 | cut -d'"' -f4)

[ -z "$TAG" ] && error "无法获取最新版本号"

info "最新版本: ${TAG}"

# --- download ---
URL=$(mirror "https://github.com/${REPO}/releases/download/${TAG}/${ASSET}")
TMPFILE=$(mktemp)
trap 'rm -f "$TMPFILE"' EXIT

info "下载 ${ASSET}..."
curl -fSL --progress-bar -o "$TMPFILE" "$URL" \
  || error "下载失败: ${URL}"

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
info "版本: $(\"$TARGET\" --version 2>/dev/null || echo "${TAG}")"
info ""
info "快速开始:"
info "  gateway install    # 安装向导"
info "  sudo gateway start # 启动网关"
info "  gateway status     # 查看状态"
