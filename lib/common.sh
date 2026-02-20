#!/bin/bash
# lan-proxy-gateway - 通用工具函数库
# 颜色输出、日志、Logo

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m' # No Color

# 项目根目录（相对于调用脚本）
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SECRET_FILE="${PROJECT_DIR}/.secret"
DATA_DIR="${PROJECT_DIR}/data"
TEMPLATE_FILE="${PROJECT_DIR}/config/template.yaml"
CONFIG_FILE="${DATA_DIR}/config.yaml"
LOG_FILE="/tmp/lan-proxy-gateway.log"
MIHOMO_API="http://127.0.0.1:9090"

# Logo
show_logo() {
    echo -e "${CYAN}"
    echo '  _                  ____                      '
    echo ' | |    __ _ _ __   |  _ \ _ __ _____  ___   _ '
    echo ' | |   / _` | `_ \  | |_) | `__/ _ \ \/ / | | |'
    echo ' | |__| (_| | | | | |  __/| | | (_) >  <| |_| |'
    echo ' |_____\__,_|_| |_| |_|   |_|  \___/_/\_\\__, |'
    echo '                    Gateway                |___/ '
    echo -e "${NC}"
}

# 日志函数
info() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

error() {
    echo -e "${RED}[ERROR]${NC} $*"
}

success() {
    echo -e "${GREEN}[${BOLD}OK${NC}${GREEN}]${NC} $*"
}

step() {
    echo -e "${BLUE}[${1}]${NC} ${2}"
}

# 分隔线
separator() {
    echo -e "${DIM}─────────────────────────────────────────${NC}"
}

# 检查是否 root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "此操作需要 root 权限，请使用 sudo 运行"
        exit 1
    fi
}

# 加载 .secret 文件
load_secret() {
    if [[ ! -f "$SECRET_FILE" ]]; then
        error "未找到 .secret 配置文件"
        echo "  请先运行 install.sh 或手动创建 .secret 文件"
        echo "  参考 .secret.example"
        exit 1
    fi
    source "$SECRET_FILE"
}

# 确保 data 目录存在
ensure_data_dir() {
    mkdir -p "$DATA_DIR"
}
