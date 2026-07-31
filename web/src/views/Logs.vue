<script setup lang="ts">
import { ref, computed, h, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { ElButton, ElIcon, ElMessage } from 'element-plus'
import { ArrowDownload24Regular, Delete24Regular, Pause24Regular, Play24Regular } from '@vicons/fluent'
import { useLogsStore } from '../stores/logs'
import { useHeaderActionsStore } from '../stores/headerActions'
import { useEventStream } from '../composables/useEventStream'

// 日志条目类型
interface LogEntry {
  time: string
  level: string
  caller: string
  message: string
  fields?: string
}

const logsStore = useLogsStore()
const headerActions = useHeaderActionsStore()
const { logs } = storeToRefs(logsStore)

const connected = ref(false)
const paused = ref(false)
const autoScroll = ref(true)
const levelFilter = ref<'all' | 'debug' | 'info' | 'warn' | 'error'>('all')
const searchQuery = ref('')
const maxLogs = 1000 // 最大保留日志条数
const lastConnectError = ref<string>('')

// 日志容器引用
const logContainer = ref<HTMLElement | null>(null)

// 过滤后的日志
const filteredLogs = computed(() => {
  let result = logs.value

  // 级别过滤（精确匹配选中的级别）
  if (levelFilter.value !== 'all') {
    result = result.filter(log => 
      log.level.toLowerCase() === levelFilter.value.toLowerCase()
    )
  }

  // 搜索过滤
  if (searchQuery.value.trim()) {
    const q = searchQuery.value.toLowerCase()
    result = result.filter(log =>
      log.message.toLowerCase().includes(q) ||
      log.caller.toLowerCase().includes(q) ||
      (log.fields && log.fields.toLowerCase().includes(q))
    )
  }

  return result
})

const stream = useEventStream<LogEntry>({
  path: '/logs/stream',
  eventName: 'log',
  query: { level: '' },
  parse: (payload) => JSON.parse(payload) as LogEntry,
  onConnected: () => {
    connected.value = true
    lastConnectError.value = ''
  },
  onEvent: (entry) => {
    if (paused.value) return
    logsStore.append(entry, maxLogs)
    if (!autoScroll.value) return
    nextTick(() => {
      if (logContainer.value) logContainer.value.scrollTop = logContainer.value.scrollHeight
    })
  }
})

function connect() {
  connected.value = false
  stream.setPaused(false)
}

function disconnect() {
  stream.disconnect()
  connected.value = false
}

// 暂停/继续
function togglePause() {
  paused.value = !paused.value
  stream.setPaused(paused.value)
  if (!paused.value) connect()
}

// 清空日志
function clearLogs() {
  logsStore.clear()
}

// 导出日志
function exportLogs() {
  const content = filteredLogs.value.map(log => {
    const time = new Date(log.time).toLocaleString()
    const fields = log.fields ? ` ${log.fields}` : ''
    return `[${time}] ${log.level.toUpperCase().padEnd(5)} ${log.caller} ${log.message}${fields}`
  }).join('\n')

  const blob = new Blob([content], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `logs-${new Date().toISOString().slice(0, 10)}.txt`
  a.click()
  URL.revokeObjectURL(url)
  ElMessage.success('已导出日志')
}

// 日志级别颜色
function getLevelClass(level: string): string {
  switch (level.toLowerCase()) {
    case 'debug': return 'text-purple-500'
    case 'info': return 'text-blue-500'
    case 'warn': return 'text-yellow-500'
    case 'error': return 'text-red-500'
    case 'fatal': return 'text-red-600 font-bold'
    default: return 'text-gray-500'
  }
}

// 格式化日期时间
function formatDateTime(isoTime: string): string {
  try {
    const d = new Date(isoTime)
    const yyyy = d.getFullYear()
    const MM = String(d.getMonth() + 1).padStart(2, '0')
    const dd = String(d.getDate()).padStart(2, '0')
    const HH = String(d.getHours()).padStart(2, '0')
    const mm = String(d.getMinutes()).padStart(2, '0')
    const ss = String(d.getSeconds()).padStart(2, '0')
    return `${yyyy}-${MM}-${dd} ${HH}:${mm}:${ss}`
  } catch {
    return isoTime
  }
}

// 加载历史日志
async function loadHistory() {
  const result = await logsStore.fetchHistory(500)
  if (!result.ok) return
  nextTick(() => {
    if (logContainer.value) logContainer.value.scrollTop = logContainer.value.scrollHeight
  })
}

onMounted(async () => {
  await loadHistory()
  connect()
})

watch(paused, () => {
  headerActions.setActions(h('div', { class: 'flex items-center gap-2' }, [
    h(ElButton, { onClick: togglePause, type: paused.value ? 'success' : 'warning' }, () => [
      h(ElIcon, null, () => h(paused.value ? Play24Regular : Pause24Regular)),
      paused.value ? '继续' : '暂停'
    ]),
    h(ElButton, { onClick: clearLogs }, () => [
      h(ElIcon, null, () => h(Delete24Regular)),
      '清空'
    ]),
    h(ElButton, { onClick: exportLogs, type: 'primary' }, () => [
      h(ElIcon, null, () => h(ArrowDownload24Regular)),
      '导出'
    ])
  ]))
}, { immediate: true })

onUnmounted(() => {
  disconnect()
  headerActions.clear()
})

watch(levelFilter, () => {
  stream.setQuery({ level: levelFilter.value === 'all' ? '' : levelFilter.value })
  if (!paused.value) {
    connect()
  }
})
</script>

<template>
  <div class="logs-page">
    <div class="log-panel-wrapper">
      <!-- 过滤器栏 -->
      <div class="log-toolbar">
        <el-select v-model="levelFilter" placeholder="日志级别" class="w-32">
          <el-option label="全部" value="all" />
          <el-option label="DEBUG" value="debug" />
          <el-option label="INFO" value="info" />
          <el-option label="WARN" value="warn" />
          <el-option label="ERROR" value="error" />
        </el-select>
        <el-input
          v-model="searchQuery"
          placeholder="搜索日志内容..."
          clearable
          class="w-64"
        />
        <span class="text-sm" style="color: var(--muted-foreground);">显示 {{ filteredLogs.length }} / {{ logs.length }} 条</span>
        <div class="flex-1" />
        <div class="flex items-center gap-2">
          <span class="w-2 h-2 rounded-full" :style="{ background: connected ? 'var(--success)' : 'var(--destructive)', animation: connected ? 'pulse 2s ease-in-out infinite' : 'none' }" />
          <span class="text-sm" style="color: var(--muted-foreground);">{{ connected ? '已连接' : '未连接' }}</span>
        </div>
        <span v-if="!connected && lastConnectError" class="text-sm truncate" style="color: var(--destructive);" :title="lastConnectError">{{ lastConnectError }}</span>
        <el-checkbox v-model="autoScroll" label="自动追尾" />
      </div>

      <!-- 日志控制台 -->
      <div
        ref="logContainer"
        class="log-console"
      >
        <div v-if="filteredLogs.length === 0" class="text-center py-8" style="color: var(--muted-foreground);">
          {{ connected ? '等待日志...' : '未连接到日志流' }}
        </div>
        <div
          v-for="(log, idx) in filteredLogs"
          :key="idx"
          class="log-line"
        >
          <span class="log-time">[{{ formatDateTime(log.time) }}]</span>
          <span class="log-level" :class="getLevelClass(log.level)">{{ log.level.toUpperCase().padEnd(5) }}</span>
          <span class="log-caller" :title="log.caller">{{ log.caller }}</span>
          <span class="log-message">{{ log.message }}</span>
          <span v-if="log.fields" class="log-fields">{{ log.fields }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.logs-page {
  display: flex;
  flex-direction: column;
  height: calc(100svh - 56px - 48px);
  min-height: 0;
  overflow: hidden;
}

.log-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.filter-panel {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--card);
  box-shadow: var(--console-shadow-sm);
  padding: 16px;
}

.log-panel-wrapper {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--card);
  box-shadow: var(--console-shadow-sm);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.log-console {
  flex: 1;
  min-height: 0;
  overflow: auto;
  font-family: var(--oomol-font-sans);
  font-size: 12px;
  background: #1a1a1a;
  color: #e0e0e0;
  padding: 16px;
  border-radius: 0 0 8px 8px;
}

.log-line {
  padding: 1px 8px;
  margin: 0 -8px;
  border-radius: 4px;
  white-space: nowrap;
  line-height: 1.6;
}

.log-line:hover {
  background: rgba(255, 255, 255, 0.08);
}

.log-time {
  color: rgba(224, 224, 224, 0.5);
}

.log-level {
  display: inline-block;
  width: 56px;
  margin-left: 4px;
  font-weight: 700;
}

.log-caller {
  display: inline-block;
  width: 192px;
  margin-left: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  vertical-align: bottom;
  color: var(--brand);
}

.log-message {
  margin-left: 4px;
}

.log-fields {
  margin-left: 4px;
  color: color-mix(in oklab, var(--warning) 70%, transparent);
}
</style>
