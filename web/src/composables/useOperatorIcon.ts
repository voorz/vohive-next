/**
 * 运营商图标管理
 * - 数据源: plmn-index 仓库（懒加载 all.json + localStorage 缓存 catalog 元数据）
 * - 图标下载: IndexedDB 缓存（Base64 Data URL），容量无上限
 * - 覆盖: IndexedDB overrides store
 * - 匹配: 评分制（MNC 匹配 + 名称双向匹配 + 子运营商匹配）
 */

import { getPlmnCatalog, type IconCatalogEntry, type IconSubOperator } from './plmn-catalog'
import { getDB, idbGet, idbSet, idbDelete, idbClear, idbCount, STORE_ICONS, STORE_OVERRIDES } from './useIDB'

export const ICON_BASE = 'https://raw.githubusercontent.com/voorz/plmn-index/main/plmn/icons'
export const MIRROR_BASE = 'https://cdn.jsdelivr.net/gh/voorz/plmn-index@main/plmn/icons'

// localStorage 保留：settings + override 迁移用
const OLD_STORAGE_PREFIX = 'vohive.icon.'
const OLD_OVERRIDE_PREFIX = 'vohive.icon-override.'
const SETTINGS_KEY = 'vohive.personalization'

export interface IconEntry {
  iconName: string
  iconScope: string
}

/** Normalize MNC: strip leading zeros for comparison */
function normalizeMnc(mnc: string): string {
  return mnc.replace(/^0+/, '') || '0'
}

/** 生成 IndexedDB 图标缓存 key */
function iconCacheKey(mcc: string, mnc: string, iconName: string): string {
  const normalizedMnc = normalizeMnc(mnc)
  return `${mcc}-${normalizedMnc}-${iconName}`
}

/** 生成 IndexedDB override key */
function overrideKey(mcc: string, mnc: string, carrierKey?: string): string {
  if (carrierKey) return carrierKey
  return `${mcc}-${mnc}`
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
  // 1. 检查用户覆盖（IndexedDB 异步读取，但 getIconInfo 是同步的）
  //    覆盖数据量小，用内存缓存保持同步接口
  const override = getOverrideFromMemory(mcc, mnc, carrierKey)
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
// Override 数据量小（JSON），用内存缓存保持同步接口 + IndexedDB 持久化

const overrideCache = new Map<string, IconEntry | null>()
let overrideCacheLoaded = false

/** 从内存缓存读取 override（同步，供 getIconInfo 使用） */
function getOverrideFromMemory(mcc: string, mnc: string, carrierKey?: string): IconEntry | null {
  if (!overrideCacheLoaded) return null
  const key = overrideKey(mcc, mnc, carrierKey)
  return overrideCache.get(key) ?? null
}

/**
 * 异步加载所有 override 到内存缓存。
 * 在 App 启动或页面加载时调用一次。
 */
export async function loadOverridesFromIDB(): Promise<void> {
  try {
    const db = await getDB()
    const tx = db.transaction(STORE_OVERRIDES, 'readonly')
    const store = tx.objectStore(STORE_OVERRIDES)
    const req = store.getAll()
    const keysReq = store.getAllKeys()

    const [values, keys] = await Promise.all([
      new Promise<string[]>((resolve) => {
        req.onsuccess = () => resolve(req.result ?? [])
        req.onerror = () => resolve([])
      }),
      new Promise<IDBValidKey[]>((resolve) => {
        keysReq.onsuccess = () => resolve(keysReq.result ?? [])
        keysReq.onerror = () => resolve([])
      }),
    ])

    overrideCache.clear()
    for (let i = 0; i < keys.length; i++) {
      const key = String(keys[i])
      const value = values[i]
      if (value) {
        try {
          const parsed = JSON.parse(value)
          if (parsed.iconName && parsed.iconScope) {
            overrideCache.set(key, parsed)
          }
        } catch { /* ignore */ }
      }
    }
    overrideCacheLoaded = true
  } catch {
    // 如果 IndexedDB 不可用，尝试从旧 localStorage 迁移
    overrideCacheLoaded = true
  }

  // 迁移旧 localStorage override
  migrateOldOverrides()
}

/** 从旧 localStorage 迁移 override 数据到内存缓存 + IndexedDB */
function migrateOldOverrides() {
  let migrated = false
  for (let i = 0; i < localStorage.length; i++) {
    const key = localStorage.key(i)
    if (!key || !key.startsWith(OLD_OVERRIDE_PREFIX)) continue
    try {
      const raw = localStorage.getItem(key)
      if (!raw) continue
      const parsed = JSON.parse(raw)
      if (parsed.iconName && parsed.iconScope) {
        const idbKey = key.replace(OLD_OVERRIDE_PREFIX, '')
        overrideCache.set(idbKey, parsed)
        idbSet(STORE_OVERRIDES, idbKey, raw)
        localStorage.removeItem(key)
        migrated = true
      }
    } catch { /* ignore */ }
  }
  if (migrated) {
    // 标记已加载
    overrideCacheLoaded = true
  }
}

export async function getIconOverrideAsync(mcc: string, mnc: string, carrierKey?: string): Promise<IconEntry | null> {
  // 优先查 carrierKey 专属覆盖
  if (carrierKey) {
    const key = overrideKey(mcc, mnc, carrierKey)
    const cached = overrideCache.get(key)
    if (cached) return cached
    const raw = await idbGet(STORE_OVERRIDES, key)
    if (raw) {
      try {
        const parsed = JSON.parse(raw)
        if (parsed.iconName && parsed.iconScope) {
          overrideCache.set(key, parsed)
          return parsed
        }
      } catch { /* ignore */ }
    }
  }
  // 回退到 mcc-mnc 覆盖
  const key = overrideKey(mcc, mnc)
  const cached = overrideCache.get(key)
  if (cached) return cached
  const raw = await idbGet(STORE_OVERRIDES, key)
  if (raw) {
    try {
      const parsed = JSON.parse(raw)
      if (parsed.iconName && parsed.iconScope) {
        overrideCache.set(key, parsed)
        return parsed
      }
    } catch { /* ignore */ }
  }
  return null
}

export async function setIconOverride(mcc: string, mnc: string, iconName: string, iconScope: string, carrierKey?: string) {
  const key = overrideKey(mcc, mnc, carrierKey)
  const value = JSON.stringify({ iconName, iconScope })
  const entry: IconEntry = { iconName, iconScope }
  overrideCache.set(key, entry)
  await idbSet(STORE_OVERRIDES, key, value)
  // 清除旧图标缓存，下次显示时重新下载
  const cacheKey = iconCacheKey(mcc, mnc, iconName)
  await idbDelete(STORE_ICONS, cacheKey)
}

export async function clearIconOverride(mcc: string, mnc: string, carrierKey?: string) {
  const key = overrideKey(mcc, mnc, carrierKey)
  overrideCache.delete(key)
  await idbDelete(STORE_OVERRIDES, key)
  // 同时清除 mcc-mnc 级别的旧覆盖（迁移场景）
  if (carrierKey) {
    const oldKey = overrideKey(mcc, mnc)
    overrideCache.delete(oldKey)
    await idbDelete(STORE_OVERRIDES, oldKey)
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

/**
 * 从 IndexedDB 获取缓存的图标（异步）。
 * 返回 Base64 Data URL 或 null。
 */
export async function getCachedIcon(mcc: string, mnc: string, name?: string, carrierKey?: string): Promise<string | null> {
  const info = getIconInfo(mcc, mnc, name, carrierKey)
  if (!info) return null
  const key = iconCacheKey(mcc, mnc, info.iconName)
  const cached = await idbGet(STORE_ICONS, key)
  if (cached) return cached

  // 向后兼容：检查旧 localStorage 缓存并迁移到 IndexedDB
  const oldKey1 = `${OLD_STORAGE_PREFIX}${mcc}-${mnc}-${info.iconName}`
  const oldCached1 = localStorage.getItem(oldKey1)
  if (oldCached1) {
    await idbSet(STORE_ICONS, key, oldCached1)
    localStorage.removeItem(oldKey1)
    return oldCached1
  }
  // 兼容规范化 MNC 的旧 localStorage key
  const normalizedMnc = normalizeMnc(mnc)
  const oldKey2 = `${OLD_STORAGE_PREFIX}${mcc}-${normalizedMnc}-${info.iconName}`
  const oldCached2 = localStorage.getItem(oldKey2)
  if (oldCached2) {
    await idbSet(STORE_ICONS, key, oldCached2)
    localStorage.removeItem(oldKey2)
    return oldCached2
  }
  // 兼容最旧的无 iconName 格式
  const oldKey3 = `${OLD_STORAGE_PREFIX}${mcc}-${normalizedMnc}`
  const oldCached3 = localStorage.getItem(oldKey3)
  if (oldCached3) {
    await idbSet(STORE_ICONS, key, oldCached3)
    localStorage.removeItem(oldKey3)
    return oldCached3
  }

  return null
}

export async function downloadIcon(mcc: string, mnc: string, name?: string, carrierKey?: string): Promise<string | null> {
  const info = getIconInfo(mcc, mnc, name, carrierKey)
  if (!info) return null
  const path = `${info.iconScope}/${info.iconName}.png`
  const mirrorUrl = `${MIRROR_BASE}/${path}`
  const directUrl = `${ICON_BASE}/${path}`
  const cacheKey = iconCacheKey(mcc, mnc, info.iconName)

  // 1. 镜像下载（最多重试 2 次）
  for (let i = 0; i < 2; i++) {
    const result = await tryFetchIcon(mirrorUrl)
    if (result) {
      await idbSet(STORE_ICONS, cacheKey, result)
      return result
    }
  }

  // 2. 直链兜底
  const result = await tryFetchIcon(directUrl)
  if (result) {
    await idbSet(STORE_ICONS, cacheKey, result)
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

export async function hasIconCache(): Promise<boolean> {
  const count = await idbCount(STORE_ICONS)
  return count > 0
}

export async function clearAllIconCache() {
  await idbClear(STORE_ICONS)
  await idbClear(STORE_OVERRIDES)
  overrideCache.clear()
  overrideCacheLoaded = false
  // 同时清除 PLMN catalog 缓存
  localStorage.removeItem('vohive.plmn-catalog')
  localStorage.removeItem('vohive.plmn-catalog.version')
  // 清理旧 localStorage 图标缓存
  const keys: string[] = []
  for (let i = 0; i < localStorage.length; i++) {
    const key = localStorage.key(i)
    if (key && key.startsWith(OLD_STORAGE_PREFIX)) keys.push(key)
  }
  keys.forEach(k => localStorage.removeItem(k))
}

/** 获取图标缓存数量（运营商图标 + override） */
export async function getIconCacheCount(): Promise<{ icons: number; overrides: number }> {
  const [icons, overrides] = await Promise.all([
    idbCount(STORE_ICONS),
    idbCount(STORE_OVERRIDES),
  ])
  return { icons, overrides }
}

function blobToBase64(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = reject
    reader.readAsDataURL(blob)
  })
}
