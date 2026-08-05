/**
 * PLMN 运营商数据 catalog — 从 plmn-index 仓库懒加载
 * 数据源: https://raw.githubusercontent.com/voorz/plmn-index/main/plmn/all.json
 * 缓存策略: localStorage 缓存 + 7 天 TTL，可手动清除
 */

const ALL_JSON_URL = 'https://raw.githubusercontent.com/voorz/plmn-index/main/plmn/all.json'
const ALL_JSON_MIRROR = 'https://cdn.jsdelivr.net/gh/voorz/plmn-index@main/plmn/all.json'
const CACHE_KEY = 'vohive.plmn-catalog'
const CACHE_VERSION_KEY = 'vohive.plmn-catalog.version'
const CACHE_TTL = 7 * 24 * 60 * 60 * 1000 // 7 days

export interface IconSubOperator {
  names: string[]
  icon: string
  scope: string
}

export interface IconCatalogEntry {
  mnc: string
  icon: string
  scope: string
  subs?: IconSubOperator[]
}

// PLMN all.json 的原始结构
interface PlmnEntry {
  mcc: string
  mnc: string
  operators: Array<{
    brand?: string
    icon?: string
    icon_scope?: string
    subs?: Array<{
      brand?: string
      names?: string[]
      icon?: string
      icon_scope?: string
    }>
  }>
}

let cachedCatalog: Record<string, IconCatalogEntry[]> | null = null
let loadingPromise: Promise<Record<string, IconCatalogEntry[]> | null> | null = null

/** 将 all.json 转换为 icon catalog 格式 */
function convertAllJson(data: Record<string, PlmnEntry>): Record<string, IconCatalogEntry[]> {
  const catalog: Record<string, IconCatalogEntry[]> = {}
  for (const entry of Object.values(data)) {
    const mcc = entry.mcc
    if (!mcc) continue
    // 找第一个有 icon 的 operator
    const op = entry.operators?.find(o => o.icon)
    if (!op) continue
    const item: IconCatalogEntry = {
      mnc: entry.mnc,
      icon: op.icon!,
      scope: op.icon_scope || mcc,
    }
    // 提取有 icon 的 subs
    const subs: IconSubOperator[] = []
    for (const sub of op.subs || []) {
      if (sub.names && sub.names.length > 0) {
        subs.push({
          names: sub.names,
          icon: sub.icon || op.icon!,
          scope: sub.icon_scope || op.icon_scope || mcc,
        })
      }
    }
    if (subs.length > 0) item.subs = subs
    if (!catalog[mcc]) catalog[mcc] = []
    // 去重：同 MNC 只保留第一个
    if (!catalog[mcc].some(e => e.mnc === item.mnc)) {
      catalog[mcc].push(item)
    }
  }
  return catalog
}

/** 从 localStorage 读取缓存 */
function readCache(): Record<string, IconCatalogEntry[]> | null {
  try {
    const ts = localStorage.getItem(CACHE_VERSION_KEY)
    if (!ts) return null
    const age = Date.now() - parseInt(ts, 10)
    if (age > CACHE_TTL) return null
    const raw = localStorage.getItem(CACHE_KEY)
    if (!raw) return null
    return JSON.parse(raw)
  } catch {
    return null
  }
}

/** 写入 localStorage 缓存 */
function writeCache(catalog: Record<string, IconCatalogEntry[]>) {
  try {
    localStorage.setItem(CACHE_KEY, JSON.stringify(catalog))
    localStorage.setItem(CACHE_VERSION_KEY, String(Date.now()))
  } catch {
    // localStorage 满了，跳过
  }
}

/**
 * 加载 PLMN catalog（懒加载，首次调用时 fetch，后续从缓存读）。
 * 返回 null 表示加载失败。
 */
export function loadPlmnCatalog(): Promise<Record<string, IconCatalogEntry[]> | null> {
  if (cachedCatalog) return Promise.resolve(cachedCatalog)
  if (loadingPromise) return loadingPromise

  // 先试 localStorage 缓存
  const cached = readCache()
  if (cached) {
    cachedCatalog = cached
    return Promise.resolve(cached)
  }

  loadingPromise = (async () => {
    // 镜像优先（jsdelivr CDN），直链兜底
    for (const url of [ALL_JSON_MIRROR, ALL_JSON_URL]) {
      try {
        const res = await fetch(url)
        if (!res.ok) continue
        const data = await res.json() as Record<string, PlmnEntry>
        const catalog = convertAllJson(data)
        cachedCatalog = catalog
        writeCache(catalog)
        return catalog
      } catch {
        continue
      }
    }
    return null
  })()

  loadingPromise.finally(() => {
    loadingPromise = null
  })

  return loadingPromise
}

/**
 * 同步获取已加载的 catalog。
 * 如果尚未加载，返回 null 并触发后台加载。
 */
export function getPlmnCatalog(): Record<string, IconCatalogEntry[]> | null {
  if (cachedCatalog) return cachedCatalog
  // 尝试从 localStorage 读
  const cached = readCache()
  if (cached) {
    cachedCatalog = cached
    return cached
  }
  // 触发后台加载
  loadPlmnCatalog()
  return null
}

/** 清除 PLMN catalog 缓存 */
export function clearPlmnCatalogCache() {
  localStorage.removeItem(CACHE_KEY)
  localStorage.removeItem(CACHE_VERSION_KEY)
  cachedCatalog = null
}

export type { IconCatalogEntry as PlmnCatalogEntry }
