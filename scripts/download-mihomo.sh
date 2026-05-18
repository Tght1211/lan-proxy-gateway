#!/usr/bin/env bash
set -euo pipefail

VERSION="v1.19.8"
ARCH="linux-amd64"  # 默认 amd64，其他架构请自行修改
URL="https://raw.githubusercontent.com/MetaCubeX/mihomo-releases/${VERSION}/mihomo-${ARCH}"
TMPFILE=$(mktemp)

echo "正在下载 mihomo ${VERSION} (${ARCH})..."
curl -fsSL -o "$TMPFILE" "$URL" || {
  echo "失败，尝试镜像..."
  curl -fsSL -o "$TMPFILE" "https://raw.gitmirror.com/MetaCubeX/mihomo-releases/${VERSION}/mihomo-${ARCH}" || {
    echo "还是失败，请手动下载:"
    echo "https://github.com/MetaCubeX/mihomo/releases"
    exit 1
  }
}

echo "下载成功，安装中..."
sudo mv "$TMPFILE" /usr/local/bin/mihomo
chmod +x /usr/local/bin/mihomo

echo "mihomo 已安装到 /usr/local/bin"
echo "运行 gateway install 继续"