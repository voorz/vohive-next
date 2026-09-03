import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { AppError } from '../types/domain'
import { logsService, type LogEntry, type LogDate } from '../services/logs'

export const useLogsStore = defineStore('logs', () => {
  // 实时日志
  const logs = ref<LogEntry[]>([])
  const loading = ref(false)
  const lastOkAt = ref<number | null>(null)
  const error = ref<AppError | null>(null)

  // 历史日志
  const historyLogs = ref<LogEntry[]>([])
  const availableDates = ref<LogDate[]>([])

  function append(entry: LogEntry, max = 1000) {
    logs.value.push(entry)
    if (logs.value.length > max) logs.value = logs.value.slice(-max)
  }

  /** 获取实时日志历史（加载初始历史填充到实时列表） */
  async function fetchHistory(lines = 500) {
    loading.value = true
    error.value = null
    const result = await logsService.history(lines)
    if (result.ok) {
      logs.value = result.data || []
      lastOkAt.value = Date.now()
    } else {
      error.value = result.error
    }
    loading.value = false
    return result
  }

  /** 按日期获取历史日志（填充到 historyLogs） */
  async function fetchHistoryByDate(date: string, lines = 500) {
    loading.value = true
    error.value = null
    const result = await logsService.history(lines, date)
    if (result.ok) {
      historyLogs.value = result.data || []
      lastOkAt.value = Date.now()
    } else {
      error.value = result.error
    }
    loading.value = false
    return result
  }

  /** 获取可用日志日期列表 */
  async function fetchDates() {
    const result = await logsService.dates()
    if (result.ok) {
      availableDates.value = result.data || []
    }
    return result
  }

  /** 清理历史日志文件 */
  async function clearHistory(date?: string) {
    return await logsService.clearHistory(date)
  }

  function clear() {
    logs.value = []
  }

  function clearHistoryLogs() {
    historyLogs.value = []
  }

  return {
    // 实时
    logs,
    loading,
    lastOkAt,
    error,
    append,
    fetchHistory,
    clear,
    // 历史
    historyLogs,
    availableDates,
    fetchHistoryByDate,
    fetchDates,
    clearHistory,
    clearHistoryLogs,
  }
})
