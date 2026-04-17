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
GITHUB_PROBE_MAX_TIME="${GITHUB_PROBE_MAX_TIME:-15}"
GITHUB_SPEED_LIMIT="${GITHUB_SPEED_LIMIT:-32768}"
GITHUB_SPEED_TIME="${GITHUB_SPEED_TIME:-20}"
GITHUB_CURL_HTTP1="${GITHUB_CURL_HTTP1:-1}"

MIRRORS=(
  "https://hub.gitmirror.com/"
  "https://mirror.ghproxy.com/"
  "https://github.moeyy.xyz/"
  "https://gh.ddlc.top/"
)

info()  { printf "\033[1;32m%s\033[0m\n" "$*"; }
warn()  { printf "\033[1;33m%s\033[0m\n" "$*"; }
error() { printf "\033[1;31m%s\033[0m\n" "$*" >&2; exit 1; }

HTTP_VERSION_ARGS=()
if [ "$GITHUB_CURL_HTTP1" = "1" ]; then
  HTTP_VERSION_ARGS+=(--http1.1)
fi

build_download_candidates() {
  local url="$1"
  if [ -n "$GITHUB_MIRROR" ]; then
    printf '%s\n' "${GITHUB_MIRROR}${url}"
    return 0
  fi

  printf '%s\n' "$url"
  local m
  for m in "${MIRRORS[@]}"; do
    printf '%s\n' "${m}${url}"
  done
}

probe_download_candidate() {
  local candidate="$1"
  local tmpfile
  tmpfile=$(mktemp)
  if curl -L "${HTTP_VERSION_ARGS[@]}" \
    --range 0-0 \
    --connect-timeout "$GITHUB_CONNECT_TIMEOUT" \
    --max-time "$GITHUB_PROBE_MAX_TIME" \
    -o "$tmpfile" \
    -s \
    "$candidate"; then
    rm -f "$tmpfile"
    return 0
  fi
  rm -f "$tmpfile"
  return 1
}

download_with_candidates() {
  local url="$1" output="$2" show_progress="${3:-}" max_time="${4:-$GITHUB_ASSET_MAX_TIME}"
  local progress_args=("-s")
  local candidate selected=""
  local -a candidates=()
  local -a curl_opts=(
    -fSL
    --connect-timeout "$GITHUB_CONNECT_TIMEOUT"
    --max-time "$max_time"
    --retry "$GITHUB_DOWNLOAD_RETRIES"
    --retry-delay 2
    --retry-all-errors
    --speed-limit "$GITHUB_SPEED_LIMIT"
    --speed-time "$GITHUB_SPEED_TIME"
  )

  if [ "$show_progress" = "--progress" ]; then
    progress_args=(--progress-bar)
  fi
  while IFS= read -r candidate; do
    candidates+=("$candidate")
  done < <(build_download_candidates "$url")

  for candidate in "${candidates[@]}"; do
    if probe_download_candidate "$candidate"; then
      selected="$candidate"
      break
    fi
  done

  if [ -z "$selected" ]; then
    error "没有找到可用下载源。你也可以手动指定: GITHUB_MIRROR=https://你的镜像/ bash install.sh"
  fi

  if [ "$selected" != "$url" ]; then
    warn "直连 GitHub 不稳定，切换到加速源: ${selected}"
  fi

  rm -f "$output"
  if curl "${curl_opts[@]}" "${HTTP_VERSION_ARGS[@]}" "${progress_args[@]}" -o "$output" "$selected"; then
    return 0
  fi

  for candidate in "${candidates[@]}"; do
    [ "$candidate" = "$selected" ] && continue
    if [ "$candidate" != "$url" ]; then
      info "尝试镜像: ${candidate}"
    fi
    rm -f "$output"
    if curl "${curl_opts[@]}" "${HTTP_VERSION_ARGS[@]}" "${progress_args[@]}" -o "$output" "$candidate"; then
      return 0
    fi
  done

  error "所有下载方式均失败。请稍后重试，或手动指定: GITHUB_MIRROR=https://你的镜像/ bash install.sh"
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

info "检测到系统: ${OS}/${ARCH}"
info "安装目录: ${INSTALL_DIR}"
info "正在获取最新版本..."

# --- get latest release tag ---
API_TMPFILE=$(mktemp)
download_with_candidates "https://api.github.com/repos/${REPO}/releases/latest" "$API_TMPFILE" "" "$GITHUB_API_MAX_TIME"
TAG=$(grep '"tag_name"' "$API_TMPFILE" | head -1 | cut -d'"' -f4)
rm -f "$API_TMPFILE"

[ -z "$TAG" ] && error "无法获取最新版本号"

info "最新版本: ${TAG}"

# --- 资产名：v3.0.0 起统一使用 gateway-<tag>-<os>-<arch>.tar.gz（含版本号 + 压缩包）---
ASSET="${BINARY}-${TAG}-${OS}-${ARCH}.tar.gz"

# --- download tarball ---
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT
TARBALL="$TMPDIR/$ASSET"

info "下载 ${ASSET}..."
download_with_candidates "https://github.com/${REPO}/releases/download/${TAG}/${ASSET}" "$TARBALL" --progress

info "解压..."
tar -C "$TMPDIR" -xzf "$TARBALL"
EXTRACTED="$TMPDIR/${BINARY}-${TAG}-${OS}-${ARCH}"
[ -f "$EXTRACTED" ] || error "解压后找不到二进制：$EXTRACTED"
chmod +x "$EXTRACTED"

# --- install ---
TARGET="${INSTALL_DIR}/${BINARY}"
if [ -w "$INSTALL_DIR" ]; then
  mv "$EXTRACTED" "$TARGET"
else
  info "需要 sudo 权限安装到 ${INSTALL_DIR}"
  sudo mv "$EXTRACTED" "$TARGET"
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
info "下一步:"
info "  sudo gateway install     # 下载 mihomo 内核 + 配置向导 + 自动启动"
info "  sudo gateway             # 以后进主菜单（状态 / 切换模式 / 换代理源）"
info "  gateway status           # 非交互查看状态"
info ""
info "开机自启:"
info "  sudo gateway service install    # launchd (mac) / systemd (linux)"
