#!/bin/bash
# lan-proxy-gateway - 配置管理模块
# 模板渲染、配置生成

# 渲染模板：将 template.yaml 中的 {{变量}} 替换为实际值，输出到 data/config.yaml
render_template() {
    local template="$TEMPLATE_FILE"
    local output="$CONFIG_FILE"

    if [[ ! -f "$template" ]]; then
        error "模板文件不存在: $template"
        return 1
    fi

    # 确保输出目录存在
    ensure_data_dir

    # 加载 .secret 配置
    load_secret

    # 自动检测变量
    source "${PROJECT_DIR}/lib/detect.sh"
    local iface ip

    iface="$(detect_interface)"
    ip="$(detect_ip "$iface")"

    # 设置默认值
    local mixed_port="${MIXED_PORT:-7890}"
    local redir_port="${REDIR_PORT:-7892}"
    local api_port="${API_PORT:-9090}"
    local api_secret="${API_SECRET:-}"
    local dns_listen_port="${DNS_LISTEN_PORT:-53}"
    local subscription_url="${SUBSCRIPTION_URL}"
    local subscription_name="${SUBSCRIPTION_NAME:-subscription}"

    if [[ -z "$subscription_url" ]]; then
        error "订阅链接未配置，请检查 .secret 文件"
        return 1
    fi

    # 执行模板替换
    sed \
        -e "s|{{MIXED_PORT}}|${mixed_port}|g" \
        -e "s|{{REDIR_PORT}}|${redir_port}|g" \
        -e "s|{{API_PORT}}|${api_port}|g" \
        -e "s|{{API_SECRET}}|${api_secret}|g" \
        -e "s|{{DNS_LISTEN_PORT}}|${dns_listen_port}|g" \
        -e "s|{{SUBSCRIPTION_URL}}|${subscription_url}|g" \
        -e "s|{{SUBSCRIPTION_NAME}}|${subscription_name}|g" \
        -e "s|{{LAN_INTERFACE}}|${iface}|g" \
        -e "s|{{LAN_IP}}|${ip}|g" \
        "$template" > "$output"

    if [[ $? -eq 0 ]]; then
        success "配置文件已生成: $output"
        return 0
    else
        error "配置文件生成失败"
        return 1
    fi
}
