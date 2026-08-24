<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { devicesService } from '../services/devices'
import { errorMessage } from '../services/http'
import type { EsimNotificationItem } from '../types/api'
import {
  formatEsimNotificationEvent
} from './deviceEsimNotifications'
import {
  getCachedNotifications,
  refreshNotificationCache,
  handleNotificationRetryResult,
  type NotificationItemWithStatus,
  type NotificationStatus
} from '../composables/useEsimNotifications'
import ModuleEsimNotificationsSettings from './ModuleEsimNotificationsSettings.vue'

const props = defineProps<{
  visible: boolean
  deviceId: string
}>()

const emit = defineEmits<{
  'update:visible': [val: boolean]
  'count-change': [count: number]
}>()

// Tab 切换
const activeTab = ref<'current' | 'history'>('current')

// 当前通知
const items = ref<NotificationItemWithStatus[]>([])
const loading = ref(false)
const retryingSeq = ref<number | null>(null)
const settingsVisible = ref(false)

// 历史记录
interface HistoryRecord {
  eid: string
  seq_number: number
  iccid: string
  status: number
  notification_type: string
  notification_server: string
  response_code: number | null
  response_content: string
  timestamp: number
  created_at: string
  updated_at: string
}
const historyRecords = ref<HistoryRecord[]>([])
const historyLoading = ref(false)

const statusLabel = (status: NotificationStatus) => {
  switch (status) {
    case 'sent': return { text: '已发送', class: 'notif-status-sent' }
    case 'failed': return { text: '发送失败', class: 'notif-status-failed' }
    default: return null
  }
}

const historyStatusLabel = (status: number) => {
  switch (status) {
    case 1: return { text: '已发送', class: 'notif-status-sent' }
    case 2: return { text: '发送失败', class: 'notif-status-failed' }
    case 3: return { text: '已删除', class: 'notif-status-deleted' }
    default: return { text: '未发送', class: 'notif-status-pending' }
  }
}

const formatTime = (ts: number) => {
  if (!ts) return '--'
  return new Date(ts).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

watch(() => props.visible, async (open) => {
  if (open) {
    activeTab.value = 'current'
    items.value = getCachedNotifications(props.deviceId)
    await fetchNotifications()
  }
})

watch(() => props.deviceId, () => {
  items.value = getCachedNotifications(props.deviceId)
})

async function fetchNotifications() {
  loading.value = true
  const result = await devicesService.getEsimNotifications(props.deviceId)
  try {
    if (!result.ok) throw result.error
    items.value = refreshNotificationCache(props.deviceId, result.data as EsimNotificationItem[])
    emit('count-change', items.value.length)
  } catch (e: unknown) {
    ElMessage.error(errorMessage(e, '获取通知列表失败'))
  } finally {
    loading.value = false
  }
}

async function fetchHistory() {
  historyLoading.value = true
  const result = await devicesService.getEsimNotificationHistory(props.deviceId)
  try {
    if (!result.ok) throw result.error
    historyRecords.value = (result.data as HistoryRecord[] || []).sort((a, b) => b.seq_number - a.seq_number)
  } catch (e: unknown) {
    ElMessage.error(errorMessage(e, '获取通知历史记录失败'))
  } finally {
    historyLoading.value = false
  }
}

watch(activeTab, (tab) => {
  if (tab === 'history' && historyRecords.value.length === 0) {
    fetchHistory()
  }
})

async function retryNotification(item: NotificationItemWithStatus) {
  if (!item.can_retry || retryingSeq.value !== null) return
  retryingSeq.value = item.sequence_number
  try {
    const result = await devicesService.retryEsimNotification(props.deviceId, item.sequence_number, item.aid_hex || '')
    if (!result.ok) throw result.error
    retryingSeq.value = null
    ElMessage.success(result.data.message)
    handleNotificationRetryResult(props.deviceId, item.sequence_number, true)
    items.value = items.value.filter(i => i.sequence_number !== item.sequence_number)
    emit('count-change', items.value.length)
  } catch (e: unknown) {
    handleNotificationRetryResult(props.deviceId, item.sequence_number, false)
    retryingSeq.value = null
    ElMessage.error(errorMessage(e, '通知重试发送失败'))
    const idx = items.value.findIndex(i => i.sequence_number === item.sequence_number)
    if (idx >= 0) {
      items.value[idx] = { ...items.value[idx], status: 'failed' as NotificationStatus }
    }
  }
}
</script>

<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="(v: boolean) => emit('update:visible', v)"
    width="min(460px, 90vw)"
  >
    <template #header>
      <div class="notif-dialog-header">
        <span>通知列表</span>
        <button class="notif-settings-btn" title="通知处理设置" @click="settingsVisible = true">⚙</button>
      </div>
    </template>
    <ModuleEsimNotificationsSettings
      v-model:visible="settingsVisible"
      :device-id="deviceId"
    />

    <!-- Tab 切换 -->
    <div class="notif-tabs">
      <button
        class="notif-tab"
        :class="{ active: activeTab === 'current' }"
        @click="activeTab = 'current'"
      >当前通知</button>
      <button
        class="notif-tab"
        :class="{ active: activeTab === 'history' }"
        @click="activeTab = 'history'"
      >历史记录</button>
    </div>

    <!-- 当前通知 Tab -->
    <div v-if="activeTab === 'current'">
      <div v-if="items.length === 0 && loading" class="notif-loading">
        <div class="notif-loading-spinner" />
        <span>正在加载通知...</span>
      </div>
      <div v-else-if="items.length === 0 && !loading" class="notif-empty">
        当前没有可展示的通知
      </div>
      <div v-else class="notif-list">
        <div v-if="loading" class="notif-refreshing-mask" />
        <div v-for="item in items" :key="item.sequence_number" class="notif-item" :class="{ 'notif-item-sent': item.status === 'sent' }">
          <div v-if="item.status === 'sent'" class="notif-sent-corner" />
          <div class="notif-item-main">
            <div class="notif-item-header">
              <span class="notif-seq">#{{ item.sequence_number }}</span>
              <span class="notif-event">{{ formatEsimNotificationEvent(item.event) }}</span>
              <span
                v-if="statusLabel(item.status)"
                :class="statusLabel(item.status)!.class"
                class="notif-status-tag"
              >{{ statusLabel(item.status)!.text }}</span>
            </div>
            <div v-if="item.iccid" class="notif-meta">
              <span class="notif-meta-label">ICCID</span>
              <span class="notif-meta-value">{{ item.iccid }}</span>
            </div>
            <div v-if="item.address" class="notif-meta">
              <span class="notif-meta-label">地址</span>
              <span class="notif-meta-value">{{ item.address }}</span>
            </div>
          </div>
          <button
            class="notif-retry-btn"
            :disabled="!item.can_retry || retryingSeq === item.sequence_number || item.status === 'sent'"
            @click="retryNotification(item)"
          >
            {{ retryingSeq === item.sequence_number ? '...' : item.status === 'sent' ? '已发送' : '重发' }}
          </button>
        </div>
      </div>
    </div>

    <!-- 历史记录 Tab -->
    <div v-if="activeTab === 'history'">
      <div v-if="historyLoading" class="notif-loading">
        <div class="notif-loading-spinner" />
        <span>正在加载历史记录...</span>
      </div>
      <div v-else-if="historyRecords.length === 0" class="notif-empty">
        暂无历史记录
      </div>
      <div v-else class="notif-list">
        <div v-for="record in historyRecords" :key="`${record.seq_number}-${record.iccid}`" class="notif-item">
          <div class="notif-item-main">
            <div class="notif-item-header">
              <span class="notif-seq">#{{ record.seq_number }}</span>
              <span class="notif-event">{{ record.notification_type || '--' }}</span>
              <span
                :class="historyStatusLabel(record.status)!.class"
                class="notif-status-tag"
              >{{ historyStatusLabel(record.status)!.text }}</span>
            </div>
            <div v-if="record.iccid" class="notif-meta">
              <span class="notif-meta-label">ICCID</span>
              <span class="notif-meta-value">{{ record.iccid }}</span>
            </div>
            <div v-if="record.notification_server" class="notif-meta">
              <span class="notif-meta-label">服务器</span>
              <span class="notif-meta-value">{{ record.notification_server }}</span>
            </div>
            <div v-if="record.response_code" class="notif-meta">
              <span class="notif-meta-label">响应</span>
              <span class="notif-meta-value">{{ record.response_code }}</span>
            </div>
            <div class="notif-meta">
              <span class="notif-meta-label">时间</span>
              <span class="notif-meta-value">{{ formatTime(record.timestamp) }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </el-dialog>
</template>

<style scoped>
/* 覆盖 el-dialog body 默认高度限制 */
:deep(.el-dialog__body) {
  max-height: 60vh;
  overflow-y: auto;
}

.notif-dialog-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

.notif-settings-btn {
  border: none;
  background: transparent;
  font-size: 18px;
  cursor: pointer;
  padding: 4px;
  color: var(--muted-foreground);
  transition: color 0.12s;
}
.notif-settings-btn:hover {
  color: var(--brand);
}

/* Tab 切换 */
.notif-tabs {
  display: flex;
  gap: 4px;
  margin-bottom: 12px;
  border-bottom: 1px solid var(--border);
}

.notif-tab {
  padding: 6px 16px;
  border: none;
  background: transparent;
  font-size: 13px;
  font-weight: 600;
  color: var(--muted-foreground);
  cursor: pointer;
  border-bottom: 2px solid transparent;
  transition: color 0.12s, border-color 0.12s;
}

.notif-tab.active {
  color: var(--brand);
  border-bottom-color: var(--brand);
}

.notif-tab:hover:not(.active) {
  color: var(--foreground);
}

.notif-loading {
  padding: 40px 0;
  text-align: center;
  color: var(--muted-foreground);
  font-size: 13px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}

.notif-loading-spinner {
  width: 24px;
  height: 24px;
  border: 2px solid var(--border);
  border-top-color: var(--brand);
  border-radius: 999px;
  animation: notif-spin 0.8s linear infinite;
}

@keyframes notif-spin {
  to { transform: rotate(360deg); }
}

.notif-empty {
  padding: 40px 0;
  text-align: center;
  color: var(--muted-foreground);
  font-size: 13px;
}

.notif-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 420px;
  overflow-y: auto;
  position: relative;
}

.notif-refreshing-mask {
  position: absolute;
  inset: 0;
  background: var(--card);
  opacity: 0.5;
  z-index: 1;
  pointer-events: none;
  border-radius: 6px;
}

.notif-item {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  padding: 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
  position: relative;
  z-index: 0;
  overflow: hidden;
}

.notif-sent-corner {
  position: absolute;
  top: 0;
  right: 0;
  width: 0;
  height: 0;
  border-style: solid;
  border-width: 0 24px 24px 0;
  border-color: transparent #3b82f6 transparent transparent;
  z-index: 2;
  pointer-events: none;
}

.notif-item-sent {
  border-color: rgba(59, 130, 246, 0.3);
}

.notif-item-main {
  flex: 1;
  min-width: 0;
}

.notif-item-header {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 4px;
}

.notif-seq {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
}

.notif-event {
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 11px;
  font-weight: 600;
  background: var(--muted);
  color: var(--muted-foreground);
}

.notif-status-tag {
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 10px;
  font-weight: 600;
}

.notif-status-sent {
  background: rgba(0, 188, 125, 0.12);
  color: var(--brand);
}

.notif-status-failed {
  background: rgba(239, 68, 68, 0.12);
  color: #ef4444;
}

.notif-status-deleted {
  background: rgba(107, 114, 128, 0.12);
  color: var(--muted-foreground);
}

.notif-status-pending {
  background: rgba(59, 130, 246, 0.12);
  color: #3b82f6;
}

.notif-meta {
  display: flex;
  align-items: baseline;
  gap: 4px;
  font-size: 11px;
  margin-top: 2px;
}

.notif-meta-label {
  color: var(--muted-foreground);
  flex-shrink: 0;
}

.notif-meta-value {
  color: var(--foreground);
  font-family: var(--oomol-font-mono);
  word-break: break-all;
}

.notif-retry-btn {
  padding: 4px 12px;
  border: 1px solid var(--brand);
  border-radius: 4px;
  background: var(--brand);
  color: #fff;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: opacity 0.12s;
  flex-shrink: 0;
}
.notif-retry-btn:hover {
  opacity: 0.9;
}
.notif-retry-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
</style>
