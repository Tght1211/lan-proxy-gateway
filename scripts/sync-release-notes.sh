#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

declare -A TITLES=(
  [v1.0.0]="v1.0.0 · 初始公开版"
  [v1.1.0]="v1.1.0 · 安装与稳定性增强"
  [v1.1.1]="v1.1.1 · 安装升级修复"
  [v1.2.0]="v1.2.0 · Chains 链式代理"
  [v1.3.0]="v1.3.0 · 代理模式扩展"
  [v2.0.0]="v2.0.0 · 扩展脚本架构"
  [v2.1.0]="v2.1.0 · TUN 控制命令"
  [v2.1.1]="v2.1.1 · TUN 启动修复"
  [v2.2.0]="v2.2.0 · 统一控制台与配置"
  [v2.2.1]="v2.2.1 · 控制台体验修复"
  [v2.2.2]="v2.2.2 · TUI 焦点与导航收口"
  [v2.2.3]="v2.2.3 · TUI 刷新反馈增强"
)

for tag in v1.0.0 v1.1.0 v1.1.1 v1.2.0 v1.3.0 v2.0.0 v2.1.0 v2.1.1 v2.2.0 v2.2.1 v2.2.2 v2.2.3; do
  notes_file="$ROOT_DIR/docs/releases/$tag.md"
  if [ "$tag" = "v2.2.3" ]; then
    notes_file="$ROOT_DIR/docs/releases/latest.md"
  fi
  gh release edit "$tag" \
    --title "${TITLES[$tag]}" \
    --notes-file "$notes_file"
done
