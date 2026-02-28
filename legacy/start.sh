#!/bin/bash
# lan-proxy-gateway - 启动网关
# 用法: sudo ./start.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${SCRIPT_DIR}/lib/common.sh"
source "${SCRIPT_DIR}/lib/detect.sh"
source "${SCRIPT_DIR}/lib/config.sh"

show_logo

# 检查 root
check_root

step "1/5" "准备环境..."

# 加载配置
load_secret

# 检测环境
iface="$(detect_interface)"
lan_ip="$(detect_ip "$iface")"
mihomo_bin="$(detect_mihomo)"

if [[ -z "$mihomo_bin" ]]; then
    error "未找到 mihomo，请先运行 install.sh"
    exit 1
fi

# 停掉旧进程
if pgrep -x mihomo &>/dev/null; then
    warn "检测到 mihomo 正在运行，先停止..."
    killall mihomo 2>/dev/null
    sleep 2
fi

# ========== 生成/更新配置 ==========
step "2/5" "生成配置文件..."
render_template

# ========== 开启 IP 转发 ==========
step "3/5" "开启 IP 转发..."
sysctl -w net.inet.ip.forwarding=1 > /dev/null 2>&1
success "IP 转发已开启"

# 关掉可能干扰的 pf
pfctl -d 2>/dev/null

# ========== 启动 mihomo ==========
step "4/5" "启动 mihomo (TUN 模式)..."

"$mihomo_bin" -d "$DATA_DIR" > "$LOG_FILE" 2>&1 &
MIHOMO_PID=$!
sleep 5

if kill -0 "$MIHOMO_PID" 2>/dev/null; then
    success "mihomo 启动成功 (PID: ${MIHOMO_PID})"
else
    error "mihomo 启动失败！"
    echo ""
    echo "最后 20 行日志:"
    tail -20 "$LOG_FILE"
    exit 1
fi

# ========== 验证 TUN ==========
step "5/5" "验证 TUN 接口..."

TUN_IF="$(ifconfig | grep -B1 "inet 198.18" | head -1 | cut -d: -f1)"
if [[ -n "$TUN_IF" ]]; then
    success "TUN 接口已创建: ${TUN_IF}"
else
    warn "TUN 接口未检测到（可能还在创建中）"
    echo "  可通过 ifconfig | grep utun 手动检查"
fi

# ========== 显示信息 ==========
echo ""
separator
echo -e "${GREEN}${BOLD}  LAN Proxy Gateway 已启动！${NC}"
separator
echo ""
echo -e "  ${BOLD}本机 IP:${NC}     ${lan_ip}"
echo -e "  ${BOLD}网络接口:${NC}    ${iface}"
echo -e "  ${BOLD}API 面板:${NC}    http://${lan_ip}:${API_PORT:-9090}/ui"
echo ""
echo -e "  ${BOLD}其他设备网络设置:${NC}"
echo -e "  ┌───────────────────────────────┐"
echo -e "  │  网关 (Gateway):  ${CYAN}${lan_ip}${NC}"
echo -e "  │  DNS:             ${CYAN}${lan_ip}${NC}"
echo -e "  │  IP:              ${DIM}同网段任意可用 IP${NC}"
echo -e "  │  子网掩码:        ${DIM}255.255.255.0${NC}"
echo -e "  └───────────────────────────────┘"
echo ""
echo -e "  ${DIM}日志: tail -f ${LOG_FILE}${NC}"
echo -e "  ${DIM}停止: sudo ./stop.sh${NC}"
echo ""
