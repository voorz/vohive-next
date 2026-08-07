/**
 * PLMN 运营商信息查询 — 从 plmn-index 仓库懒加载 all.json
 * 提供国家/代码/ISO 等信息查询，供详情页等组件使用
 * 数据源: https://raw.githubusercontent.com/voorz/plmn-index/main/plmn/all.json
 * 缓存策略: localStorage 缓存 + 7 天 TTL
 */

const ALL_JSON_URL = 'https://raw.githubusercontent.com/voorz/plmn-index/main/plmn/all.json'
const ALL_JSON_MIRROR = 'https://cdn.jsdelivr.net/gh/voorz/plmn-index@main/plmn/all.json'
const CACHE_KEY = 'vohive.plmn-info'
const CACHE_TS_KEY = 'vohive.plmn-info.ts'
const CACHE_TTL = 7 * 24 * 60 * 60 * 1000 // 7 days

export interface PlmnInfoEntry {
  country: {
    name: string
    iso: string
    code: string
    region: string
  }
  operators: Array<{
    brand?: string
    operator?: string
    status?: string
    type?: string
    bands?: string
  }>
}

let cachedData: Record<string, PlmnInfoEntry> | null = null
let loadingPromise: Promise<Record<string, PlmnInfoEntry> | null> | null = null

function readCache(): Record<string, PlmnInfoEntry> | null {
  try {
    const ts = localStorage.getItem(CACHE_TS_KEY)
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

function writeCache(data: Record<string, PlmnInfoEntry>) {
  try {
    localStorage.setItem(CACHE_KEY, JSON.stringify(data))
    localStorage.setItem(CACHE_TS_KEY, String(Date.now()))
  } catch {
    // localStorage full, skip
  }
}

/** 加载 all.json 并缓存（懒加载） */
export function loadPlmnInfo(): Promise<Record<string, PlmnInfoEntry> | null> {
  if (cachedData) return Promise.resolve(cachedData)
  if (loadingPromise) return loadingPromise

  const cached = readCache()
  if (cached) {
    cachedData = cached
    return Promise.resolve(cached)
  }

  loadingPromise = (async () => {
    for (const url of [ALL_JSON_URL, ALL_JSON_MIRROR]) {
      try {
        const res = await fetch(url)
        if (!res.ok) continue
        const data = await res.json() as Record<string, PlmnInfoEntry>
        cachedData = data
        writeCache(data)
        return data
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

/** 同步获取已加载的数据，未加载则返回 null 并触发后台加载 */
export function getPlmnInfoData(): Record<string, PlmnInfoEntry> | null {
  if (cachedData) return cachedData
  const cached = readCache()
  if (cached) {
    cachedData = cached
    return cached
  }
  loadPlmnInfo()
  return null
}

/** 查询单个 PLMN 的信息（同步，需先加载） */
export function getPlmnInfo(plmn: string): PlmnInfoEntry | null {
  const data = getPlmnInfoData()
  return data?.[plmn] || null
}
