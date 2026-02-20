#!/bin/bash
# lan-proxy-gateway - 一键安装向导
# 用法: bash install.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${SCRIPT_DIR}/lib/common.sh"
source "${SCRIPT_DIR}/lib/detect.sh"

show_logo
echo -e "${BOLD}欢迎使用 LAN Proxy Gateway 安装向导${NC}"
separator

# ========== Step 1: 系统检查 ==========
step "1/6" "检查系统环境..."

# 检查 macOS
if [[ "$(uname)" != "Darwin" ]]; then
    error "此工具仅支持 macOS"
    exit 1
fi

macos_version="$(sw_vers -productVersion)"
info "macOS 版本: ${macos_version}"

# 检查 Homebrew
if ! command -v brew &>/dev/null; then
    warn "未检测到 Homebrew"
    echo ""
    echo "  请先安装 Homebrew:"
    echo "  /bin/bash -c \"\$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\""
    echo ""
    exit 1
fi
success "Homebrew 已安装"

# ========== Step 2: 安装 mihomo ==========
step "2/6" "检查 mihomo..."

mihomo_bin="$(detect_mihomo)"
if [[ -z "$mihomo_bin" ]]; then
    info "正在通过 Homebrew 安装 mihomo..."
    brew install mihomo
    mihomo_bin="$(detect_mihomo)"
    if [[ -z "$mihomo_bin" ]]; then
        error "mihomo 安装失败"
        exit 1
    fi
fi
success "mihomo 已就绪: ${mihomo_bin}"

# ========== Step 3: 下载 GeoIP/GeoSite 数据 ==========
step "3/6" "下载 GeoIP/GeoSite 数据文件..."

ensure_data_dir

GEOIP_URL="https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/country.mmdb"
GEOSITE_URL="https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geosite.dat"
GEOIP_DAT_URL="https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip.dat"

download_file() {
    local url="$1"
    local dest="$2"
    local name="$(basename "$dest")"

    if [[ -f "$dest" ]]; then
        info "${name} 已存在，跳过下载"
        return 0
    fi

    info "下载 ${name}..."
    if curl -fsSL --connect-timeout 30 --retry 3 -o "$dest" "$url"; then
        success "${name} 下载完成"
    else
        warn "${name} 下载失败，将使用 CDN 镜像重试..."
        local mirror_url="${url/github.com/mirror.ghproxy.com\/https:\/\/github.com}"
        if curl -fsSL --connect-timeout 30 --retry 3 -o "$dest" "$mirror_url"; then
            success "${name} (CDN 镜像) 下载完成"
        else
            warn "${name} 下载失败，mihomo 首次启动时会自动下载"
        fi
    fi
}

download_file "$GEOIP_URL" "${DATA_DIR}/country.mmdb"
download_file "$GEOSITE_URL" "${DATA_DIR}/geosite.dat"
download_file "$GEOIP_DAT_URL" "${DATA_DIR}/geoip.dat"

# ========== Step 4: 配置订阅链接 ==========
step "4/6" "配置订阅链接..."

if [[ -f "$SECRET_FILE" ]]; then
    source "$SECRET_FILE"
    if [[ -n "$SUBSCRIPTION_URL" ]]; then
        info "已有订阅配置"
        echo "  当前订阅: ${SUBSCRIPTION_URL:0:40}..."
        echo ""
        read -p "是否重新配置？[y/N] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            info "保留现有配置"
        else
            unset SUBSCRIPTION_URL
        fi
    fi
fi

if [[ -z "$SUBSCRIPTION_URL" ]]; then
    echo ""
    echo -e "${BOLD}请输入你的代理订阅链接:${NC}"
    echo -e "${DIM}（通常是机场提供的 Clash/mihomo 订阅 URL）${NC}"
    echo ""
    read -p "> " sub_url

    if [[ -z "$sub_url" ]]; then
        error "订阅链接不能为空"
        exit 1
    fi

    echo ""
    read -p "给订阅起个名字 [subscription]: " sub_name
    sub_name="${sub_name:-subscription}"

    # 写入 .secret
    cat > "$SECRET_FILE" << EOF
# lan-proxy-gateway 敏感配置
# ！！！此文件包含隐私信息，绝对不要提交到 Git ！！！

# 代理订阅链接
SUBSCRIPTION_URL="${sub_url}"
SUBSCRIPTION_NAME="${sub_name}"

# --- 可选配置（取消注释并修改）---
# MIXED_PORT=7890
# REDIR_PORT=7892
# API_PORT=9090
# API_SECRET=your_secret_here
# DNS_LISTEN_PORT=53
EOF

    chmod 600 "$SECRET_FILE"
    success "订阅配置已保存到 .secret"
fi

# ========== Step 5: 网络检测 & 生成配置 ==========
step "5/6" "检测网络并生成配置..."

separator
print_detect_summary
separator

# 生成运行时配置
source "${SCRIPT_DIR}/lib/config.sh"
render_template

# ========== Step 6: 验证 ==========
step "6/6" "安装验证..."

# 检查关键文件
all_ok=true

check_file() {
    if [[ -f "$1" ]]; then
        success "$2"
    else
        error "$2 - 文件缺失: $1"
        all_ok=false
    fi
}

check_file "$mihomo_bin" "mihomo 可执行文件"
check_file "$CONFIG_FILE" "运行时配置文件"
check_file "$SECRET_FILE" "订阅配置文件"

echo ""
if $all_ok; then
    separator
    echo -e "${GREEN}${BOLD}安装完成！${NC}"
    separator
    echo ""
    echo "  启动网关:  sudo ./start.sh"
    echo "  停止网关:  sudo ./stop.sh"
    echo "  查看状态:  ./status.sh"
    echo ""
    echo -e "${DIM}启动后，将其他设备的网关和 DNS 设为本机 IP 即可${NC}"
else
    separator
    error "安装未完成，请检查上方错误信息"
    exit 1
fi
