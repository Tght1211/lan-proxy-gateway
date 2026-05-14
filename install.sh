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
GITHUB_PROBE_MAX_TIME="${GITHUB_PROBE_MAX_TIME:-6}"
GITHUB_SPEED_LIMIT="${GITHUB_SPEED_LIMIT:-32768}"
GITHUB_SPEED_TIME="${GITHUB_SPEED_TIME:-20}"
GITHUB_CURL_HTTP1="${GITHUB_CURL_HTTP1:-1}"

# 镜像顺序按经验排：靠前的更稳定。如果某条被官方下线就移掉，加新的到末尾。
# 用户也可以用 GITHUB_MIRROR=https://你的镜像/ 覆盖整张表。
MIRRORS=(
  "https://hub.gitmirror.com/"
  "https://mirror.ghproxy.com/"
  "https://ghps.cc/"
  "https://gh-proxy.com/"
  "https://github.moeyy.xyz/"
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

short_source() {
  # 把候选 URL 截成一个能看的源标签：mirrors 拿出主机名，直连保留 github.com。
  local s="$1"
  printf '%s' "$s" | sed -E 's#^https?://([^/]+)/?.*#\1#'
}

download_with_candidates() {
  local url="$1" output="$2" show_progress="${3:-}" max_time="${4:-$GITHUB_ASSET_MAX_TIME}"
  local progress_args=("-s")
  local candidate selected=""
  local -a candidates=()
  local -a tried=()
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

  # 第一阶段：用 1-byte probe 快速挑出"看起来通"的候选。多数情况下能选出最快的一条。
  for candidate in "${candidates[@]}"; do
    if probe_download_candidate "$candidate"; then
      selected="$candidate"
      break
    fi
  done

  # 第二阶段：如果探测选出了候选就先用它。
  if [ -n "$selected" ]; then
    if [ "$selected" != "$url" ]; then
      warn "直连不稳定，先试加速源: $(short_source "$selected")"
    fi
    rm -f "$output"
    tried+=("$selected")
    if curl "${curl_opts[@]}" "${HTTP_VERSION_ARGS[@]}" "${progress_args[@]}" -o "$output" "$selected"; then
      return 0
    fi
    warn "下载失败：$(short_source "$selected")"
  fi

  # 第三阶段：兜底 —— 即使所有 probe 都失败，也按顺序把每个候选真的跑一遍。
  # 1-byte probe 失败不代表持续下载也失败（很多镜像 HEAD/short range 不友好，
  # 但实际 GET 是稳的）。让 curl 自己用 --connect-timeout / --max-time 决断。
  local already
  for candidate in "${candidates[@]}"; do
    already=0
    for t in "${tried[@]:-}"; do
      [ "$t" = "$candidate" ] && already=1 && break
    done
    [ "$already" = "1" ] && continue
    info "尝试: $(short_source "$candidate")"
    rm -f "$output"
    tried+=("$candidate")
    if curl "${curl_opts[@]}" "${HTTP_VERSION_ARGS[@]}" "${progress_args[@]}" -o "$output" "$candidate"; then
      return 0
    fi
  done

  warn "全部下载源都失败。尝试过："
  for t in "${tried[@]:-}"; do
    warn "  - $(short_source "$t")"
  done
  warn ""
  warn "可以试试："
  warn "  1) 等几分钟网络恢复后重试"
  warn "  2) 指定一个能用的镜像：  GITHUB_MIRROR=https://你的镜像/ bash install.sh"
  warn "  3) 走本机已经在跑的 Clash / Mihomo 端口： HTTP_PROXY=http://127.0.0.1:7897 HTTPS_PROXY=http://127.0.0.1:7897 bash install.sh"
  error "下载失败，安装中止。"
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

# --- 资产名：与 Makefile build-all 输出一致：gateway-<os>-<arch>.tar.gz ---
ASSET="${BINARY}-${OS}-${ARCH}.tar.gz"

# --- download tarball ---
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT
TARBALL="$TMPDIR/$ASSET"

info "下载 ${ASSET}..."
download_with_candidates "https://github.com/${REPO}/releases/download/${TAG}/${ASSET}" "$TARBALL" --progress

info "解压..."
tar -C "$TMPDIR" -xzf "$TARBALL"
EXTRACTED="$TMPDIR/${BINARY}-${OS}-${ARCH}"
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
info "✔ gateway 二进制已安装（版本 $("$TARGET" --version 2>/dev/null || echo "${TAG}")）"
info ""

# 只要当前 shell 有可用终端（即使 stdin 是 pipe，比如 curl|bash），就顺势进入配置向导。
# 用 < /dev/tty 把终端显式绑给 sudo/gateway install，保证它能读交互输入。
if [ -r /dev/tty ] && [ -w /dev/tty ]; then
  info "接下来进入配置向导（会请求 sudo 密码；向导里会问开机自启）"
  info ""
  exec sudo "$TARGET" install < /dev/tty
fi

# 非交互场景（CI / 无终端自动化）：只打印提示
info "下一步:"
info "  sudo gateway install     # 配置 + 启动 + 开机自启（一条龙）"
info "  sudo gateway             # 之后进主菜单（换源 / 切模式 / 看日志）"
info "  gateway status           # 非交互查看状态"
