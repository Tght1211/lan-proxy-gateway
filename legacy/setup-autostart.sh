#!/bin/bash
# 一键配置开机自启动

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PLIST_FILE="com.lan-proxy-gateway.plist"
LAUNCH_DAEMON_PATH="/Library/LaunchDaemons/${PLIST_FILE}"

echo "================================"
echo "配置 LAN Proxy Gateway 开机自启动"
echo "================================"
echo ""

# 检查是否有 sudo 权限
if [[ $EUID -ne 0 ]]; then
   echo "需要使用 sudo 权限运行此脚本"
   echo "请运行: sudo ./setup-autostart.sh"
   exit 1
fi

# 检查 .secret 文件是否存在
if [[ ! -f "${SCRIPT_DIR}/.secret" ]]; then
    echo "错误：未找到 .secret 文件"
    echo "请先运行 bash install.sh 配置订阅链接"
    exit 1
fi

# 复制 plist 文件到 LaunchDaemons
echo "1. 安装 LaunchDaemon..."
cp "${SCRIPT_DIR}/${PLIST_FILE}" "${LAUNCH_DAEMON_PATH}"
chmod 644 "${LAUNCH_DAEMON_PATH}"

# 加载服务
echo "2. 加载服务..."
launchctl load -w "${LAUNCH_DAEMON_PATH}" 2>/dev/null || \
    launchctl bootstrap system "${LAUNCH_DAEMON_PATH}"

echo ""
echo "✓ 开机自启动配置完成！"
echo ""
echo "服务管理命令："
echo "  启动服务:  sudo launchctl kickstart -k system/com.lan-proxy-gateway"
echo "  停止服务:  sudo launchctl bootout system/com.lan-proxy-gateway"
echo "  查看状态:  sudo launchctl list | grep lan-proxy-gateway"
echo "  查看日志:  tail -f ${SCRIPT_DIR}/logs/service.log"
echo ""
