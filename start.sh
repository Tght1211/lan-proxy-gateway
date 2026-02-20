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

step "1/6" "准备环境..."

# 加载配置
load_secret

# 检测环境
iface="$(detect_interface)"
lan_ip="$(detect_ip "$iface")"
mihomo_bin="$(detect_mihomo)"
redir_port="${REDIR_PORT:-7892}"

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
step "2/6" "生成配置文件..."
render_template

# ========== 开启 IP 转发 ==========
step "3/6" "开启 IP 转发..."
sysctl -w net.inet.ip.forwarding=1 > /dev/null 2>&1
success "IP 转发已开启"

# ========== 启动 mihomo ==========
step "4/6" "启动 mihomo..."

"$mihomo_bin" -d "$DATA_DIR" > "$LOG_FILE" 2>&1 &
MIHOMO_PID=$!
sleep 3

if kill -0 "$MIHOMO_PID" 2>/dev/null; then
    success "mihomo 启动成功 (PID: ${MIHOMO_PID})"
else
    error "mihomo 启动失败！"
    echo ""
    echo "最后 20 行日志:"
    tail -20 "$LOG_FILE"
    exit 1
fi

# ========== 配置 pf 重定向 ==========
step "5/6" "配置 pf 流量重定向..."

PF_CONF="/tmp/lan-proxy-gateway-pf.conf"
cat > "$PF_CONF" << PFEOF
# lan-proxy-gateway pf rules
# 将 LAN 设备的 TCP 流量重定向到 mihomo redir-port

lan_if = "${iface}"
redir_port = "${redir_port}"

# 发往本机的流量不重定向（DNS:53、API:9090 等正常到达 mihomo）
no rdr on \$lan_if proto tcp to (\$lan_if)

# LAN 设备的 TCP 流量 → mihomo redir-port（内核态重定向，高性能）
rdr pass on \$lan_if proto tcp to any -> 127.0.0.1 port \$redir_port

# 屏蔽 QUIC（UDP 443），迫使 YouTube/Netflix 回退到 TCP 走代理
block in quick on \$lan_if proto udp to any port 443

pass all
PFEOF

pfctl -d 2>/dev/null
pfctl -f "$PF_CONF" 2>/dev/null
pfctl -e 2>/dev/null
success "pf 重定向已启用 (${iface} → redir-port:${redir_port})"

# ========== 验证 ==========
step "6/6" "验证服务..."

# 检查 mihomo 端口
if lsof -i ":${redir_port}" -sTCP:LISTEN &>/dev/null; then
    success "redir-port ${redir_port} 已监听"
else
    warn "redir-port ${redir_port} 未检测到（可能还在启动）"
fi

if lsof -i ":53" &>/dev/null; then
    success "DNS :53 已监听"
else
    warn "DNS :53 未检测到"
fi

# 检查 pf 状态
if pfctl -s info 2>/dev/null | grep -q "Status: Enabled"; then
    success "pf 防火墙已激活"
else
    warn "pf 未正确启用"
fi

# ========== 显示信息 ==========
echo ""
separator
echo -e "${GREEN}${BOLD}  LAN Proxy Gateway 已启动！${NC}"
separator
echo ""
echo -e "  ${BOLD}本机 IP:${NC}     ${lan_ip}"
echo -e "  ${BOLD}网络接口:${NC}    ${iface}"
echo -e "  ${BOLD}代理方式:${NC}    pf + redir-port（内核级重定向）"
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
