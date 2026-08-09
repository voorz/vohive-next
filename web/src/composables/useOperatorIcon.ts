/**
 * 运营商图标管理
 * - 数据源: plmn-index 仓库（懒加载 all.json + localStorage 缓存）
 * - 图标下载: localStorage 缓存，用户手动触发
 * - 匹配: 评分制（MNC 匹配 + 名称双向匹配 + 子运营商匹配）
 */

import { getPlmnCatalog, type IconCatalogEntry, type IconSubOperator } from './plmn-catalog'

export const ICON_BASE = 'https://raw.githubusercontent.com/voorz/plmn-index/main/plmn/icons'
export const MIRROR_BASE = 'https://cdn.jsdelivr.net/gh/voorz/plmn-index@main/plmn/icons'
const STORAGE_PREFIX = 'vohive.icon.'
const OVERRIDE_PREFIX = 'vohive.icon-override.'
const SETTINGS_KEY = 'vohive.personalization'

export interface IconEntry {
  iconName: string
  iconScope: string
}

/** Normalize MNC: strip leading zeros for comparison */
function normalizeMnc(mnc: string): string {
  return mnc.replace(/^0+/, '') || '0'
}

/** 双向名称匹配 — 参考 NekokoLPA2 的 _matchesName */
function matchesName(candidate: string, expected: string): boolean {
  const c = candidate.trim().toLowerCase()
  const e = expected.trim().toLowerCase()
  if (!c || !e) return false
  return c === e || c.includes(e) || e.includes(c)
}

/** 评分单个 catalog entry 与请求参数的匹配度 */
function scoreEntry(
  entry: IconCatalogEntry,
  nameLower: string | null,
): { score: number; icon: string; scope: string } | null {
  let score = 100 // MNC 匹配基础分

  // 检查子运营商匹配（优先级最高）
  if (nameLower && entry.subs) {
    for (const sub of entry.subs) {
      if (sub.names.some(n => matchesName(nameLower, n))) {
        return { score: score + 30, icon: sub.icon, scope: sub.scope }
      }
    }
  }

  // 检查运营商品牌名匹配
  if (nameLower && entry.brand && matchesName(nameLower, entry.brand)) {
    score += 20
  }

  return { score, icon: entry.icon, scope: entry.scope }
}

export function getIconInfo(mcc: string, mnc: string, name?: string, carrierKey?: string): IconEntry | null {
  // 1. 检查用户覆盖
  const override = getIconOverride(mcc, mnc, carrierKey)
  if (override) return override

  // 2. 查 catalog（从 plmn-index 懒加载）
  const catalog = getPlmnCatalog()
  if (!catalog) return null
  const entries = catalog[mcc]
  if (!entries) return null

  const target = normalizeMnc(mnc)
  const nameLower = name ? name.toLowerCase().trim() : null

  // 3. 评分所有 MNC 匹配的 entry，取最高分
  let bestScore = -1
  let bestIcon: IconEntry | null = null

  for (const entry of entries) {
    if (normalizeMnc(entry.mnc) !== target) continue
    const result = scoreEntry(entry, nameLower)
    if (result && result.score > bestScore) {
      bestScore = result.score
      bestIcon = { iconName: result.icon, iconScope: result.scope }
    }
  }

  return bestIcon
}

// ── 图标覆盖（用户手动选择）──

function getOverrideKey(mcc: string, mnc: string, carrierKey?: string): string {
  // 有 carrierKey 时区分子品牌（如 234-10__giffgaff vs 234-10）
  if (carrierKey) {
    return `${OVERRIDE_PREFIX}${carrierKey}`
  }
  return `${OVERRIDE_PREFIX}${mcc}-${mnc}`
}

export function getIconOverride(mcc: string, mnc: string, carrierKey?: string): IconEntry | null {
  // 优先查 carrierKey 专属覆盖
  if (carrierKey) {
    const key = getOverrideKey(mcc, mnc, carrierKey)
    try {
      const raw = localStorage.getItem(key)
      if (raw) {
        const parsed = JSON.parse(raw)
        if (parsed.iconName && parsed.iconScope) return parsed
      }
    } catch { /* ignore */ }
  }
  // 回退到 mcc-mnc 覆盖（兼容旧数据 + 设备页面无 carrierKey 的场景）
  const key = getOverrideKey(mcc, mnc)
  try {
    const raw = localStorage.getItem(key)
    if (raw) {
      const parsed = JSON.parse(raw)
      if (parsed.iconName && parsed.iconScope) return parsed
    }
  } catch { /* ignore */ }
  return null
}

export function setIconOverride(mcc: string, mnc: string, iconName: string, iconScope: string, carrierKey?: string) {
  const key = getOverrideKey(mcc, mnc, carrierKey)
  localStorage.setItem(key, JSON.stringify({ iconName, iconScope }))
  // 清除旧缓存，下次显示时重新下载
  const cacheKey = `${STORAGE_PREFIX}${mcc}-${mnc}-${iconName}`
  localStorage.removeItem(cacheKey)
}

export function clearIconOverride(mcc: string, mnc: string, carrierKey?: string) {
  const key = getOverrideKey(mcc, mnc, carrierKey)
  localStorage.removeItem(key)
  // 同时清除 mcc-mnc 级别的旧覆盖（迁移场景）
  if (carrierKey) {
    const oldKey = getOverrideKey(mcc, mnc)
    localStorage.removeItem(oldKey)
  }
}

/** 获取指定 MCC 下所有可选图标（含子运营商图标） */
export function getAvailableIcons(mcc: string): { icon: string; scope: string; mnc: string; label?: string }[] {
  const catalog = getPlmnCatalog()
  if (!catalog) return []
  const entries = catalog[mcc] || []
  const result: { icon: string; scope: string; mnc: string; label?: string }[] = []
  for (const entry of entries) {
    // 主运营商图标
    result.push({ icon: entry.icon, scope: entry.scope, mnc: entry.mnc, label: entry.brand })
    // 子运营商图标
    if (entry.subs) {
      for (const sub of entry.subs) {
        // 只添加与父级不同的子运营商图标
        if (sub.icon !== entry.icon || sub.scope !== entry.scope) {
          result.push({ icon: sub.icon, scope: sub.scope, mnc: entry.mnc, label: sub.names[0] })
        }
      }
    }
  }
  return result
}

export function getIconUrl(mcc: string, mnc: string, name?: string): string | null {
  const info = getIconInfo(mcc, mnc, name)
  if (!info) return null
  return `${ICON_BASE}/${info.iconScope}/${info.iconName}.png`
}

export function getCachedIcon(mcc: string, mnc: string, name?: string, carrierKey?: string): string | null {
  const info = getIconInfo(mcc, mnc, name, carrierKey)
  if (!info) return null
  // 用 iconName 区分同 PLMN 下不同子运营商的缓存
  const key = `${STORAGE_PREFIX}${mcc}-${mnc}-${info.iconName}`
  const cached = localStorage.getItem(key)
  if (cached) return cached
  // 向后兼容：检查旧缓存 key
  return localStorage.getItem(`${STORAGE_PREFIX}${mcc}-${mnc}`)
}

export async function downloadIcon(mcc: string, mnc: string, name?: string, carrierKey?: string): Promise<string | null> {
  const info = getIconInfo(mcc, mnc, name, carrierKey)
  if (!info) return null
  const path = `${info.iconScope}/${info.iconName}.png`
  const mirrorUrl = `${MIRROR_BASE}/${path}`
  const directUrl = `${ICON_BASE}/${path}`
  const cacheKey = `${STORAGE_PREFIX}${mcc}-${mnc}-${info.iconName}`

  // 1. 镜像下载（最多重试 2 次）
  for (let i = 0; i < 2; i++) {
    const result = await tryFetchIcon(mirrorUrl)
    if (result) {
      localStorage.setItem(cacheKey, result)
      return result
    }
  }

  // 2. 直链兜底
  const result = await tryFetchIcon(directUrl)
  if (result) {
    localStorage.setItem(cacheKey, result)
    return result
  }

  return null
}

async function tryFetchIcon(url: string): Promise<string | null> {
  try {
    const res = await fetch(url)
    if (!res.ok) return null
    const blob = await res.blob()
    if (blob.size === 0) return null
    return await blobToBase64(blob)
  } catch {
    return null
  }
}


export function isPersonalizationEnabled(): boolean {
  try {
    const raw = localStorage.getItem(SETTINGS_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      return parsed.use_custom_icons ?? true
    }
  } catch { /* default */ }
  return true
}

export function hasIconCache(): boolean {
  for (let i = 0; i < localStorage.length; i++) {
    const key = localStorage.key(i)
    if (key && key.startsWith(STORAGE_PREFIX)) return true
  }
  return false
}

export function clearAllIconCache() {
  const keys: string[] = []
  for (let i = 0; i < localStorage.length; i++) {
    const key = localStorage.key(i)
    if (key && key.startsWith(STORAGE_PREFIX)) keys.push(key)
  }
  keys.forEach(k => localStorage.removeItem(k))
  // 同时清除 PLMN catalog 缓存
  localStorage.removeItem('vohive.plmn-catalog')
  localStorage.removeItem('vohive.plmn-catalog.version')
}

function blobToBase64(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = reject
    reader.readAsDataURL(blob)
  })
}
