# embed-frontend.ps1 — VoHive 前端构建 + embed 产物更新脚本
# 用法: pwsh -NoProfile -File embed-frontend.ps1
#
# 自动 npm run build → 清理旧 embed assets → 复制新 dist 到 internal/web/dist/
# 仅处理前端，不涉及 Go 编译

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

$webDir = Join-Path $ScriptDir "web"
$webDist = Join-Path $webDir "dist"
$embedDist = Join-Path $ScriptDir "internal/web/dist"

Write-Host "=== VoHive Frontend Build & Embed ===" -ForegroundColor Cyan

# --- Lint 检查（构建前必检，避免 lint 错误堆积） ---
Write-Host "Running ESLint check..." -ForegroundColor Yellow
Push-Location $webDir
npm run lint 2>&1 | ForEach-Object {
    if ($_ -match 'error|warning|problem') { Write-Host $_ -ForegroundColor Red }
}
if ($LASTEXITCODE -ne 0) {
    Write-Host "LINT CHECK FAILED - fix errors before building" -ForegroundColor Red
    Pop-Location
    exit 1
}
Pop-Location
Write-Host "Lint check OK" -ForegroundColor Green

# --- 前端构建 ---
Write-Host "Building frontend (npm run build)..." -ForegroundColor Yellow
Push-Location $webDir
npm run build 2>&1 | ForEach-Object {
    if ($_ -match 'built in|error|Error') { Write-Host $_ }
}
if ($LASTEXITCODE -ne 0) {
    Write-Host "FRONTEND BUILD FAILED" -ForegroundColor Red
    Pop-Location
    exit 1
}
Pop-Location
Write-Host "Frontend build OK" -ForegroundColor Green

# --- 清理旧 embed assets ---
$oldAssets = Join-Path $embedDist "assets"
if (Test-Path $oldAssets) {
    Remove-Item $oldAssets -Recurse -Force
    Write-Host "Cleaned old embed assets/" -ForegroundColor DarkGray
}

# --- 复制新产物 ---
Copy-Item "$webDist/*" $embedDist -Recurse -Force
Write-Host "Copied to internal/web/dist/" -ForegroundColor Green

# --- 验证 ---
$indexHtml = Join-Path $embedDist "index.html"
if (Test-Path $indexHtml) {
    $jsHash = (Get-Content $indexHtml | Select-String 'index-[A-Za-z0-9_-]+\.js').Matches[0].Value
    Write-Host "Embed index.html references: $jsHash" -ForegroundColor Cyan
}

Write-Host "Done. Run build.ps1 -Deploy to compile & deploy." -ForegroundColor Green
