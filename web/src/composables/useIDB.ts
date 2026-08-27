/**
 * 共享 IndexedDB 连接管理
 * 
 * DB: vohive (version 2)
 * Stores:
 *   - flags:     国家旗帜缓存（Base64 Data URL）
 *   - icons:     运营商图标缓存（Base64 Data URL）
 *   - overrides: 运营商图标用户覆盖（JSON: { iconName, iconScope }）
 * 
 * 所有 store 使用 key-value 模式（无 keyPath，外部指定 key）。
 */

const DB_NAME = 'vohive'
const DB_VERSION = 2

export const STORE_FLAGS = 'flags'
export const STORE_ICONS = 'icons'
export const STORE_OVERRIDES = 'overrides'

let dbPromise: Promise<IDBDatabase> | null = null

/**
 * 获取共享 DB 连接（单例）。
 * 首次打开时自动升级到 DB_VERSION，创建缺失的 store。
 */
export function getDB(): Promise<IDBDatabase> {
  if (dbPromise) return dbPromise
  dbPromise = new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, DB_VERSION)
    req.onupgradeneeded = (event) => {
      const db = req.result
      const oldVersion = event.oldVersion

      // v1 → v2: 创建所有 store（首次或从 v1 升级）
      if (oldVersion < 1) {
        // 首次创建：创建 flags store（兼容 useCountryFlag 旧数据）
        if (!db.objectStoreNames.contains(STORE_FLAGS)) {
          db.createObjectStore(STORE_FLAGS)
        }
      }
      if (oldVersion < 2) {
        // v2 新增：运营商图标缓存 + override
        if (!db.objectStoreNames.contains(STORE_ICONS)) {
          db.createObjectStore(STORE_ICONS)
        }
        if (!db.objectStoreNames.contains(STORE_OVERRIDES)) {
          db.createObjectStore(STORE_OVERRIDES)
        }
      }
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
  return dbPromise
}

/** 从指定 store 读取一个 key */
export async function idbGet(store: string, key: string): Promise<string | null> {
  try {
    const db = await getDB()
    return new Promise((resolve) => {
      const tx = db.transaction(store, 'readonly')
      const req = tx.objectStore(store).get(key)
      req.onsuccess = () => resolve(req.result ?? null)
      req.onerror = () => resolve(null)
    })
  } catch {
    return null
  }
}

/** 向指定 store 写入一个 key-value */
export async function idbSet(store: string, key: string, value: string): Promise<void> {
  try {
    const db = await getDB()
    await new Promise<void>((resolve) => {
      const tx = db.transaction(store, 'readwrite')
      tx.objectStore(store).put(value, key)
      tx.oncomplete = () => resolve()
      tx.onerror = () => resolve()
    })
  } catch {
    // 放弃缓存
  }
}

/** 从指定 store 删除一个 key */
export async function idbDelete(store: string, key: string): Promise<void> {
  try {
    const db = await getDB()
    await new Promise<void>((resolve) => {
      const tx = db.transaction(store, 'readwrite')
      tx.objectStore(store).delete(key)
      tx.oncomplete = () => resolve()
      tx.onerror = () => resolve()
    })
  } catch {
    // ignore
  }
}

/** 清空指定 store 的所有数据 */
export async function idbClear(store: string): Promise<void> {
  try {
    const db = await getDB()
    await new Promise<void>((resolve) => {
      const tx = db.transaction(store, 'readwrite')
      tx.objectStore(store).clear()
      tx.oncomplete = () => resolve()
      tx.onerror = () => resolve()
    })
  } catch {
    // ignore
  }
}

/** 统计指定 store 的记录数 */
export async function idbCount(store: string): Promise<number> {
  try {
    const db = await getDB()
    return new Promise((resolve) => {
      const tx = db.transaction(store, 'readonly')
      const req = tx.objectStore(store).count()
      req.onsuccess = () => resolve(req.result ?? 0)
      req.onerror = () => resolve(0)
    })
  } catch {
    return 0
  }
}

/** 获取指定 store 的所有 key */
export async function idbKeys(store: string): Promise<IDBValidKey[]> {
  try {
    const db = await getDB()
    return new Promise((resolve) => {
      const tx = db.transaction(store, 'readonly')
      const req = tx.objectStore(store).getAllKeys()
      req.onsuccess = () => resolve(req.result ?? [])
      req.onerror = () => resolve([])
    })
  } catch {
    return []
  }
}
