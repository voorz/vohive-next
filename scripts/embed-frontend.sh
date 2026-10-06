#!/bin/bash
# embed-frontend.sh — 前端构建 + embed 到 Go 二进制（Linux 版，对标 embed-frontend.ps1）
# 用法: ./scripts/embed-frontend.sh
#
# 流程：
#   1. 清理旧产物（web/dist + internal/web/dist）
#   2. 代码风格检查（npm run lint）
#   3. 类型检查（npm run typecheck）
#   4. 构建前端（vite build）
#   5. 复制新 dist 到 internal/web/dist/
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"
WEB_DIR="$REPO_ROOT/web"
DIST_DIR="$WEB_DIR/dist"
EMBED_DIR="$REPO_ROOT/internal/web/dist"

echo "=== VoHive Frontend Build + Embed ==="

echo "[1/5] Cleaning old build artifacts..."
rm -rf "$DIST_DIR" "$EMBED_DIR"
echo "  Cleaned."

echo "[2/5] Running lint (eslint)..."
npm run lint --prefix "$WEB_DIR"
echo "  Lint passed."

echo "[3/5] Running type check (vue-tsc)..."
npm run typecheck --prefix "$WEB_DIR"
echo "  Type check passed."

echo "[4/5] Building frontend (vite build)..."
pushd "$WEB_DIR" >/dev/null
npx vite build
popd >/dev/null
echo "  Build completed."

echo "[5/5] Copying dist to internal/web/dist/..."
mkdir -p "$EMBED_DIR"
cp -r "$DIST_DIR/"* "$EMBED_DIR/"
count=$(find "$EMBED_DIR" -type f | wc -l)
echo "  Copied $count files."

echo ""
echo "=== Frontend build + embed completed ==="
echo "Next: ./scripts/build.sh --deploy to compile and deploy."
