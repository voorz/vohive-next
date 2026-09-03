<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import { ElSelect, ElOption, ElButton, ElIcon, ElMessage, ElMessageBox } from 'element-plus'
import { Delete24Regular, ArrowDownload24Regular } from '@vicons/fluent'
import LogLine, { type LogEntry } from './LogLine.vue'
import { useLogsStore } from '../stores/logs'

const emit = defineEmits<{
  (e: 'open-detail', log: LogEntry): void
}>()

const logsStore = useLogsStore()

const selectedDate = ref('')
const viewportCount = ref(500)
const searchQuery = ref('')
const levelFilter = ref<'all' | 'debug' | 'info' | 'warn' | 'error'>('all')

const viewportOptions = [
  { label: '100 条', value: 100 },
  { label: '500 条', value: 500 },
  { label: '1000 条', value: 1000 },
  { label: '2000 条', value: 2000 },
]

const availableDates = computed(() => logsStore.availableDates)
const historyLogs = computed(() => logsStore.historyLogs)
const loading = computed(() => logsStore.loading)

const filteredHistoryLogs = computed(() => {
  let result = historyLogs.value

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

  return result.slice(-viewportCount.value)
})

async function loadHistory() {
  if (!selectedDate.value) return
  const result = await logsStore.fetchHistoryByDate(selectedDate.value, viewportCount.value)
  if (!result.ok) {
    ElMessage.error('加载历史日志失败')
    return
  }
}

async function loadDates() {
  await logsStore.fetchDates()
  // 自动选中最新日期
  if (availableDates.value.length > 0 && !selectedDate.value) {
    selectedDate.value = availableDates.value[0]
  }
  await loadHistory()
}

async function handleDateChange() {
  await loadHistory()
}

async function handleViewportChange() {
  await loadHistory()
}

async function handleClearHistory() {
  try {
    await ElMessageBox.confirm(
      selectedDate.value
        ? `确认删除 ${selectedDate.value} 的历史日志文件？此操作不可恢复。`
        : '确认清理所有历史日志文件（保留当天）？此操作不可恢复。',
      '清理历史日志',
      { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }

  const result = await logsStore.clearHistory(selectedDate.value)
  if (result.ok) {
    ElMessage.success('历史日志已清理')
    await loadDates()
  } else {
    ElMessage.error('清理失败')
  }
}

function handleExport() {
  const content = filteredHistoryLogs.value.map(log => {
    const time = new Date(log.time).toLocaleString()
    const fields = log.fields ? ` ${log.fields}` : ''
    return `[${time}] ${log.level.toUpperCase().padEnd(5)} ${log.caller} ${log.message}${fields}`
  }).join('\n')

  const blob = new Blob([content], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `logs-history-${selectedDate.value || new Date().toISOString().slice(0, 10)}.txt`
  a.click()
  URL.revokeObjectURL(url)
  ElMessage.success('已导出历史日志')
}

const logContainer = ref<HTMLElement | null>(null)

onMounted(async () => {
  await loadDates()
  nextTick(() => {
    if (logContainer.value) logContainer.value.scrollTop = logContainer.value.scrollHeight
  })
})
</script>

<template>
  <div class="logs-history-tab">
    <!-- 控制栏 -->
    <div class="history-toolbar">
      <el-select
        v-model="selectedDate"
        placeholder="选择日期"
        class="w-40"
        @change="handleDateChange"
      >
        <el-option
          v-for="date in availableDates"
          :key="date"
          :label="date"
          :value="date"
        />
      </el-select>

      <el-select
        v-model="viewportCount"
        placeholder="显示数量"
        class="w-32"
        @change="handleViewportChange"
      >
        <el-option
          v-for="opt in viewportOptions"
          :key="opt.value"
          :label="opt.label"
          :value="opt.value"
        />
      </el-select>

      <el-select v-model="levelFilter" placeholder="级别" class="w-28">
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
        class="w-56"
      />

      <span class="history-count">
        显示 {{ filteredHistoryLogs.length }} / {{ historyLogs.length }} 条
      </span>

      <div class="flex-1" />

      <el-button @click="handleExport">
        <el-icon><ArrowDownload24Regular /></el-icon>
        导出
      </el-button>

      <el-button type="danger" @click="handleClearHistory">
        <el-icon><Delete24Regular /></el-icon>
        清理历史
      </el-button>
    </div>

    <!-- 日志列表 -->
    <div ref="logContainer" class="history-console">
      <div v-if="filteredHistoryLogs.length === 0" class="history-empty">
        {{ loading ? '加载中...' : '暂无历史日志' }}
      </div>
      <LogLine
        v-for="(log, idx) in filteredHistoryLogs"
        :key="idx"
        :log="log"
        @open-detail="emit('open-detail', $event)"
      />
    </div>
  </div>
</template>

<style scoped>
.logs-history-tab {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.history-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  height: 60px;
  padding: 0 16px;
  border-bottom: 1px solid var(--border, #E9E9E9);
  flex-shrink: 0;
}

.history-count {
  font-size: 12px;
  color: var(--muted-foreground);
  white-space: nowrap;
}

.history-console {
  flex: 1;
  min-height: 0;
  overflow: auto;
  font-family: var(--oomol-font-sans);
  font-size: 12px;
  background: #000000;
  color: #e0e0e0;
  padding: 16px;
}

.history-empty {
  text-align: center;
  padding: 32px 0;
  color: var(--muted-foreground);
}
</style>
