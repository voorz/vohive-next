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

export const carrierService = {
  list() {
    return callService(async () => {
      const res = await api.get('/carrier')
      return res.data as CarrierListItem[]
    })
  },

  get(mcc: string, mnc: string) {
    return callService(async () => {
      const res = await api.get(`/carrier/${mcc}/${mnc}`)
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

  remove(mcc: string, mnc: string) {
    return callService(async () => {
      await api.delete(`/carrier/${mcc}/${mnc}`)
      return true
    })
  },

  save(mcc: string, mnc: string, payload: CarrierSavePayload) {
    return callService(async () => {
      await api.put(`/carrier/${mcc}/${mnc}`, payload)
      return true
    })
  },

  deleteConfig(mcc: string, mnc: string) {
    return callService(async () => {
      await api.delete(`/carrier/${mcc}/${mnc}/config`)
      return true
    })
  },

  activate(mcc: string, mnc: string) {
    return callService(async () => {
      await api.post(`/carrier/${mcc}/${mnc}/activate`)
      return true
    })
  },

  deactivate(mcc: string, mnc: string) {
    return callService(async () => {
      await api.post(`/carrier/${mcc}/${mnc}/deactivate`)
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
