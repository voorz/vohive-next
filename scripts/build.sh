#!/bin/bash
# build.sh — VoHive Linux 构建脚本（Linux 版，对标 build.ps1）
# 用法: ./scripts/build.sh [--deploy]
#   --deploy: 编译后部署到本机 /opt/vohive 并重启服务
#
# 流程：
#   1. swag init 重新生成 OpenAPI spec
#   2. 自动递增版本号（扫描 dist/ 中 v*.*.* 取最大 +1）
#   3. 从 go.mod 读取模块路径，ldflags 注入版本和构建时间
#   4. 编译 linux/amd64（-s -w -trimpath）+ UPX 压缩
#   5. 清理旧版本二进制
set -euo pipefail

DEPLOY=false
[[ "${1:-}" == "--deploy" ]] && DEPLOY=true

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"
DIST_DIR="$REPO_ROOT/dist"
DATE=$(date +%Y-%m-%d)

# --- 从 go.mod 读取模块路径 ---
MODULE_PATH=$(grep -E '^module\s+' "$REPO_ROOT/go.mod" | awk '{print $2}')

# --- 确定下一个版本号（x.x.xxx，范围 0.0.001 ~ 9.9.999）---
get_max_version() {
  ls "$DIST_DIR" 2>/dev/null | grep -oP 'vohive_v\K\d+\.\d+\.\d+(?=_linux_amd64$)' \
    | sort -t. -k1,1n -k2,2n -k3,3n | tail -1
}
MAX_VER=$(get_max_version || true)
if [ -n "$MAX_VER" ]; then
  IFS=. read -r MAJ MIN PAT <<< "$MAX_VER"
  PAT=$((PAT + 1))
  if [ "$PAT" -gt 999 ]; then PAT=0; MIN=$((MIN + 1)); fi
  if [ "$MIN" -gt 9 ]; then MIN=0; MAJ=$((MAJ + 1)); fi
  if [ "$MAJ" -gt 9 ]; then echo "ERROR: Version overflow (max 9.9.999)" >&2; exit 1; fi
else
  MAJ=0; MIN=0; PAT=1
fi
VERSION_STR="$MAJ.$MIN.$PAT"
VERSION="v$VERSION_STR"

# --- 同步 VERSION 文件 ---
echo -n "$VERSION_STR" > "$REPO_ROOT/VERSION"
echo "VERSION file updated: $VERSION_STR"

OUTPUT="$DIST_DIR/vohive_${VERSION}_linux_amd64"
mkdir -p "$DIST_DIR"

echo "=== VoHive Build (Linux) ==="
echo "Module:    $MODULE_PATH"
echo "Version:   $VERSION"
echo "BuildTime: $DATE"
echo "Output:    $OUTPUT"
echo ""

# --- 重新生成 OpenAPI spec ---
echo "Generating OpenAPI spec..."
pushd "$REPO_ROOT" >/dev/null
swag init -g cmd/vohive/main.go -o internal/api/docs --parseDependency --parseInternal
popd >/dev/null
echo "OpenAPI spec generated."

# --- 编译 ---
echo "Building..."
pushd "$REPO_ROOT" >/dev/null
export PATH="$HOME/go/go/bin:$PATH"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -tags "with_utls" \
  -ldflags "-s -w -X ${MODULE_PATH}/internal/global.Version=$VERSION -X ${MODULE_PATH}/internal/global.BuildTime=$DATE" \
  -o "$OUTPUT" ./cmd/vohive/
popd >/dev/null
SIZE_MB=$(du -m "$OUTPUT" | cut -f1)
echo "BUILD OK: $OUTPUT (${SIZE_MB} MB)"

# --- UPX 压缩 ---
if command -v upx >/dev/null 2>&1; then
  echo "Compressing with UPX..."
  if upx --best --lzma "$OUTPUT" 2>&1; then
    SIZE_AFTER=$(du -m "$OUTPUT" | cut -f1)
    echo "COMPRESSED: ${SIZE_MB} MB -> ${SIZE_AFTER} MB"
  else
    echo "UPX failed, continuing without compression."
  fi
else
  echo "UPX not found, skipping compression."
fi

# --- 清理旧版本 ---
echo "Cleaning old binaries (keeping $VERSION)..."
# 残文件（缺少 _linux_arch 后缀）
find "$DIST_DIR" -maxdepth 1 -name "vohive_v*" ! -name "*_linux_amd64" ! -name "*_linux_arm64" ! -name "*_linux_armv7" -type f -delete 2>/dev/null || true
# 旧 amd64
find "$DIST_DIR" -maxdepth 1 -name "vohive_v*_linux_amd64" ! -name "vohive_${VERSION}_linux_amd64" -type f -delete 2>/dev/null || true
# 旧 arm64
find "$DIST_DIR" -maxdepth 1 -name "vohive_v*_linux_arm64" ! -name "vohive_${VERSION}_linux_arm64" -type f -delete 2>/dev/null || true
echo "Old binaries cleaned."

# --- 部署 ---
if $DEPLOY; then
  echo ""
  echo "=== Deploying ==="
  sudo systemctl stop vohive && sleep 1
  sudo cp "$OUTPUT" /opt/vohive/bin/vohive && sudo chmod 755 /opt/vohive/bin/vohive
  sudo systemctl start vohive && sleep 5
  CODE=$(curl -s -o /dev/null -w '%{http_code}' http://localhost:7575/ping)
  echo "ping: $CODE"
  systemctl is-active vohive
  echo "DEPLOY OK — Frontend: http://localhost:7575 (admin/admin)"
fi
