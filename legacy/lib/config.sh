#!/bin/bash
# lan-proxy-gateway - 配置管理模块
# 模板渲染、配置生成、代理节点提取

# 从 Clash/mihomo 配置文件中提取 proxies 段落，保存为 proxy-provider 格式
extract_proxies() {
    local config_file="$1"
    local output_file="$2"

    if [[ ! -f "$config_file" ]]; then
        error "配置文件不存在: $config_file"
        return 1
    fi

    # 提取 proxies: 到下一个顶级 key 之间的内容
    awk '/^proxies:/{found=1} found && /^[a-zA-Z]/ && !/^proxies:/{found=0} found' \
        "$config_file" > "$output_file"

    # 检查是否成功提取到节点
    if [[ ! -s "$output_file" ]] || ! grep -q "name:" "$output_file"; then
        error "未能从配置文件中提取到代理节点"
        echo "  请确认文件包含 proxies: 段落"
        return 1
    fi

    local count
    count=$(grep -c "  - name:" "$output_file")
    success "已从配置文件中提取 ${count} 个代理节点"
    return 0
}

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
    mkdir -p "${DATA_DIR}/proxy_provider"

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
    local subscription_url="${SUBSCRIPTION_URL:-}"
    local subscription_name="${SUBSCRIPTION_NAME:-subscription}"
    local proxy_source="${PROXY_SOURCE:-url}"

    # 根据代理来源类型校验配置
    if [[ "$proxy_source" == "url" ]]; then
        if [[ -z "$subscription_url" ]]; then
            error "订阅链接未配置，请检查 .secret 文件"
            return 1
        fi
    elif [[ "$proxy_source" == "file" ]]; then
        local proxy_config_file="${PROXY_CONFIG_FILE:-}"
        if [[ -z "$proxy_config_file" ]]; then
            error "配置文件路径未配置，请检查 .secret 文件中的 PROXY_CONFIG_FILE"
            return 1
        fi
        # 提取代理节点到 proxy_provider 目录
        local provider_file="${DATA_DIR}/proxy_provider/${subscription_name}.yaml"
        extract_proxies "$proxy_config_file" "$provider_file" || return 1
    else
        error "未知的代理来源类型: $proxy_source（应为 url 或 file）"
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

    # 配置文件模式：将 proxy-providers 从 http 改为 file
    if [[ "$proxy_source" == "file" ]]; then
        sed -i '' \
            -e 's/^    type: http$/    type: file/' \
            -e '/^    url: "/d' \
            -e '/^    interval: 3600$/d' \
            "$output"
    fi

    if [[ $? -eq 0 ]]; then
        success "配置文件已生成: $output"
        return 0
    else
        error "配置文件生成失败"
        return 1
    fi
}
