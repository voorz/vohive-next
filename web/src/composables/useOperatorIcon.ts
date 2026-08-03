/**
 * 运营商图标管理
 * - 全量 catalog 映射（140 MCC，500+ 运营商）
 * - localStorage 缓存，不主动下载
 * - 用户手动触发下载
 */

import { ICON_CATALOG } from './icon-catalog'

const ICON_BASE = 'https://raw.githubusercontent.com/NekokoLPA/operator-icons/master/icons'
const MIRROR_BASE = 'https://cdn.jsdelivr.net/gh/NekokoLPA/operator-icons@master/icons'
const STORAGE_PREFIX = 'vohive.icon.'
const OVERRIDE_PREFIX = 'vohive.icon-override.'
const SETTINGS_KEY = 'vohive.personalization'

interface IconEntry {
  iconName: string
  iconScope: string
}

/** Normalize MNC: strip leading zeros for comparison */
function normalizeMnc(mnc: string): string {
  return mnc.replace(/^0+/, '') || '0'
}

export function getIconInfo(mcc: string, mnc: string): IconEntry | null {
  // 1. 检查用户覆盖
  const override = getIconOverride(mcc, mnc)
  if (override) return override
  // 2. 查 catalog
  const entries = ICON_CATALOG[mcc]
  if (!entries) return null
  const target = normalizeMnc(mnc)
  const found = entries.find(e => normalizeMnc(e.mnc) === target)
  if (!found) return null
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
  return ICON_CATALOG[mcc] || []
}

export function getIconUrl(mcc: string, mnc: string): string | null {
  const info = getIconInfo(mcc, mnc)
  if (!info) return null
  return `${ICON_BASE}/${info.iconScope}/${info.iconName}.png`
}

export function getCachedIcon(mcc: string, mnc: string): string | null {
  const key = `${STORAGE_PREFIX}${mcc}-${mnc}`
  return localStorage.getItem(key)
}

export async function downloadIcon(mcc: string, mnc: string): Promise<string | null> {
  const info = getIconInfo(mcc, mnc)
  if (!info) return null
  const path = `${info.iconScope}/${info.iconName}.png`
  const mirrorUrl = `${MIRROR_BASE}/${path}`
  const directUrl = `${ICON_BASE}/${path}`

  // 1. 镜像下载（最多重试 2 次）
  for (let i = 0; i < 2; i++) {
    const result = await tryFetchIcon(mirrorUrl)
    if (result) {
      cacheIcon(mcc, mnc, result)
      return result
    }
  }

  // 2. 直链兜底
  const result = await tryFetchIcon(directUrl)
  if (result) {
    cacheIcon(mcc, mnc, result)
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

function cacheIcon(mcc: string, mnc: string, base64: string) {
  const key = `${STORAGE_PREFIX}${mcc}-${mnc}`
  localStorage.setItem(key, base64)
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
}

function blobToBase64(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = reject
    reader.readAsDataURL(blob)
  })
}
