import { api } from '../stores/auth'
import { callService } from './http'
import type {
  CarrierListItem,
  CarrierDetail,
  CarrierSavePayload,
  CarrierAddPayload,
  CarrierBatchImportPayload,
  CarrierBatchImportResult,
  CarrierProfile
} from '../types/api'

/** 从 profile key 解析 mcc/mnc/brand */
export function parseKey(key: string): { mcc: string; mnc: string; brand: string } {
  let base = key
  let brand = ''
  const idx = base.indexOf('__')
  if (idx >= 0) {
    brand = base.slice(idx + 2)
    base = base.slice(0, idx)
  }
  const parts = base.split('-')
  return { mcc: parts[0] || '', mnc: parts[1] || '', brand }
}

export const carrierService = {
  list() {
    return callService(async () => {
      const res = await api.get('/carrier')
      return res.data as CarrierListItem[]
    })
  },

  get(key: string) {
    const { mcc, mnc, brand } = parseKey(key)
    return callService(async () => {
      const url = brand
        ? `/carrier/${mcc}/${mnc}?brand=${encodeURIComponent(brand)}`
        : `/carrier/${mcc}/${mnc}`
      const res = await api.get(url)
      return res.data as CarrierDetail
    })
  },

  add(payload: CarrierAddPayload) {
    return callService(async () => {
      await api.post('/carrier', payload)
      return true
    })
  },

  batchImport(payload: CarrierBatchImportPayload) {
    return callService(async () => {
      const res = await api.post('/carrier/batch', payload)
      return res.data as CarrierBatchImportResult
    })
  },

  remove(key: string) {
    const { mcc, mnc, brand } = parseKey(key)
    return callService(async () => {
      const url = brand
        ? `/carrier/${mcc}/${mnc}?brand=${encodeURIComponent(brand)}`
        : `/carrier/${mcc}/${mnc}`
      await api.delete(url)
      return true
    })
  },

  save(key: string, payload: CarrierSavePayload) {
    const { mcc, mnc, brand } = parseKey(key)
    return callService(async () => {
      const url = brand
        ? `/carrier/${mcc}/${mnc}?brand=${encodeURIComponent(brand)}`
        : `/carrier/${mcc}/${mnc}`
      await api.put(url, payload)
      return true
    })
  },

  deleteConfig(key: string) {
    const { mcc, mnc, brand } = parseKey(key)
    return callService(async () => {
      const url = brand
        ? `/carrier/${mcc}/${mnc}/config?brand=${encodeURIComponent(brand)}`
        : `/carrier/${mcc}/${mnc}/config`
      await api.delete(url)
      return true
    })
  },

  activate(key: string) {
    const { mcc, mnc, brand } = parseKey(key)
    return callService(async () => {
      const url = brand
        ? `/carrier/${mcc}/${mnc}/activate?brand=${encodeURIComponent(brand)}`
        : `/carrier/${mcc}/${mnc}/activate`
      await api.post(url)
      return true
    })
  },

  deactivate(key: string) {
    const { mcc, mnc, brand } = parseKey(key)
    return callService(async () => {
      const url = brand
        ? `/carrier/${mcc}/${mnc}/deactivate?brand=${encodeURIComponent(brand)}`
        : `/carrier/${mcc}/${mnc}/deactivate`
      await api.post(url)
      return true
    })
  },

  getDefaults() {
    return callService(async () => {
      const res = await api.get('/carrier/defaults')
      return res.data as CarrierListItem[]
    })
  },

  getDefault(mcc: string, mnc: string) {
    return callService(async () => {
      const res = await api.get(`/carrier/defaults/${mcc}/${mnc}`)
      return res.data as CarrierProfile
    })
  }
}
