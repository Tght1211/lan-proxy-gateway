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
  [v2.2.4]="v2.2.4 · TUI 交互结构收口"
  [v2.2.5]="v2.2.5 · Windows 内核与升级链路修复"
  [v2.2.6]="v2.2.6 · 控制台工作台增强"
  [v2.2.7]="v2.2.7 · Windows 兼容性补齐与验收修复"
  [v2.2.8]="v2.2.8 · TUI Dashboard 改版"
  [v2.2.9]="v2.2.9 · 默认切回简单模式"
  [v2.2.10]="v2.2.10 · 安装下载链路修复"
)

for tag in v1.0.0 v1.1.0 v1.1.1 v1.2.0 v1.3.0 v2.0.0 v2.1.0 v2.1.1 v2.2.0 v2.2.1 v2.2.2 v2.2.3 v2.2.4 v2.2.5 v2.2.6 v2.2.7 v2.2.8 v2.2.9 v2.2.10; do
  notes_file="$ROOT_DIR/docs/releases/$tag.md"
  if [ ! -f "$notes_file" ]; then
    notes_file="$ROOT_DIR/docs/releases/latest.md"
  fi
  gh release edit "$tag" \
    --title "${TITLES[$tag]}" \
    --notes-file "$notes_file"
done
