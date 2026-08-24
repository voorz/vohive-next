import { ref, type Ref } from 'vue'
import type { EsimNotificationItem } from '../types/api'

/**
 * eSIM 通知状态管理（对标 NekoKo NotificationService + SQLite DB）
 *
 * 功能：
 * - F1: 通知列表缓存（秒开）—— 打开弹窗时先展示缓存数据，后台刷新后替换
 * - F2: 发送状态持久化 —— sessionStorage 记录 sent/failed 状态
 * - F3: 局部刷新 —— 重发后更新单条 item 状态，不重新拉全量
 * - F4: 红点同步 —— count 变化时通知父组件更新
 */

export type NotificationStatus = 'pending' | 'sent' | 'failed'

export type NotificationItemWithStatus = EsimNotificationItem & {
  status: NotificationStatus
}

type DeviceNotificationState = {
  items: NotificationItemWithStatus[]
  count: number
}

// 设备维度缓存
const deviceCache = new Map<string, DeviceNotificationState>()
// 设备维度 status 记录（sessionStorage key 前缀）
const STATUS_KEY_PREFIX = 'vohive:esim:notif:status:'

function loadStatusMap(deviceId: string): Map<number, NotificationStatus> {
  try {
    const raw = sessionStorage.getItem(STATUS_KEY_PREFIX + deviceId)
    if (!raw) return new Map()
    const arr = JSON.parse(raw) as [number, NotificationStatus][]
    return new Map(arr)
  } catch {
    return new Map()
  }
}

function saveStatusMap(deviceId: string, map: Map<number, NotificationStatus>) {
  try {
    const arr = Array.from(map.entries())
    sessionStorage.setItem(STATUS_KEY_PREFIX + deviceId, JSON.stringify(arr))
  } catch {
    // sessionStorage 满或不可用，忽略
  }
}

/**
 * 将 API 返回的 NotificationItem 列表合并本地状态，返回带 status 的列表。
 */
export function mergeWithLocalStatus(
  deviceId: string,
  items: EsimNotificationItem[]
): NotificationItemWithStatus[] {
  const statusMap = loadStatusMap(deviceId)
  return items.map(item => ({
    ...item,
    status: statusMap.get(item.sequence_number) ?? 'pending'
  }))
}

/**
 * 更新单条通知的状态（F2 + F3）。
 */
export function updateNotificationStatus(
  deviceId: string,
  sequenceNumber: number,
  status: NotificationStatus
) {
  const statusMap = loadStatusMap(deviceId)
  statusMap.set(sequenceNumber, status)
  saveStatusMap(deviceId, statusMap)

  // 同步更新内存缓存
  const cached = deviceCache.get(deviceId)
  if (cached) {
    const item = cached.items.find(i => i.sequence_number === sequenceNumber)
    if (item) {
      item.status = status
    }
  }
}

/**
 * 从缓存中移除已删除/已发送的通知（F3）。
 */
export function removeNotificationFromCache(
  deviceId: string,
  sequenceNumber: number
) {
  const cached = deviceCache.get(deviceId)
  if (cached) {
    cached.items = cached.items.filter(i => i.sequence_number !== sequenceNumber)
    cached.count = cached.items.length
  }
}

/**
 * 更新缓存（F1: 后台刷新后替换缓存数据）。
 */
export function updateNotificationCache(
  deviceId: string,
  items: EsimNotificationItem[]
) {
  const merged = mergeWithLocalStatus(deviceId, items)
  deviceCache.set(deviceId, {
    items: merged,
    count: merged.length
  })
}

/**
 * 获取缓存的通知列表（F1: 秒开用）。
 */
export function getCachedNotifications(deviceId: string): NotificationItemWithStatus[] {
  return deviceCache.get(deviceId)?.items ?? []
}

/**
 * 获取缓存的通知数量（F4: 红点更新用）。
 */
export function getCachedNotificationCount(deviceId: string): number {
  return deviceCache.get(deviceId)?.count ?? 0
}

/**
 * 清空设备缓存（设备切换/删除时）。
 */
export function clearNotificationCache(deviceId: string) {
  deviceCache.delete(deviceId)
}

// F4: 红点变化事件（简易 event bus）
type CountChangeCallback = (deviceId: string, count: number) => void
const countChangeListeners = new Set<CountChangeCallback>()

export function onNotificationCountChange(cb: CountChangeCallback) {
  countChangeListeners.add(cb)
  return () => {
    countChangeListeners.delete(cb)
  }
}

function emitCountChange(deviceId: string, count: number) {
  countChangeListeners.forEach(cb => cb(deviceId, count))
}

/**
 * 通知列表刷新后的完整更新流程（F1 + F4）。
 */
export function refreshNotificationCache(
  deviceId: string,
  items: EsimNotificationItem[]
): NotificationItemWithStatus[] {
  updateNotificationCache(deviceId, items)
  emitCountChange(deviceId, items.length)
  return getCachedNotifications(deviceId)
}

/**
 * 重发通知后的局部更新流程（F2 + F3 + F4）。
 */
export function handleNotificationRetryResult(
  deviceId: string,
  sequenceNumber: number,
  success: boolean
) {
  const status: NotificationStatus = success ? 'sent' : 'failed'
  updateNotificationStatus(deviceId, sequenceNumber, status)
  if (success) {
    // 发送成功后，从缓存中移除该通知（卡片端也会 autoClean/remove）
    removeNotificationFromCache(deviceId, sequenceNumber)
    emitCountChange(deviceId, getCachedNotificationCount(deviceId))
  }
}

/**
 * Composable: 在组件中使用通知缓存（F1 秒开）。
 */
export function useEsimNotifications(deviceId: Ref<string | undefined>) {
  const cachedItems = ref<NotificationItemWithStatus[]>([])
  const cachedCount = ref(0)

  function loadCache() {
    const id = deviceId.value
    if (!id) {
      cachedItems.value = []
      cachedCount.value = 0
      return
    }
    cachedItems.value = getCachedNotifications(id)
    cachedCount.value = getCachedNotificationCount(id)
  }

  return {
    cachedItems,
    cachedCount,
    loadCache
  }
}
