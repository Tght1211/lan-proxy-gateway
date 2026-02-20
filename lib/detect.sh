#!/bin/bash
# lan-proxy-gateway - 自动检测模块
# 检测 CPU 架构、网络接口、IP 地址、mihomo 路径

# 检测 CPU 架构
detect_arch() {
    local arch
    arch="$(uname -m)"
    case "$arch" in
        arm64)
            echo "arm64"
            ;;
        x86_64)
            echo "amd64"
            ;;
        *)
            echo "unknown"
            ;;
    esac
}

# 检测 mihomo 可执行文件路径
detect_mihomo() {
    local arch mihomo_path

    # 优先检查 Homebrew 安装
    if [[ -x "/opt/homebrew/opt/mihomo/bin/mihomo" ]]; then
        echo "/opt/homebrew/opt/mihomo/bin/mihomo"
        return 0
    fi

    if [[ -x "/usr/local/opt/mihomo/bin/mihomo" ]]; then
        echo "/usr/local/opt/mihomo/bin/mihomo"
        return 0
    fi

    # 检查 PATH 中的 mihomo
    mihomo_path="$(command -v mihomo 2>/dev/null)"
    if [[ -n "$mihomo_path" ]]; then
        echo "$mihomo_path"
        return 0
    fi

    return 1
}

# 检测默认网络接口
detect_interface() {
    local iface

    # 方法1: 通过默认路由获取
    iface="$(route -n get default 2>/dev/null | awk '/interface:/ {print $2}')"
    if [[ -n "$iface" ]]; then
        echo "$iface"
        return 0
    fi

    # 方法2: 查找活跃的 en 接口
    for i in en0 en1 en2; do
        if ifconfig "$i" 2>/dev/null | grep -q "status: active"; then
            echo "$i"
            return 0
        fi
    done

    # 方法3: 默认 en0
    echo "en0"
}

# 检测本机局域网 IP
detect_ip() {
    local iface="${1:-$(detect_interface)}"
    local ip

    ip="$(ifconfig "$iface" 2>/dev/null | awk '/inet / && !/127.0.0.1/ {print $2}')"
    if [[ -n "$ip" ]]; then
        echo "$ip"
        return 0
    fi

    # 备选方案
    ip="$(ipconfig getifaddr "$iface" 2>/dev/null)"
    if [[ -n "$ip" ]]; then
        echo "$ip"
        return 0
    fi

    return 1
}

# 检测子网网关（路由器 IP）
detect_gateway() {
    route -n get default 2>/dev/null | awk '/gateway:/ {print $2}'
}

# 打印检测结果摘要
print_detect_summary() {
    local arch iface ip gw mihomo_bin

    arch="$(detect_arch)"
    iface="$(detect_interface)"
    ip="$(detect_ip "$iface")"
    gw="$(detect_gateway)"
    mihomo_bin="$(detect_mihomo)"

    echo -e "${BOLD}系统检测结果:${NC}"
    echo "  CPU 架构:    ${arch}"
    echo "  网络接口:    ${iface}"
    echo "  局域网 IP:   ${ip}"
    echo "  网关地址:    ${gw}"
    if [[ -n "$mihomo_bin" ]]; then
        echo -e "  mihomo:      ${GREEN}${mihomo_bin}${NC}"
    else
        echo -e "  mihomo:      ${RED}未安装${NC}"
    fi
}
