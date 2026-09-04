<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { ElSelect, ElOption, ElInput, ElButton, ElIcon, ElCheckbox, ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDownload24Regular, Delete24Regular, Pause24Regular, Play24Regular } from '@vicons/fluent'
import { type LogEntry } from './LogLine.vue'
import VirtualLogList from './VirtualLogList.vue'
import { useLogsStore } from '../stores/logs'
import { useEventStream } from '../composables/useEventStream'

const props = defineProps<{
  active: boolean
}>()

const emit = defineEmits<{
  (e: 'open-detail', log: LogEntry): void
}>()

const logsStore = useLogsStore()

const connected = ref(false)
const paused = ref(false)
const autoScroll = ref(true)
const levelFilter = ref<'all' | 'debug' | 'info' | 'warn' | 'error'>('all')
const searchQuery = ref('')
const maxLogs = ref(1000)
const viewportOptions = [
  { label: '500 条', value: 500 },
  { label: '1000 条', value: 1000 },
  { label: '2000 条', value: 2000 },
  { label: '5000 条', value: 5000 },
]
const lastConnectError = ref<string>('')

const logContainer = ref<HTMLElement | null>(null)

const realtimeLogs = computed(() => logsStore.logs)

const filteredLogs = computed(() => {
  let result = realtimeLogs.value

  if (levelFilter.value !== 'all') {
    result = result.filter(log =>
      log.level.toLowerCase() === levelFilter.value.toLowerCase()
    )
  }

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
    logsStore.append(entry, maxLogs.value)
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

function togglePause() {
  paused.value = !paused.value
  stream.setPaused(paused.value)
  if (!paused.value) connect()
}

async function clearLogs() {
  try {
    await ElMessageBox.confirm(
      '确认清空当天的日志？',
      '清空日志',
      { confirmButtonText: '清空', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }
  // 先清空前端内存，再清空后端文件
  logsStore.clear()
  const today = new Date().toISOString().slice(0, 10)
  const result = await logsStore.clearHistory(today)
  if (result.ok) {
    ElMessage.success('已清空当日日志')
  } else {
    ElMessage.error('清空失败')
  }
}

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

// 当 Tab 激活时确保 SSE 连接，失活时断开
watch(() => props.active, (active) => {
  if (active) {
    if (!paused.value) connect()
  } else {
    disconnect()
  }
}, { immediate: false })

onUnmounted(() => {
  disconnect()
})

watch(levelFilter, () => {
  stream.setQuery({ level: levelFilter.value === 'all' ? '' : levelFilter.value })
  if (!paused.value) {
    connect()
  }
})
</script>

<template>
  <div class="logs-realtime-tab">
    <!-- 工具栏 -->
    <div class="realtime-toolbar">
      <el-select v-model="levelFilter" placeholder="日志级别" class="w-32">
        <el-option label="全部" value="all" />
        <el-option label="DEBUG" value="debug" />
        <el-option label="INFO" value="info" />
        <el-option label="WARN" value="warn" />
        <el-option label="ERROR" value="error" />
      </el-select>

      <el-select v-model="maxLogs" placeholder="显示数量" class="w-32">
        <el-option
          v-for="opt in viewportOptions"
          :key="opt.value"
          :label="opt.label"
          :value="opt.value"
        />
      </el-select>

      <el-input
        v-model="searchQuery"
        placeholder="搜索日志内容..."
        clearable
        class="w-56"
      />

      <el-button
        @click="togglePause"
        :type="paused ? 'success' : 'warning'"
      >
        <el-icon><component :is="paused ? Play24Regular : Pause24Regular" /></el-icon>
        {{ paused ? '继续' : '暂停' }}
      </el-button>

      <span class="realtime-count">
        显示 {{ filteredLogs.length }} / {{ realtimeLogs.length }} 条
      </span>

      <div class="flex-1" />

      <div class="realtime-status">
        <span
          class="status-dot"
          :style="{
            background: connected ? 'var(--success, #00BC7D)' : 'var(--destructive, #ff3b30)',
            animation: connected ? 'pulse 2s ease-in-out infinite' : 'none'
          }"
        />
        <span class="status-text">{{ connected ? '已连接' : '未连接' }}</span>
      </div>

      <span
        v-if="!connected && lastConnectError"
        class="realtime-error"
        :title="lastConnectError"
      >{{ lastConnectError }}</span>

      <el-checkbox v-model="autoScroll" label="自动追尾" />

      <el-button type="danger" @click="clearLogs">
        <el-icon><Delete24Regular /></el-icon>
        删除
      </el-button>

      <el-button type="primary" @click="exportLogs">
        <el-icon><ArrowDownload24Regular /></el-icon>
        导出
      </el-button>
    </div>

    <!-- 日志控制台（虚拟滚动） -->
    <VirtualLogList
      :logs="filteredLogs"
      :auto-scroll="autoScroll"
      @open-detail="emit('open-detail', $event)"
    >
      <template #empty>
        <div class="realtime-empty">
          {{ connected ? '等待日志...' : '未连接到日志流' }}
        </div>
      </template>
    </VirtualLogList>
  </div>
</template>

<style scoped>
.logs-realtime-tab {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.realtime-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  height: 60px;
  padding: 0 16px;
  border-bottom: 1px solid var(--border, #E9E9E9);
  flex-shrink: 0;
}

.realtime-count {
  font-size: 12px;
  color: var(--muted-foreground);
  white-space: nowrap;
}

.realtime-status {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.status-text {
  font-size: 12px;
  color: var(--muted-foreground);
}

.realtime-error {
  font-size: 12px;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--destructive, #ff3b30);
}

.realtime-console {
  flex: 1;
  min-height: 0;
  overflow: auto;
  font-family: var(--oomol-font-sans);
  font-size: 12px;
  background: #000000;
  color: #e0e0e0;
  padding: 16px;
}

.realtime-empty {
  text-align: center;
  padding: 32px 0;
  color: var(--muted-foreground);
}

@keyframes pulse {
  0%, 100% { opacity: 0.3; transform: scale(1); }
  50% { opacity: 0.6; transform: scale(1.1); }
}
</style>
