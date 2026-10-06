#!/bin/bash
# fake-dist.sh — 为后端构建创建伪前端产物（不进 git）
# 用法: ./scripts/fake-dist.sh
# 
# 背景：internal/web/fs.go 用 //go:embed all:dist 嵌入前端产物，
# 沙箱/服务器上没有 npm 构建环境时，go build ./cmd/... 会失败。
# 此脚本创建一个最小占位 dist，让后端完整构建能跑通。
# 真实部署时审计伙伴会在本地 npm build 生成真正的 dist。

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"
DIST_DIR="$REPO_ROOT/internal/web/dist"

if [ -d "$DIST_DIR" ] && [ -n "$(ls -A "$DIST_DIR" 2>/dev/null)" ]; then
    echo "dist 已存在且非空，跳过"
    exit 0
fi

mkdir -p "$DIST_DIR"
cat > "$DIST_DIR/index.html" << 'EOF'
<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>vohive (placeholder)</title></head>
<body><h1>vohive 后端构建占位页</h1><p>真实前端请由审计伙伴 npm build 生成</p></body></html>
EOF
# embed 要求至少一个文件，再放个空 assets 避免某些逻辑报错
mkdir -p "$DIST_DIR/assets"
touch "$DIST_DIR/assets/.placeholder"

echo "已创建伪 dist: $DIST_DIR"
echo "注意：此目录已在 .gitignore，不会提交"
