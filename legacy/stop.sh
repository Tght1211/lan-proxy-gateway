#!/bin/bash
# lan-proxy-gateway - 停止网关
# 用法: sudo ./stop.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${SCRIPT_DIR}/lib/common.sh"

show_logo

# 检查 root
check_root

echo -e "${BOLD}正在停止 LAN Proxy Gateway...${NC}"
echo ""

# ========== 停止 mihomo ==========
step "1/3" "停止 mihomo..."
if pgrep -x mihomo &>/dev/null; then
    killall mihomo 2>/dev/null
    sleep 2
    if pgrep -x mihomo &>/dev/null; then
        warn "正常停止失败，强制终止..."
        killall -9 mihomo 2>/dev/null
    fi
    success "mihomo 已停止"
else
    info "mihomo 未在运行"
fi

# ========== 清除 pf 规则 ==========
step "2/3" "清除 pf 规则..."
pfctl -d 2>/dev/null
success "pf 规则已清除"

# ========== 关闭 IP 转发 ==========
step "3/3" "关闭 IP 转发..."
sysctl -w net.inet.ip.forwarding=0 > /dev/null 2>&1
success "IP 转发已关闭"

echo ""
separator
echo -e "${GREEN}${BOLD}  LAN Proxy Gateway 已停止${NC}"
separator
echo ""
echo -e "  ${DIM}设备网络设置可恢复为自动获取 (DHCP)${NC}"
echo ""
