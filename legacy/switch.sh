#!/bin/bash
# lan-proxy-gateway - 快速切换代理来源
# 用法:
#   ./switch.sh              # 查看当前模式
#   ./switch.sh url          # 切换到订阅链接模式
#   ./switch.sh file         # 切换到配置文件模式
#   ./switch.sh file /path   # 切换到配置文件模式并更新路径

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${SCRIPT_DIR}/lib/common.sh"

if [[ ! -f "$SECRET_FILE" ]]; then
    error "未找到 .secret 文件，请先运行 install.sh"
    exit 1
fi

source "$SECRET_FILE"
current="${PROXY_SOURCE:-url}"

# 无参数：显示当前状态
if [[ $# -eq 0 ]]; then
    echo ""
    echo -e "  ${BOLD}当前模式:${NC} ${CYAN}${current}${NC}"
    if [[ "$current" == "url" ]]; then
        echo -e "  ${BOLD}订阅链接:${NC} ${SUBSCRIPTION_URL:0:50}..."
    else
        echo -e "  ${BOLD}配置文件:${NC} ${PROXY_CONFIG_FILE:-未设置}"
    fi
    echo ""
    echo -e "${DIM}用法: ./switch.sh [url|file] [配置文件路径]${NC}"
    exit 0
fi

target="$1"

if [[ "$target" != "url" && "$target" != "file" ]]; then
    error "参数应为 url 或 file"
    echo "  用法: ./switch.sh [url|file] [配置文件路径]"
    exit 1
fi

# 切换到 url 模式：检查 SUBSCRIPTION_URL 是否存在
if [[ "$target" == "url" && -z "$SUBSCRIPTION_URL" ]]; then
    error ".secret 中没有 SUBSCRIPTION_URL，请先配置订阅链接"
    exit 1
fi

# 切换到 file 模式
if [[ "$target" == "file" ]]; then
    # 如果提供了新路径，更新 PROXY_CONFIG_FILE
    if [[ -n "$2" ]]; then
        new_path="${2/#\~/$HOME}"
        if [[ ! -f "$new_path" ]]; then
            error "文件不存在: $new_path"
            exit 1
        fi
        if ! grep -q "^proxies:" "$new_path"; then
            error "文件中未找到 proxies: 段落"
            exit 1
        fi
        # 更新或插入 PROXY_CONFIG_FILE
        if grep -q '^PROXY_CONFIG_FILE=' "$SECRET_FILE"; then
            sed -i '' "s|^PROXY_CONFIG_FILE=.*|PROXY_CONFIG_FILE=\"${new_path}\"|" "$SECRET_FILE"
        else
            echo "PROXY_CONFIG_FILE=\"${new_path}\"" >> "$SECRET_FILE"
        fi
    fi
    # 再次检查
    source "$SECRET_FILE"
    if [[ -z "$PROXY_CONFIG_FILE" ]]; then
        error ".secret 中没有 PROXY_CONFIG_FILE"
        echo "  用法: ./switch.sh file /path/to/config.yaml"
        exit 1
    fi
fi

# 更新 PROXY_SOURCE
if grep -q '^PROXY_SOURCE=' "$SECRET_FILE"; then
    sed -i '' "s|^PROXY_SOURCE=.*|PROXY_SOURCE=\"${target}\"|" "$SECRET_FILE"
else
    # 插入到文件开头（注释之后）
    sed -i '' "/^# lan-proxy-gateway/a\\
PROXY_SOURCE=\"${target}\"
" "$SECRET_FILE"
fi

if [[ "$current" == "$target" ]]; then
    info "当前已是 ${target} 模式，无需切换"
else
    success "已切换: ${current} → ${target}"
fi

# 询问是否重新生成配置
echo ""
read -p "是否立即重新生成配置？[Y/n] " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Nn]$ ]]; then
    source "${SCRIPT_DIR}/lib/config.sh"
    render_template
    echo ""
    info "如需生效，请重启网关: sudo ./start.sh"
fi
