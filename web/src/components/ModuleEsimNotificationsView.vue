<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import { api } from '../stores/auth'
import {
  ArrowLeft24Regular,
  Alert24Regular,
  History24Regular,
  Settings24Regular,
  ArrowDownload24Regular,
  CheckmarkCircle24Regular,
  DismissCircle24Regular,
  Delete24Regular,
  Send24Regular,
  MoreHorizontal24Regular
} from '@vicons/fluent'
import { devicesService } from '../services/devices'
import { errorMessage } from '../services/http'
import type { EsimNotificationItem, EsimChipInfo } from '../types/api'
import { formatEsimNotificationEvent } from './deviceEsimNotifications'
import {
  getCachedNotifications,
  refreshNotificationCache,
  handleNotificationRetryResult,
  type NotificationItemWithStatus,
  type NotificationStatus
} from '../composables/useEsimNotifications'
import ModuleEsimNotificationsSettings from './ModuleEsimNotificationsSettings.vue'

const props = defineProps<{
  deviceId: string
  chipInfo: EsimChipInfo | null
  showSensitive: boolean
}>()

const emit = defineEmits<{
  'back': []
  'count-change': [count: number]
}>()

const notifTab = ref<'current' | 'history' | 'settings'>('current')
const notifItems = ref<NotificationItemWithStatus[]>([])
const notifLoading = ref(false)
const notifRetryingSeq = ref<number | null>(null)

// 5s 倒计时后自动逐条处理通知（对标 NekoKoLPA2）
const processCountdown = ref(0)
const processing = ref(false)
const processingSeq = ref<number | null>(null)
let countdownTimer: ReturnType<typeof setInterval> | null = null
let processAbortCtrl: AbortController | null = null

interface NotifHistoryRecord {
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
const notifHistoryRecords = ref<NotifHistoryRecord[]>([])
const notifHistoryLoading = ref(false)

const notifEventIcon = (event: string) => {
  switch (event) {
    case 'install': return ArrowDownload24Regular
    case 'enable': return CheckmarkCircle24Regular
    case 'disable': return DismissCircle24Regular
    case 'delete': return Delete24Regular
    default: return Alert24Regular
  }
}

const notifStatusLabel = (status: NotificationStatus) => {
  switch (status) {
    case 'sent': return { text: '已发送', class: 'notif-status-sent' }
    case 'failed': return { text: '发送失败', class: 'notif-status-failed' }
    default: return null
  }
}

const notifHistoryStatusLabel = (status: number) => {
  switch (status) {
    case 1: return { text: '已发送', class: 'notif-status-sent' }
    case 2: return { text: '发送失败', class: 'notif-status-failed' }
    case 3: return { text: '已删除', class: 'notif-status-deleted' }
    default: return { text: '未发送', class: 'notif-status-pending' }
  }
}

const notifFormatTime = (ts: number) => {
  if (!ts) return '--'
  return new Date(ts).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

const eidDisplay = computed(() => props.chipInfo?.eids?.[0]?.eid || '')
const freeNvram = computed(() => props.chipInfo?.eids?.[0]?.free_nvram || '')
const chipName = computed(() => props.chipInfo?.sku_name || 'eUICC')
const footerText = computed(() => {
  if (notifTab.value === 'settings') return '通知处理设置'
  if (notifTab.value === 'current') return `当前通知 ${notifItems.value.length} 条`
  return `历史记录 ${notifHistoryRecords.value.length} 条`
})

watch(() => props.deviceId, () => {
  notifItems.value = getCachedNotifications(props.deviceId)
}, { immediate: true })

onMounted(() => {
  notifItems.value = getCachedNotifications(props.deviceId)
  fetchNotifications()
})

onBeforeUnmount(() => {
  stopCountdown()
  stopProcessing()
})

async function fetchNotifications() {
  if (!props.deviceId) return
  notifLoading.value = true
  const result = await devicesService.getEsimNotifications(props.deviceId)
  try {
    if (!result.ok) throw result.error
    notifItems.value = refreshNotificationCache(props.deviceId, result.data as EsimNotificationItem[])
    emit('count-change', notifItems.value.length)
    // 有通知时启动 5s 倒计时
    if (notifItems.value.length > 0) {
      startCountdown()
    }
  } catch (e: unknown) {
    ElMessage.error(errorMessage(e, '获取通知列表失败'))
  } finally {
    notifLoading.value = false
  }
}

function startCountdown() {
  stopCountdown()
  processCountdown.value = 5
  countdownTimer = setInterval(() => {
    processCountdown.value--
    if (processCountdown.value <= 0) {
      stopCountdown()
      void startProcessing()
    }
  }, 1000)
}

function stopCountdown() {
  if (countdownTimer) {
    clearInterval(countdownTimer)
    countdownTimer = null
  }
  processCountdown.value = 0
}

async function startProcessing() {
  if (processing.value || notifItems.value.length === 0) return
  processing.value = true
  processAbortCtrl = new AbortController()

  const token = localStorage.getItem('token') || ''
  const base = api.defaults.baseURL || ''
  const url = `${base}/devices/${props.deviceId}/esim/notifications/actions/process`

  try {
    const res = await fetch(url, {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}`, Accept: 'text/event-stream' },
      signal: processAbortCtrl.signal
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    if (!res.body) throw new Error('No stream body')

    const reader = res.body.getReader()
    const decoder = new TextDecoder('utf-8')
    let buffer = ''

    while (true) {
      const { value, done } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })

      while (true) {
        const nl = buffer.indexOf('\n')
        if (nl < 0) break
        let line = buffer.slice(0, nl)
        buffer = buffer.slice(nl + 1)
        if (line.endsWith('\r')) line = line.slice(0, -1)
        if (!line.startsWith('data:')) continue

        const payload = line.slice('data:'.length).trim()
        try {
          const evt = JSON.parse(payload) as {
            step: string
            sequence_number?: number
            event?: string
            message?: string
            processed_count?: number
            total_count?: number
          }
          handleProcessEvent(evt)
        } catch {
          // ignore parse error
        }
      }
    }
  } catch (err: unknown) {
    if (processAbortCtrl?.signal.aborted) return
    ElMessage.error(errorMessage(err, '通知处理失败'))
  } finally {
    processing.value = false
    processingSeq.value = null
  }
}

function handleProcessEvent(evt: {
  step: string
  sequence_number?: number
  message?: string
  processed_count?: number
  total_count?: number
}) {
  switch (evt.step) {
    case 'processing':
      processingSeq.value = evt.sequence_number ?? null
      break
    case 'sent':
    case 'deleted':
      // 从列表移除已处理的通知
      notifItems.value = notifItems.value.filter(i => i.sequence_number !== evt.sequence_number)
      emit('count-change', notifItems.value.length)
      break
    case 'failed':
      // 标记为失败但不移除
      {
        const idx = notifItems.value.findIndex(i => i.sequence_number === evt.sequence_number)
        if (idx >= 0) {
          notifItems.value[idx] = { ...notifItems.value[idx], status: 'failed' as NotificationStatus }
        }
      }
      break
    case 'skipped':
      // 跳过，不处理
      break
    case 'done':
      processingSeq.value = null
      ElMessage.success(evt.message || '处理完成')
      // 刷新 overview 缓存让红点递减
      break
    case 'error':
      ElMessage.error(evt.message || '处理失败')
      break
  }
}

function stopProcessing() {
  if (processAbortCtrl) {
    processAbortCtrl.abort()
    processAbortCtrl = null
  }
  processing.value = false
  processingSeq.value = null
}

async function fetchNotifHistory() {
  if (!props.deviceId) return
  notifHistoryLoading.value = true
  const result = await devicesService.getEsimNotificationHistory(props.deviceId)
  try {
    if (!result.ok) throw result.error
    notifHistoryRecords.value = (result.data as NotifHistoryRecord[] || []).sort((a, b) => b.seq_number - a.seq_number)
  } catch (e: unknown) {
    ElMessage.error(errorMessage(e, '获取通知历史记录失败'))
  } finally {
    notifHistoryLoading.value = false
  }
}

function switchNotifTab(tab: 'current' | 'history' | 'settings') {
  notifTab.value = tab
  if (tab === 'history' && notifHistoryRecords.value.length === 0) {
    fetchNotifHistory()
  }
}

async function retryNotification(item: NotificationItemWithStatus) {
  if (!item.can_retry || notifRetryingSeq.value !== null) return
  notifRetryingSeq.value = item.sequence_number
  try {
    const result = await devicesService.retryEsimNotification(props.deviceId, item.sequence_number, item.aid_hex || '')
    if (!result.ok) throw result.error
    notifRetryingSeq.value = null
    ElMessage.success(result.data.message)
    handleNotificationRetryResult(props.deviceId, item.sequence_number, true)
    notifItems.value = notifItems.value.filter(i => i.sequence_number !== item.sequence_number)
    emit('count-change', notifItems.value.length)
  } catch (e: unknown) {
    handleNotificationRetryResult(props.deviceId, item.sequence_number, false)
    notifRetryingSeq.value = null
    ElMessage.error(errorMessage(e, '通知重试发送失败'))
    const idx = notifItems.value.findIndex(i => i.sequence_number === item.sequence_number)
    if (idx >= 0) {
      notifItems.value[idx] = { ...notifItems.value[idx], status: 'failed' as NotificationStatus }
    }
  }
}
</script>

<template>
  <div class="notif-view">
    <!-- 头部 (60px) — 参照 preview-header -->
    <div class="notif-header">
      <div class="notif-header-left">
        <button class="notif-header-back" title="返回" @click="emit('back')">
          <el-icon size="20"><ArrowLeft24Regular /></el-icon>
        </button>
        <div class="notif-title">通知管理</div>
      </div>
      <div class="notif-header-actions">
        <button
          class="notif-tab-btn"
          :class="{ active: notifTab === 'current' }"
          title="当前通知"
          @click="switchNotifTab('current')"
        >
          <el-icon size="20"><Alert24Regular /></el-icon>
        </button>
        <button
          class="notif-tab-btn"
          :class="{ active: notifTab === 'history' }"
          title="历史记录"
          @click="switchNotifTab('history')"
        >
          <el-icon size="20"><History24Regular /></el-icon>
        </button>
        <button
          class="notif-tab-btn"
          :class="{ active: notifTab === 'settings' }"
          title="设置"
          @click="switchNotifTab('settings')"
        >
          <el-icon size="20"><Settings24Regular /></el-icon>
        </button>
      </div>
    </div>

    <!-- 设置面板模式 — 参照 preview-download-scroll -->
    <div v-if="notifTab === 'settings'" class="notif-settings-area">
      <ModuleEsimNotificationsSettings
        :device-id="deviceId"
      />
    </div>

    <!-- 通知列表模式 — 参照 preview-profile-wrapper + preview-profile-scroll -->
    <div v-else class="notif-list-wrapper">
      <div class="notif-list-scroll">
        <!-- 当前通知 Tab -->
        <template v-if="notifTab === 'current'">
          <div v-if="notifItems.length === 0 && notifLoading" class="notif-empty">
            <div class="notif-loading-spinner" />
            <span>正在加载通知...</span>
          </div>
          <div v-else-if="notifItems.length === 0 && !notifLoading" class="notif-empty">
            <el-empty description="当前没有可展示的通知" :image-size="60" />
          </div>
          <template v-else>
            <div v-if="notifLoading" class="notif-refreshing-mask" />
            <!-- 倒计时 / 处理中提示 -->
            <div v-if="processCountdown > 0" class="notif-countdown-bar">
              <span>{{ processCountdown }}s 后自动处理通知...</span>
            </div>
            <div v-if="processing" class="notif-countdown-bar processing">
              <div class="notif-countdown-spinner" />
              <span>正在逐条处理通知...</span>
            </div>
            <div
              v-for="item in notifItems"
              :key="item.sequence_number"
              class="notif-card"
              :class="{ 'notif-card-sent': item.status === 'sent' }"
            >
              <div class="notif-card-inner">
                <div class="notif-card-layout">
                  <div class="notif-card-icon" :class="`notif-card-icon-${item.event}`">
                    <el-icon size="20"><component :is="notifEventIcon(item.event)" /></el-icon>
                  </div>
                  <div class="notif-card-content">
                    <div class="notif-card-line1">
                      <span class="notif-card-event">{{ formatEsimNotificationEvent(item.event) }}</span>
                      <span class="notif-card-seq">#{{ item.sequence_number }}</span>
                      <span
                        v-if="notifStatusLabel(item.status)"
                        :class="notifStatusLabel(item.status)!.class"
                        class="notif-card-status-tag"
                      >{{ notifStatusLabel(item.status)!.text }}</span>
                    </div>
                    <div v-if="item.iccid" class="notif-card-line2">
                      <span class="notif-card-iccid" :class="{ masked: !showSensitive }">{{ item.iccid }}</span>
                    </div>
                    <div v-if="item.address" class="notif-card-line3">
                      <span class="notif-card-address">{{ item.address }}</span>
                    </div>
                  </div>
                  <div class="notif-card-actions">
                    <button
                      class="notif-card-btn"
                      :disabled="!item.can_retry || notifRetryingSeq === item.sequence_number || item.status === 'sent'"
                      :title="item.status === 'sent' ? '已发送' : '重发'"
                      @click="retryNotification(item)"
                    >
                      <el-icon v-if="notifRetryingSeq === item.sequence_number" size="16" class="notif-spin"><Loading /></el-icon>
                      <el-icon v-else size="16"><Send24Regular /></el-icon>
                    </button>
                    <button class="notif-card-btn" title="更多">
                      <el-icon size="16"><MoreHorizontal24Regular /></el-icon>
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </template>
        </template>

        <!-- 历史记录 Tab -->
        <template v-if="notifTab === 'history'">
          <div v-if="notifHistoryLoading" class="notif-empty">
            <div class="notif-loading-spinner" />
            <span>正在加载历史记录...</span>
          </div>
          <div v-else-if="notifHistoryRecords.length === 0" class="notif-empty">
            <el-empty description="暂无历史记录" :image-size="60" />
          </div>
          <template v-else>
            <div
              v-for="record in notifHistoryRecords"
              :key="`${record.seq_number}-${record.iccid}`"
              class="notif-card"
            >
              <div class="notif-card-inner">
                <div class="notif-card-layout">
                  <div class="notif-card-icon" :class="`notif-card-icon-${record.notification_type}`">
                    <el-icon size="20"><component :is="notifEventIcon(record.notification_type)" /></el-icon>
                  </div>
                  <div class="notif-card-content">
                    <div class="notif-card-line1">
                      <span class="notif-card-event">{{ formatEsimNotificationEvent(record.notification_type || '') }}</span>
                      <span class="notif-card-seq">#{{ record.seq_number }}</span>
                      <span
                        :class="notifHistoryStatusLabel(record.status)!.class"
                        class="notif-card-status-tag"
                      >{{ notifHistoryStatusLabel(record.status)!.text }}</span>
                    </div>
                    <div v-if="record.iccid" class="notif-card-line2">
                      <span class="notif-card-iccid">{{ record.iccid }}</span>
                    </div>
                    <div v-if="record.notification_server" class="notif-card-line3">
                      <span class="notif-card-address">{{ record.notification_server }}</span>
                    </div>
                    <div class="notif-card-line4">
                      <span>{{ notifFormatTime(record.timestamp) }}</span>
                    </div>
                  </div>
                  <div class="notif-card-actions">
                    <button v-if="record.status !== 1" class="notif-card-btn" title="重发">
                      <el-icon size="16"><Send24Regular /></el-icon>
                    </button>
                    <button class="notif-card-btn" title="更多">
                      <el-icon size="16"><MoreHorizontal24Regular /></el-icon>
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </template>
        </template>
      </div>
    </div>

    <!-- 底部预留栏 — 参照 preview-footer -->
    <div class="notif-footer">
      <span>{{ footerText }}</span>
    </div>
  </div>
</template>

<style scoped>
/* 容器 — 参照 preview-panel */
.notif-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  flex: 1;
}

/* 头部 — 参照 preview-header (60px) */
.notif-header {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 0 12px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.notif-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

/* 返回按钮 — 参照 preview-header-icon (38x38, 圆角6px, 边框) */
.notif-header-back {
  width: 38px;
  height: 38px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--background);
  color: var(--foreground);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: all 0.12s;
}
.notif-header-back:hover {
  border-color: var(--brand);
  color: var(--brand);
}

.notif-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--foreground);
}

.notif-header-actions {
  display: flex;
  align-items: center;
  gap: 2px;
}

/* Tab 图标按钮 — 参照 chip-icon-btn (32x32, 圆角6px) */
.notif-tab-btn {
  position: relative;
  width: 32px;
  height: 32px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--muted-foreground);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.12s;
}
.notif-tab-btn:hover {
  background: var(--accent);
  color: var(--foreground);
}
.notif-tab-btn.active {
  color: var(--brand);
}

/* 芯片卡区域 — 参照 preview-chip-area */
.notif-chip-area {
  flex-shrink: 0;
  padding: 12px;
}

/* 芯片卡 — 参照 chip-card */
.notif-chip-card {
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--card);
  overflow: hidden;
}
.notif-chip-card-header {
  padding: 6px 10px;
  background: var(--muted);
  border-bottom: 1px solid var(--border);
}
.notif-chip-card-name {
  font-size: 11px;
  font-weight: 700;
  color: var(--muted-foreground);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.notif-chip-card-body {
  padding: 8px 10px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.notif-chip-eid-row {
  display: flex;
  align-items: center;
  gap: 6px;
}
.notif-chip-eid-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-foreground);
  flex-shrink: 0;
}
.notif-chip-eid-value {
  font-size: 11px;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: filter 0.15s;
}
.notif-chip-eid-value.masked {
  filter: blur(3px);
  user-select: none;
}
.notif-chip-space-row {
  font-size: 11px;
  color: var(--muted-foreground);
  display: flex;
  align-items: center;
  gap: 4px;
}
.notif-chip-space-sep {
  opacity: 0.4;
}

/* 全宽分割线 — 参照 preview-section-divider */
.notif-section-divider {
  height: 1px;
  background: var(--border);
  flex-shrink: 0;
}

/* 设置面板区域 — 参照 preview-download-scroll */
.notif-settings-area {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 12px;
}

/* 列表 — 参照 preview-profile-wrapper (下沉式内阴影) */
.notif-list-wrapper {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  position: relative;
  box-shadow: inset 0 4px 6px -3px rgba(0,0,0,0.12), inset 0 -4px 6px -3px rgba(0,0,0,0.12);
}

/* 列表滚动 — 参照 preview-profile-scroll */
.notif-list-scroll {
  height: 100%;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
}

/* 空状态 / 加载中 — 参照 preview-empty / preview-loading */
.notif-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  flex: 1;
  min-height: 200px;
  color: var(--muted-foreground);
  font-size: 13px;
}

.notif-loading-spinner {
  width: 24px;
  height: 24px;
  border: 2px solid var(--border);
  border-top-color: var(--brand);
  border-radius: 999px;
  animation: notif-spin 0.8s linear infinite;
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

/* 倒计时 / 处理中提示条 */
.notif-countdown-bar {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--muted);
  font-size: 12px;
  font-weight: 600;
  color: var(--muted-foreground);
  flex-shrink: 0;
}
.notif-countdown-bar.processing {
  border-color: var(--brand);
  color: var(--brand);
}
.notif-countdown-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid var(--border);
  border-top-color: var(--brand);
  border-radius: 999px;
  animation: notif-spin 0.8s linear infinite;
}

@keyframes notif-spin {
  to { transform: rotate(360deg); }
}

/* 通知卡片 — 参照 profile-card 结构 */
.notif-card {
  border-radius: 8px;
  padding: 2px;
  background: var(--border);
  box-shadow: 0 1px 3px rgba(0,0,0,0.08);
  flex-shrink: 0;
  transition: box-shadow 0.15s;
}
.notif-card:hover {
  box-shadow: 0 2px 6px rgba(0,0,0,0.12);
}
.notif-card-sent {
  opacity: 0.6;
}
.notif-card-inner {
  background: linear-gradient(0deg, var(--background), var(--card));
  border-radius: 6px;
  padding: 10px;
}
.notif-card-layout {
  display: flex;
  gap: 10px;
  align-items: flex-start;
}

/* 操作类型图标盒子 — 参照 profile-card-logo (38x38, 圆角6px) */
.notif-card-icon {
  width: 38px;
  height: 38px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.notif-card-icon-install {
  background: rgba(59, 130, 246, 0.12);
  color: #3b82f6;
}
.notif-card-icon-enable {
  background: rgba(0, 188, 125, 0.12);
  color: var(--brand);
}
.notif-card-icon-disable {
  background: rgba(245, 158, 11, 0.12);
  color: #f59e0b;
}
.notif-card-icon-delete {
  background: rgba(239, 68, 68, 0.12);
  color: #ef4444;
}

/* 内容区 — 参照 profile-card-content */
.notif-card-content {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}
.notif-card-line1 {
  display: flex;
  align-items: center;
  gap: 6px;
}
.notif-card-event {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
}
.notif-card-seq {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
}
.notif-card-status-tag {
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
.notif-card-line2 {
  display: flex;
  align-items: center;
  gap: 6px;
}
.notif-card-iccid {
  font-size: 11px;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: filter 0.15s;
}
.notif-card-iccid.masked {
  filter: blur(3px);
  user-select: none;
}
.notif-card-line3 {
  display: flex;
  align-items: center;
  gap: 6px;
}
.notif-card-address {
  font-size: 11px;
  color: var(--muted-foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.notif-card-line4 {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--muted-foreground);
  opacity: 0.7;
}

/* 操作按钮区 — 参照 profile-card-gear (24x24, 圆角4px) */
.notif-card-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}
.notif-card-btn {
  width: 24px;
  height: 24px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--muted-foreground);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.12s;
}
.notif-card-btn:hover {
  background: var(--background);
  color: var(--foreground);
}
.notif-card-btn:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}
.notif-spin {
  animation: notif-spin 0.8s linear infinite;
}

/* 底部预留栏 — 参照 preview-footer */
.notif-footer {
  flex-shrink: 0;
  border-top: 1px solid var(--border);
  padding: 8px 12px;
  min-height: 40px;
  font-size: 11px;
  color: var(--muted-foreground);
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
