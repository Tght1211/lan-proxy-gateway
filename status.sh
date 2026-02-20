#!/bin/bash
# lan-proxy-gateway - 状态面板
# 用法: ./status.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${SCRIPT_DIR}/lib/common.sh"
source "${SCRIPT_DIR}/lib/detect.sh"

show_logo

# 检测环境
iface="$(detect_interface)"
lan_ip="$(detect_ip "$iface")"
api_port="${API_PORT:-9090}"
api_url="http://127.0.0.1:${api_port}"

# ========== 运行状态 ==========
echo -e "${BOLD}运行状态${NC}"
separator

mihomo_pid="$(pgrep -x mihomo)"
if [[ -n "$mihomo_pid" ]]; then
    echo -e "  mihomo:    ${GREEN}运行中${NC} (PID: ${mihomo_pid})"
else
    echo -e "  mihomo:    ${RED}未运行${NC}"
    echo ""
    echo "  启动: sudo ./start.sh"
    exit 0
fi

# IP 转发
ip_forward="$(sysctl -n net.inet.ip.forwarding 2>/dev/null)"
if [[ "$ip_forward" == "1" ]]; then
    echo -e "  IP 转发:   ${GREEN}已开启${NC}"
else
    echo -e "  IP 转发:   ${RED}未开启${NC}"
fi

# TUN 接口
# pf 重定向
if pfctl -s info 2>/dev/null | grep -q "Status: Enabled"; then
    echo -e "  pf 重定向: ${GREEN}已启用${NC}"
else
    echo -e "  pf 重定向: ${RED}未启用${NC}"
fi

echo -e "  本机 IP:   ${lan_ip}"
echo -e "  网络接口:  ${iface}"

# ========== API 查询 ==========
echo ""
echo -e "${BOLD}代理信息${NC}"
separator

# 检查 API 是否可用
if ! curl -s --connect-timeout 2 "${api_url}" &>/dev/null; then
    warn "API 不可用 (${api_url})"
    echo ""
    exit 0
fi

# 获取版本
version_info="$(curl -s --connect-timeout 5 "${api_url}/version" 2>/dev/null)"
if [[ -n "$version_info" ]]; then
    version="$(echo "$version_info" | grep -o '"version":"[^"]*"' | cut -d'"' -f4)"
    echo -e "  版本:      ${version:-unknown}"
fi

# 获取当前代理组信息
proxies_info="$(curl -s --connect-timeout 5 "${api_url}/proxies/Proxy" 2>/dev/null)"
if [[ -n "$proxies_info" ]]; then
    current_node="$(echo "$proxies_info" | grep -o '"now":"[^"]*"' | cut -d'"' -f4)"
    echo -e "  当前节点:  ${CYAN}${current_node:-未知}${NC}"
fi

# 获取连接数
connections_info="$(curl -s --connect-timeout 5 "${api_url}/connections" 2>/dev/null)"
if [[ -n "$connections_info" ]]; then
    conn_count="$(echo "$connections_info" | grep -o '"connections":\[' | head -1)"
    if [[ -n "$conn_count" ]]; then
        # 简单计数连接数
        active="$(echo "$connections_info" | grep -o '"id":"' | wc -l | tr -d ' ')"
        echo -e "  活跃连接:  ${active}"
    fi

    upload="$(echo "$connections_info" | grep -o '"uploadTotal":[0-9]*' | cut -d: -f2)"
    download="$(echo "$connections_info" | grep -o '"downloadTotal":[0-9]*' | cut -d: -f2)"

    if [[ -n "$upload" && -n "$download" ]]; then
        # 转换为可读格式
        format_bytes() {
            local bytes=$1
            if [[ $bytes -ge 1073741824 ]]; then
                echo "$(echo "scale=2; $bytes/1073741824" | bc) GB"
            elif [[ $bytes -ge 1048576 ]]; then
                echo "$(echo "scale=2; $bytes/1048576" | bc) MB"
            elif [[ $bytes -ge 1024 ]]; then
                echo "$(echo "scale=2; $bytes/1024" | bc) KB"
            else
                echo "${bytes} B"
            fi
        }
        echo -e "  上传总量:  $(format_bytes "${upload:-0}")"
        echo -e "  下载总量:  $(format_bytes "${download:-0}")"
    fi
fi

# ========== 设备配置提示 ==========
echo ""
echo -e "${BOLD}设备配置${NC}"
separator
echo -e "  将设备网关和 DNS 设为: ${CYAN}${lan_ip}${NC}"
echo ""
echo -e "  ${DIM}API 面板: http://${lan_ip}:${api_port}/ui${NC}"
echo -e "  ${DIM}日志: tail -f ${LOG_FILE}${NC}"
echo ""
