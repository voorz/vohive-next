#!/bin/bash
# check-deps.sh — CI 前手动排查跨仓库依赖链状态（Linux 版，对标 check-deps.ps1）
# 用法: ./scripts/check-deps.sh
#
# 检查项目:
#   1. 每个依赖库的本地 git 分支（必须 main）
#   2. 每个依赖库本地 = 远程（已 push）
#   3. 每个依赖库的远程最新 tag
#   4. vohive-next go.mod 引用的版本是否 = 远程最新 tag
#   5. vowifi-core go.mod 引用的 swu-go/sipgo 版本是否 = 远程最新 tag
set -u

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"
LIBS_ROOT="$REPO_ROOT/vohive-libs"
MAIN_REPO="$REPO_ROOT"

# 依赖库配置: 名称 => go.mod 中的 module path
declare -A LIBS=(
  ["swu-go"]="github.com/voorz/swu-go"
  ["sipgo"]="github.com/voorz/sipgo"
  ["vowifi-core"]="github.com/voorz/vowifi-core"
  ["quectel-qmi-go"]="github.com/voorz/quectel-qmi-go"
  ["wwan-go"]="github.com/voorz/wwan-go"
  ["ims-go"]="github.com/voorz/ims-go"
)

# vowifi-core 额外检查的子依赖
declare -A VOWIFI_SUB_DEPS=(
  ["swu-go"]="github.com/voorz/swu-go"
  ["sipgo"]="github.com/voorz/sipgo"
)

ALL_OK=true

get_gomod_version() {
  local gomod="$1" module="$2"
  [ -f "$gomod" ] || return
  grep -E "^\s*$module\s+" "$gomod" | awk '{print $2}' | head -1
}

get_latest_remote_tag() {
  local repo="$1"
  git -C "$repo" ls-remote --tags origin 2>/dev/null \
    | grep -oP 'refs/tags/\Kv[\d.]+$' \
    | sort -V | tail -1
}

get_branch() { git -C "$1" rev-parse --abbrev-ref HEAD 2>/dev/null; }
is_clean() { [ -z "$(git -C "$1" status --porcelain 2>/dev/null)" ]; }
is_pushed() {
  local local_rev remote_rev
  local_rev=$(git -C "$1" rev-parse HEAD 2>/dev/null)
  remote_rev=$(git -C "$1" rev-parse origin/main 2>/dev/null)
  [ -n "$local_rev" ] && [ "$local_rev" = "$remote_rev" ]
}

norm_ver() {
  local v="$1"
  [[ "$v" == v* ]] && echo "$v" || echo "v$v"
}

echo ""
echo "========================================"
echo " VoHive 跨仓库依赖链 CI 前检查"
echo "========================================"
echo ""

printf "%-16s %-8s %-6s %-7s %-14s %-14s %-8s\n" "Lib" "Branch" "Clean" "Pushed" "GoModVer" "RemoteTag" "Match"
echo "--------------------------------------------------------------------------------"

for lib in "${!LIBS[@]}"; do
  lib_path="$LIBS_ROOT/$lib"
  module="${LIBS[$lib]}"
  if [ ! -d "$lib_path/.git" ]; then
    printf "%-16s %-8s %-6s %-7s %-14s %-14s %-8s\n" "$lib" "N/A" "N/A" "N/A" "N/A" "N/A" "N/A"
    echo "[WARN] $lib: 仓库目录不存在" >&2
    ALL_OK=false
    continue
  fi
  branch=$(get_branch "$lib_path")
  clean="Yes"; is_clean "$lib_path" || { clean="No"; ALL_OK=false; }
  pushed="Yes"; is_pushed "$lib_path" || { pushed="No"; ALL_OK=false; }
  remote_tag=$(get_latest_remote_tag "$lib_path")
  gomod_ver=$(get_gomod_version "$MAIN_REPO/go.mod" "$module")
  match="N/A"
  if [ -n "$gomod_ver" ] && [ -n "$remote_tag" ]; then
    if [ "$(norm_ver "$gomod_ver")" = "$remote_tag" ]; then match="OK";
    else match="MISMATCH"; ALL_OK=false; fi
  fi
  [ "$branch" != "main" ] && ALL_OK=false
  printf "%-16s %-8s %-6s %-7s %-14s %-14s %-8s\n" \
    "$lib" "$branch" "$clean" "$pushed" "${gomod_ver:--}" "${remote_tag:--}" "$match"
done

echo ""
echo "--- vowifi-core 子依赖 ---"
for sub in "${!VOWIFI_SUB_DEPS[@]}"; do
  module="${VOWIFI_SUB_DEPS[$sub]}"
  sub_path="$LIBS_ROOT/$sub"
  vowifi_ver=$(get_gomod_version "$LIBS_ROOT/vowifi-core/go.mod" "$module")
  sub_tag=""
  [ -d "$sub_path/.git" ] && sub_tag=$(get_latest_remote_tag "$sub_path")
  match="N/A"
  if [ -n "$vowifi_ver" ] && [ -n "$sub_tag" ]; then
    if [ "$(norm_ver "$vowifi_ver")" = "$sub_tag" ]; then match="OK";
    else match="MISMATCH"; ALL_OK=false; fi
  fi
  printf "  %-12s go.mod=%-14s remote=%-14s %s\n" "$sub" "${vowifi_ver:--}" "${sub_tag:--}" "$match"
done

echo ""
echo "--- vohive-next 主项目 ---"
main_branch=$(get_branch "$MAIN_REPO")
main_clean="Clean"; is_clean "$MAIN_REPO" || { main_clean="Dirty"; ALL_OK=false; }
main_pushed="Yes"
if git -C "$MAIN_REPO" rev-parse --verify origin/main >/dev/null 2>&1; then
  is_pushed "$MAIN_REPO" || { main_pushed="No"; ALL_OK=false; }
else
  main_pushed="跳过（无 origin）"
fi
main_version="?"
[ -f "$MAIN_REPO/VERSION" ] && main_version=$(tr -d '[:space:]' < "$MAIN_REPO/VERSION")
echo "  Branch:      $main_branch"
echo "  WorkingTree: $main_clean"
echo "  Pushed:      $main_pushed"
echo "  VERSION:     $main_version"
[ "$main_branch" != "main" ] && ALL_OK=false

echo ""
echo "========================================"
if $ALL_OK; then echo " ALL CHECKS PASSED — CI ready";
else echo " CHECKS FAILED — 请修复上述问题后再触发 CI"; exit 1; fi
echo "========================================"
