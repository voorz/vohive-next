<script setup lang="ts">
import { ref } from 'vue'
import { ElMessageBox } from 'element-plus'
import { CallInbound24Regular, CallOutbound24Regular, CallMissed24Regular, Chat24Regular, Delete24Regular } from '@vicons/fluent'

defineEmits<{
  callback: [number: string]
  'toggle-select': [id: number]
  sms: [number: string]
  delete: [id: number]
}>()

const props = defineProps<{
  editMode?: boolean
  selectedIds?: Set<number>
}>()

type CallType = 'incoming' | 'outgoing' | 'missed'

interface CallRecord {
  id: number
  number: string
  type: CallType
  timestamp: string
  duration: number
}

// Mock 数据
const records = ref<CallRecord[]>([
  { id: 1, number: '+8613800138000', type: 'outgoing', timestamp: '2025-01-15T10:30:00Z', duration: 120 },
  { id: 2, number: '+8613900001111', type: 'incoming', timestamp: '2025-01-15T09:15:00Z', duration: 45 },
  { id: 3, number: '+8613700002222', type: 'missed', timestamp: '2025-01-14T18:00:00Z', duration: 0 },
  { id: 4, number: '+8618800003333', type: 'outgoing', timestamp: '2025-01-14T14:22:00Z', duration: 300 },
  { id: 5, number: '+8615500004444', type: 'missed', timestamp: '2025-01-13T20:10:00Z', duration: 0 },
  { id: 6, number: '+8613600005555', type: 'incoming', timestamp: '2025-01-13T16:45:00Z', duration: 90 },
  { id: 7, number: '+8615800006666', type: 'outgoing', timestamp: '2025-01-13T11:20:00Z', duration: 210 },
  { id: 8, number: '+8617700007777', type: 'missed', timestamp: '2025-01-12T22:30:00Z', duration: 0 },
  { id: 9, number: '+8619900008888', type: 'incoming', timestamp: '2025-01-12T19:05:00Z', duration: 60 },
  { id: 10, number: '+8613300009999', type: 'outgoing', timestamp: '2025-01-12T15:40:00Z', duration: 180 },
  { id: 11, number: '+8614400000001', type: 'missed', timestamp: '2025-01-11T21:15:00Z', duration: 0 },
  { id: 12, number: '+8615500000002', type: 'incoming', timestamp: '2025-01-11T17:30:00Z', duration: 75 },
  { id: 13, number: '+8616600000003', type: 'outgoing', timestamp: '2025-01-11T13:10:00Z', duration: 240 },
  { id: 14, number: '+8617700000004', type: 'missed', timestamp: '2025-01-10T23:45:00Z', duration: 0 },
  { id: 15, number: '+8618800000005', type: 'incoming', timestamp: '2025-01-10T20:00:00Z', duration: 105 },
  { id: 16, number: '+8619900000006', type: 'outgoing', timestamp: '2025-01-10T14:25:00Z', duration: 330 },
  { id: 17, number: '+8610000000007', type: 'missed', timestamp: '2025-01-09T18:50:00Z', duration: 0 },
  { id: 18, number: '+8611100000008', type: 'incoming', timestamp: '2025-01-09T16:15:00Z', duration: 55 },
  { id: 19, number: '+8612200000009', type: 'outgoing', timestamp: '2025-01-09T10:35:00Z', duration: 150 },
  { id: 20, number: '+8613300000010', type: 'missed', timestamp: '2025-01-08T22:20:00Z', duration: 0 },
])

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

function confirmDelete(id: number) {
  ElMessageBox.confirm(
    '确定要删除这条通话记录吗？此操作不可恢复。',
    '删除确认',
    { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
  ).then(() => {
    // TODO: 接入后端 API 删除单条记录
    records.value = records.value.filter(r => r.id !== id)
  }).catch(() => {})
}
</script>

<template>
  <div class="voice-history">
    <el-empty v-if="records.length === 0" description="暂无通话记录" :image-size="60" />
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
