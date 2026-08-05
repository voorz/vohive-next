/**
 * 运营商图标管理
 * - 数据源: plmn-index 仓库（懒加载 all.json + localStorage 缓存）
 * - 图标下载: localStorage 缓存，用户手动触发
 */

import { getPlmnCatalog } from './plmn-catalog'

const ICON_BASE = 'https://raw.githubusercontent.com/voorz/plmn-index/main/plmn/icons'
const MIRROR_BASE = 'https://cdn.jsdelivr.net/gh/voorz/plmn-index@main/plmn/icons'
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

export function getIconInfo(mcc: string, mnc: string, name?: string): IconEntry | null {
  // 1. 检查用户覆盖
  const override = getIconOverride(mcc, mnc)
  if (override) return override
  // 2. 查 catalog（从 plmn-index 懒加载）
  const catalog = getPlmnCatalog()
  if (!catalog) return null
  const entries = catalog[mcc]
  if (!entries) return null
  const target = normalizeMnc(mnc)
  const found = entries.find(e => normalizeMnc(e.mnc) === target)
  if (!found) return null
  // 3. 检查子运营商匹配（GID1 覆盖图标）
  if (name && found.subs) {
    const nameLower = name.toLowerCase()
    const sub = found.subs.find(s => s.names.some(n => nameLower.includes(n)))
    if (sub) {
      return { iconName: sub.icon, iconScope: sub.scope }
    }
  }
  // 4. 返回默认图标
  return { iconName: found.icon, iconScope: found.scope }
}

// ── 图标覆盖（用户手动选择）──

export function getIconOverride(mcc: string, mnc: string): IconEntry | null {
  const key = `${OVERRIDE_PREFIX}${mcc}-${mnc}`
  try {
    const raw = localStorage.getItem(key)
    if (raw) {
      const parsed = JSON.parse(raw)
      if (parsed.iconName && parsed.iconScope) return parsed
    }
  } catch { /* ignore */ }
  return null
}

export function setIconOverride(mcc: string, mnc: string, iconName: string, iconScope: string) {
  const key = `${OVERRIDE_PREFIX}${mcc}-${mnc}`
  localStorage.setItem(key, JSON.stringify({ iconName, iconScope }))
  // 清除旧缓存，下次显示时重新下载
  const cacheKey = `${STORAGE_PREFIX}${mcc}-${mnc}`
  localStorage.removeItem(cacheKey)
}

export function clearIconOverride(mcc: string, mnc: string) {
  const key = `${OVERRIDE_PREFIX}${mcc}-${mnc}`
  localStorage.removeItem(key)
  const cacheKey = `${STORAGE_PREFIX}${mcc}-${mnc}`
  localStorage.removeItem(cacheKey)
}

/** 获取指定 MCC 下所有可选图标 */
export function getAvailableIcons(mcc: string): { icon: string; scope: string; mnc: string }[] {
  const catalog = getPlmnCatalog()
  if (!catalog) return []
  return catalog[mcc] || []
}

export function getIconUrl(mcc: string, mnc: string, name?: string): string | null {
  const info = getIconInfo(mcc, mnc, name)
  if (!info) return null
  return `${ICON_BASE}/${info.iconScope}/${info.iconName}.png`
}

export function getCachedIcon(mcc: string, mnc: string, name?: string): string | null {
  const info = getIconInfo(mcc, mnc, name)
  if (!info) return null
  // 用 iconName 区分同 PLMN 下不同子运营商的缓存
  const key = `${STORAGE_PREFIX}${mcc}-${mnc}-${info.iconName}`
  const cached = localStorage.getItem(key)
  if (cached) return cached
  // 向后兼容：检查旧缓存 key
  return localStorage.getItem(`${STORAGE_PREFIX}${mcc}-${mnc}`)
}

export async function downloadIcon(mcc: string, mnc: string, name?: string): Promise<string | null> {
  const info = getIconInfo(mcc, mnc, name)
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
