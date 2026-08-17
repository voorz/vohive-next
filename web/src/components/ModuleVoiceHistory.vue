<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { ElMessageBox } from 'element-plus'
import { CallInbound24Regular, CallOutbound24Regular, CallMissed24Regular, Chat24Regular, Delete24Regular } from '@vicons/fluent'
import { api } from '../stores/auth'

defineEmits<{
  callback: [number: string]
  'toggle-select': [id: number]
  sms: [number: string]
  delete: [id: number]
}>()

const props = defineProps<{
  editMode?: boolean
  selectedIds?: Set<number>
  deviceId?: string
}>()

type CallType = 'incoming' | 'outgoing' | 'missed'

interface CallRecord {
  id: number
  number: string
  type: CallType
  timestamp: string
  duration: number
}

const records = ref<CallRecord[]>([])
const loading = ref(false)

const typeIcons = {
  incoming: CallInbound24Regular,
  outgoing: CallOutbound24Regular,
  missed: CallMissed24Regular,
}

const typeLabels = {
  incoming: '来电',
  outgoing: '去电',
  missed: '未接',
}

async function fetchHistory() {
  if (!props.deviceId) return
  loading.value = true
  try {
    const res = await api.get(`/devices/${props.deviceId}/voice/history`, {
      params: { limit: 50 },
    })
    records.value = (res.data?.records || []).map((r: any) => ({
      id: r.id,
      number: r.number || r.peer || '',
      type: r.type as CallType,
      timestamp: r.timestamp,
      duration: r.duration || 0,
    }))
  } catch {
    records.value = []
  } finally {
    loading.value = false
  }
}

function formatTime(ts: string): string {
  const d = new Date(ts)
  const now = new Date()
  if (d.toDateString() === now.toDateString()) {
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
  return `${d.getMonth() + 1}/${d.getDate()} ${d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`
}

function formatDuration(sec: number): string {
  if (sec === 0) return ''
  const m = Math.floor(sec / 60)
  const s = sec % 60
  return `${m}m ${s}s`
}

async function confirmDelete(id: number) {
  ElMessageBox.confirm(
    '确定要删除这条通话记录吗？此操作不可恢复。',
    '删除确认',
    { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
  ).then(async () => {
    if (!props.deviceId) return
    try {
      await api.delete(`/devices/${props.deviceId}/voice/history/${id}`)
      records.value = records.value.filter(r => r.id !== id)
    } catch { /* ignore */ }
  }).catch(() => {})
}

// SSE 订阅通话状态变更，通话结束时刷新列表
let eventSource: EventSource | null = null

function setupSSE() {
  if (!props.deviceId) return
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
  const token = localStorage.getItem('token') || ''
  // EventSource 不支持自定义 header，用 query 传 token
  const base = api.defaults.baseURL || ''
  eventSource = new EventSource(
    `${base}/devices/${props.deviceId}/voice/stream?token=${token}`
  )
  eventSource.addEventListener('call', (e: MessageEvent) => {
    try {
      const data = JSON.parse(e.data)
      // 通话结束时刷新列表
      if (data.state === 'ended') {
        fetchHistory()
      }
    } catch { /* ignore */ }
  })
}

onMounted(() => {
  fetchHistory()
  setupSSE()
})

watch(() => props.deviceId, () => {
  fetchHistory()
  setupSSE()
})
</script>

<template>
  <div class="voice-history">
    <el-empty v-if="records.length === 0 && !loading" description="暂无通话记录" :image-size="60" />
    <div v-else class="history-list">
      <div
        v-for="record in records"
        :key="record.id"
        class="history-row"
        :class="{ editing: editMode }"
        @click="editMode ? $emit('toggle-select', record.id) : $emit('callback', record.number)"
      >
        <!-- 编辑模式 checkbox -->
        <el-checkbox
          v-if="editMode"
          :model-value="selectedIds?.has(record.id) ?? false"
          @change="$emit('toggle-select', record.id)"
          @click.stop
        />
        <div class="history-icon" :class="record.type">
          <el-icon size="16"><component :is="typeIcons[record.type]" /></el-icon>
        </div>
        <div class="history-info">
          <div class="history-number" :class="{ missed: record.type === 'missed' }">{{ record.number }}</div>
          <div class="history-meta">
            <span class="history-type">{{ typeLabels[record.type] }}</span>
            <span class="history-time">{{ formatTime(record.timestamp) }}</span>
          </div>
        </div>
        <div v-if="record.duration > 0" class="history-duration">
          {{ formatDuration(record.duration) }}
        </div>
        <!-- 非编辑模式悬停操作按钮 -->
        <div v-if="!editMode" class="history-actions" @click.stop>
          <button class="history-action-btn" title="发送短信" @click="$emit('sms', record.number)">
            <el-icon size="16"><Chat24Regular /></el-icon>
          </button>
          <button class="history-action-btn danger" title="删除记录" @click="confirmDelete(record.id)">
            <el-icon size="16"><Delete24Regular /></el-icon>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.voice-history {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow-y: auto;
}

.history-list {
  display: flex;
  flex-direction: column;
}

.history-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
  cursor: pointer;
  transition: background 0.12s;
}

.history-row:hover {
  background: var(--accent);
}

.history-row.editing {
  padding-left: 8px;
}

.history-icon {
  width: 32px;
  height: 32px;
  border-radius: 999px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.history-icon.incoming {
  background: color-mix(in oklab, var(--brand) 15%, transparent);
  color: var(--brand);
}

.history-icon.outgoing {
  background: color-mix(in oklab, var(--foreground) 10%, transparent);
  color: var(--foreground);
}

.history-icon.missed {
  background: color-mix(in oklab, #ef4444 15%, transparent);
  color: #ef4444;
}

.history-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.history-number {
  font-size: 14px;
  font-weight: 600;
  color: var(--foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--oomol-font-mono);
}

.history-number.missed {
  color: #ef4444;
}

.history-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--muted-foreground);
}

.history-type {
  font-weight: 600;
}

.history-duration {
  font-size: 11px;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
  flex-shrink: 0;
}

/* 悬停操作按钮 */
.history-actions {
  display: flex;
  gap: 4px;
  opacity: 0;
  transition: opacity 0.15s;
  flex-shrink: 0;
}

.history-row:hover .history-actions {
  opacity: 1;
}

.history-action-btn {
  width: 28px;
  height: 28px;
  border: none;
  background: transparent;
  color: var(--muted-foreground);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  transition: all 0.15s;
}

.history-action-btn:hover {
  background: var(--accent);
  color: var(--foreground);
}

.history-action-btn.danger:hover {
  background: #ef4444;
  color: #fff;
}
</style>
