/**
 * 运营商名称 → MCC/MNC 反查
 * 从 plmn-info 数据中匹配 eSIM Profile 的 service_provider_name
 */

import { ref, onMounted, type Ref } from 'vue'
import { loadPlmnInfo, getPlmnInfoData, type PlmnInfoEntry } from './plmn-info'
import { loadPlmnCatalog } from './plmn-catalog'
import { getCachedIcon, downloadIcon } from './useOperatorIcon'

type MccMnc = { mcc: string; mnc: string }

let reverseIndex: Map<string, MccMnc> | null = null

function buildReverseIndex(data: Record<string, PlmnInfoEntry>): Map<string, MccMnc> {
  const map = new Map<string, MccMnc>()
  for (const [plmn, entry] of Object.entries(data)) {
    const [mcc, mnc] = plmn.split('-')
    if (!mcc || !mnc) continue
    for (const op of entry.operators || []) {
      const names = [op.brand, op.operator].filter(Boolean) as string[]
      for (const name of names) {
        const key = name.toLowerCase().trim()
        if (key && !map.has(key)) {
          map.set(key, { mcc, mnc })
        }
      }
    }
  }
  return map
}

function getReverseIndex(): Map<string, MccMnc> | null {
  if (reverseIndex) return reverseIndex
  const data = getPlmnInfoData()
  if (!data) return null
  reverseIndex = buildReverseIndex(data)
  return reverseIndex
}

/**
 * 从运营商名称查找 MCC/MNC
 * 支持模糊匹配（startsWith / includes）
 */
export function providerToMccMnc(name: string): MccMnc | null {
  if (!name) return null
  const index = getReverseIndex()
  if (!index) return null

  const lower = name.toLowerCase().trim()

  // 1. 精确匹配
  if (index.has(lower)) return index.get(lower)!

  // 2. 去掉常见后缀后精确匹配 (如 "Vodafone DE" → "Vodafone")
  const parts = lower.split(/[\s]+/)
  if (parts.length > 1) {
    const base = parts[0]
    if (index.has(base)) return index.get(base)!
  }

  // 3. includes 匹配
  for (const [key, val] of index) {
    if (lower.includes(key) || key.includes(lower)) return val
  }

  return null
}

/**
 * 组合式函数：挂载时加载 PLMN 数据 + catalog
 * 并提供运营商图标自动下载能力
 */
export function useProviderLogo(): { ready: Ref<boolean> } {
  const ready = ref(false)

  onMounted(async () => {
    // 并行加载 plmn-info（反查用）和 plmn-catalog（图标目录）
    await Promise.all([loadPlmnInfo(), loadPlmnCatalog()])
    const data = getPlmnInfoData()
    if (data && !reverseIndex) {
      reverseIndex = buildReverseIndex(data)
    }
    ready.value = true
  })

  return { ready }
}

/**
 * 自动下载运营商图标（如果尚未缓存）
 * 下载成功后派发 vohive-icon-updated 事件，触发 CarrierIcon 刷新
 */
export function autoDownloadIcon(mcc: string, mnc: string, name?: string) {
  if (!mcc || !mnc) return
  if (getCachedIcon(mcc, mnc, name)) return
  downloadIcon(mcc, mnc, name).then(result => {
    if (result) {
      window.dispatchEvent(new CustomEvent('vohive-icon-updated', { detail: { mcc, mnc } }))
    }
  })
}
