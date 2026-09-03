import { api } from '../stores/auth'
import { callService } from './http'

export type LogEntry = {
  time: string
  level: string
  caller: string
  message: string
  fields?: string
}

export type LogDate = string  // "2026-09-04"

export type ServiceResult<T> = {
  ok: boolean
  data?: T
  error?: { message: string }
}

export const logsService = {
  /** 获取可用日志日期列表 */
  dates() {
    return callService(async () => {
      const res = await api.get('/logs/dates')
      return (res.data?.dates || []) as LogDate[]
    })
  },

  /** 获取历史日志（可选按日期） */
  history(lines = 500, date?: string) {
    return callService(async () => {
      const params: Record<string, string | number> = { lines }
      if (date) params.date = date
      const res = await api.get('/logs/history', { params })
      return (res.data?.logs || []) as LogEntry[]
    })
  },

  /** 清理历史日志文件（可选按日期，不传则清理全部历史保留当天） */
  clearHistory(date?: string) {
    return callService(async () => {
      const params: Record<string, string> = {}
      if (date) params.date = date
      await api.delete('/logs/history', { params })
      return true
    })
  }
}
