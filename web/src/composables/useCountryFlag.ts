/**
 * 国家旗帜图标管理
 * - 主动下载（与运营商图标不同）
 * - 镜像优先 2 次 → 直链兜底
 * - IndexedDB 缓存（配额大，适合存图片 base64）
 */

const FLAG_BASE = 'https://raw.githubusercontent.com/iebb/NekokoLPA2/master/assets/flags'
const FLAG_MIRROR = 'https://cdn.jsdelivr.net/gh/iebb/NekokoLPA2@master/assets/flags'
const DB_NAME = 'vohive'
const DB_STORE = 'flags'
const DB_VERSION = 1

// ── IndexedDB 初始化 ──

let dbPromise: Promise<IDBDatabase> | null = null

function getDB(): Promise<IDBDatabase> {
  if (dbPromise) return dbPromise
  dbPromise = new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, DB_VERSION)
    req.onupgradeneeded = () => {
      const db = req.result
      if (!db.objectStoreNames.contains(DB_STORE)) {
        db.createObjectStore(DB_STORE)
      }
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
  return dbPromise
}

async function idbGet(key: string): Promise<string | null> {
  try {
    const db = await getDB()
    return new Promise((resolve) => {
      const tx = db.transaction(DB_STORE, 'readonly')
      const req = tx.objectStore(DB_STORE).get(key)
      req.onsuccess = () => resolve(req.result ?? null)
      req.onerror = () => resolve(null)
    })
  } catch {
    return null
  }
}

async function idbSet(key: string, value: string): Promise<void> {
  try {
    const db = await getDB()
    await new Promise<void>((resolve) => {
      const tx = db.transaction(DB_STORE, 'readwrite')
      tx.objectStore(DB_STORE).put(value, key)
      tx.oncomplete = () => resolve()
      tx.onerror = () => resolve()
    })
  } catch {
    // 放弃缓存
  }
}

async function idbClear(): Promise<void> {
  try {
    const db = await getDB()
    await new Promise<void>((resolve) => {
      const tx = db.transaction(DB_STORE, 'readwrite')
      tx.objectStore(DB_STORE).clear()
      tx.oncomplete = () => resolve()
      tx.onerror = () => resolve()
    })
  } catch {
    // ignore
  }
}

// ── 旧 localStorage 缓存迁移清理 ──

function migrateOldLocalStorageFlags() {
  const OLD_PREFIX = 'vohive.flag.'
  const keys: string[] = []
  for (let i = 0; i < localStorage.length; i++) {
    const key = localStorage.key(i)
    if (key && key.startsWith(OLD_PREFIX)) keys.push(key)
  }
  keys.forEach(k => localStorage.removeItem(k))
}

// 启动时清理旧缓存
migrateOldLocalStorageFlags()

// ── 公共 API ──

export function getFlagUrl(iso: string): string {
  return `${FLAG_BASE}/${iso.toUpperCase()}.png`
}

export async function getCachedFlag(iso: string): Promise<string | null> {
  return await idbGet(iso.toUpperCase())
}

/** 主动下载国旗（自动调用，非用户触发） */
export async function downloadFlag(iso: string): Promise<string | null> {
  const code = iso.toUpperCase()
  const path = `${code}.png`
  const mirrorUrl = `${FLAG_MIRROR}/${path}`
  const directUrl = `${FLAG_BASE}/${path}`

  for (let i = 0; i < 2; i++) {
    const result = await tryFetch(mirrorUrl)
    if (result) {
      await idbSet(code, result)
      return result
    }
  }

  const result = await tryFetch(directUrl)
  if (result) {
    await idbSet(code, result)
    return result
  }

  return null
}

/** 获取国旗：有缓存用缓存，无缓存自动下载 */
export async function getOrDownloadFlag(iso: string): Promise<string | null> {
  const cached = await getCachedFlag(iso)
  if (cached) return cached
  return await downloadFlag(iso)
}

export async function clearAllFlagCache() {
  await idbClear()
}

// ── 内部工具 ──

async function tryFetch(url: string): Promise<string | null> {
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

function blobToBase64(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = reject
    reader.readAsDataURL(blob)
  })
}
