<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
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
  MoreHorizontal24Regular,
  Pause24Regular,
  Play24Regular,
  Info24Regular
} from '@vicons/fluent'
import { devicesService } from '../services/devices'
import { errorMessage } from '../services/http'
import type { EsimNotificationItem, EsimChipInfo } from '../types/api'
import { formatEsimNotificationEvent } from './deviceEsimNotifications'
import CountryFlag from './CountryFlag.vue'
import { phoneToIso } from '../utils/phone-flag'
import { mccToIso, loadPlmnInfo } from '../composables/plmn-info'
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

// 暂停/播放 + 5s 倒计时后自动逐条处理通知（对标 NekoKoLPA2）
// 状态机：idle(有通知) → countdown(5s) → processing(SSE) → done
//         ↑←←← paused(暂停，可手动发送) ←←←↓
const processCountdown = ref(0)
const processing = ref(false)
const paused = ref(false)
const processingSeq = ref<number | null>(null)
let countdownTimer: ReturnType<typeof setInterval> | null = null
let processAbortCtrl: AbortController | null = null

// 暂停/播放按钮图标
const playPauseIcon = computed(() => {
  if (paused.value) return Play24Regular
  if (processing.value || processCountdown.value > 0) return Pause24Regular
  return Play24Regular
})
const playPauseTitle = computed(() => {
  if (paused.value) return '恢复处理'
  if (processing.value) return '暂停处理'
  if (processCountdown.value > 0) return '暂停倒计时'
  return '开始处理'
})
function togglePlayPause() {
  if (paused.value) {
    // 恢复：重新启动处理
    paused.value = false
    void startProcessing()
    return
  }
  // 暂停：停止倒计时或 abort SSE
  if (processCountdown.value > 0) {
    stopCountdown()
    paused.value = true
    return
  }
  if (processing.value) {
    stopProcessing()
    paused.value = true
    return
  }
  // 空闲且有通知 → 直接开始处理（跳过倒计时）
  if (notifItems.value.length > 0) {
    void startProcessing()
  }
}

interface NotifHistoryRecord {
  eid: string
  seq_number: number
  iccid: string
  profile_name?: string
  mcc?: string
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

const notifEventTagClass = (event: string) => {
  switch (event) {
    case 'install': return 'notif-event-tag-install'
    case 'enable': return 'notif-event-tag-enable'
    case 'disable': return 'notif-event-tag-disable'
    case 'delete': return 'notif-event-tag-delete'
    default: return 'notif-event-tag-default'
  }
}

const notifFlagIso = computed(() => {
  // 优先用 PLMN (MCC) 解析国旗，回退用 profile_name（手机号）
  return (item: NotificationItemWithStatus) => {
    const iso = mccToIso(item.mcc)
    if (iso) return iso
    if (item.profile_name) return phoneToIso(item.profile_name)
    return ''
  }
})

// 历史记录国旗 ISO — 优先用 PLMN (MCC)，回退用 profile_name
function notifHistoryFlagIso(record: NotifHistoryRecord): string {
  const iso = mccToIso(record.mcc)
  if (iso) return iso
  if (record.profile_name) return phoneToIso(record.profile_name)
  return ''
}

const notifFormatTime = (ts: number) => {
  if (!ts) return '--'
  return new Date(ts).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

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
  loadPlmnInfo()
})

onBeforeUnmount(() => {
  stopCountdown()
  stopProcessing()
  paused.value = false
})

async function fetchNotifications() {
  if (!props.deviceId) return
  notifLoading.value = true
  const result = await devicesService.getEsimNotifications(props.deviceId)
  try {
    if (!result.ok) throw result.error
    notifItems.value = refreshNotificationCache(props.deviceId, result.data as EsimNotificationItem[])
    emit('count-change', notifItems.value.length)
    // 有通知时启动 5s 倒计时（除非用户已暂停）
    if (notifItems.value.length > 0 && !paused.value) {
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

// 手动发送单条通知（暂停状态下可用）
async function sendNotification(item: NotificationItemWithStatus) {
  if (notifRetryingSeq.value !== null) return
  notifRetryingSeq.value = item.sequence_number
  try {
    const result = await devicesService.retryEsimNotification(props.deviceId, item.sequence_number, item.aid_hex || '')
    if (!result.ok) throw result.error
    ElMessage.success(result.data.message)
    handleNotificationRetryResult(props.deviceId, item.sequence_number, true)
    notifItems.value = notifItems.value.filter(i => i.sequence_number !== item.sequence_number)
    emit('count-change', notifItems.value.length)
  } catch (e: unknown) {
    handleNotificationRetryResult(props.deviceId, item.sequence_number, false)
    notifRetryingSeq.value = null
    ElMessage.error(errorMessage(e, '通知发送失败'))
    const idx = notifItems.value.findIndex(i => i.sequence_number === item.sequence_number)
    if (idx >= 0) {
      notifItems.value[idx] = { ...notifItems.value[idx], status: 'failed' as NotificationStatus }
    }
  }
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

const clearingHistory = ref(false)

async function clearNotifHistory() {
  const confirmed = await ElMessageBox.confirm(
    '确定要清空所有通知历史记录吗？此操作不可恢复。',
    '清空历史记录',
    { confirmButtonText: '清空', cancelButtonText: '取消', type: 'warning' }
  ).then(() => true).catch(() => false)
  if (!confirmed) return

  clearingHistory.value = true
  try {
    const result = await devicesService.clearEsimNotificationHistory(props.deviceId)
    if (!result.ok) throw result.error
    ElMessage.success(result.data.message || '已清空')
    notifHistoryRecords.value = []
  } catch (e: unknown) {
    ElMessage.error(errorMessage(e, '清空失败'))
  } finally {
    clearingHistory.value = false
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

    <!-- 设置面板模式 — 参照 preview-profile-wrapper + preview-profile-scroll -->
    <div v-if="notifTab === 'settings'" class="notif-settings-wrapper">
      <div class="notif-settings-scroll">
        <!-- 提示信息卡 -->
        <div class="settings-info-card">
          <div class="settings-info-icon">
            <el-icon size="16"><Info24Regular /></el-icon>
          </div>
          <div class="settings-info-text">
            处理通知有助于您的 eUICC 与 SM-DP+ 服务器（运营商）之间的同步。删除已发送的通知可以保持卡存储清洁。
          </div>
        </div>
        <ModuleEsimNotificationsSettings
          :device-id="deviceId"
        />
      </div>
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
            <!-- 倒计时 / 处理中 / 暂停 提示条（含暂停/播放按钮） -->
            <div v-if="paused" class="notif-countdown-bar">
              <span>自动清理已暂停</span>
              <button
                class="notif-playpause-btn"
                :title="playPauseTitle"
                @click="togglePlayPause"
              >
                <el-icon size="16"><component :is="playPauseIcon" /></el-icon>
              </button>
            </div>
            <div v-else-if="processCountdown > 0" class="notif-countdown-bar">
              <span>{{ processCountdown }}s 后自动清理...</span>
              <button
                class="notif-playpause-btn"
                :title="playPauseTitle"
                @click="togglePlayPause"
              >
                <el-icon size="16"><component :is="playPauseIcon" /></el-icon>
              </button>
            </div>
            <div v-else-if="processing" class="notif-countdown-bar">
              <div class="notif-countdown-spinner" />
              <span>自动清理中...</span>
              <button
                class="notif-playpause-btn"
                :title="playPauseTitle"
                @click="togglePlayPause"
              >
                <el-icon size="16"><component :is="playPauseIcon" /></el-icon>
              </button>
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
                    <!-- 第一行：国旗 + 卡名 -->
                    <div class="notif-card-line1">
                      <CountryFlag v-if="notifFlagIso(item)" :iso="notifFlagIso(item)" :size="14" />
                      <span class="notif-card-profilename" :class="{ masked: !showSensitive }">{{ item.profile_name || '--' }}</span>
                    </div>
                    <!-- 第二行：服务器地址 -->
                    <div v-if="item.address" class="notif-card-line2">
                      <span class="notif-card-address">{{ item.address }}</span>
                    </div>
                    <!-- 第三行：ICCID + 状态标签 + 序号 -->
                    <div class="notif-card-line3">
                      <span v-if="item.iccid" class="notif-card-iccid" :class="{ masked: !showSensitive }">{{ item.iccid }}</span>
                      <span
                        v-if="notifStatusLabel(item.status)"
                        :class="notifStatusLabel(item.status)!.class"
                        class="notif-card-status-tag"
                      >{{ notifStatusLabel(item.status)!.text }}</span>
                      <span class="notif-card-seq">#{{ item.sequence_number }}</span>
                    </div>
                    <!-- 第四行：事件标签 + EID 胶囊 -->
                    <div class="notif-card-line4">
                      <span class="notif-card-line4-event" :class="`notif-card-icon-${item.event}`">{{ item.event.toUpperCase() }}</span>
                      <span v-if="item.eid" class="notif-card-line4-event notif-card-eid-tag">...{{ item.eid.slice(-8) }}</span>
                    </div>
                  </div>
                  <div class="notif-card-actions">
                    <button
                      class="notif-card-btn"
                      :disabled="!item.can_retry || notifRetryingSeq === item.sequence_number || item.status === 'sent' || processing"
                      :title="item.status === 'sent' ? '已发送' : '发送'"
                      @click="sendNotification(item)"
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
                    <!-- 第一行：国旗 + 卡名 -->
                    <div class="notif-card-line1">
                      <CountryFlag v-if="notifHistoryFlagIso(record)" :iso="notifHistoryFlagIso(record)" :size="14" />
                      <span class="notif-card-profilename" :class="{ masked: !showSensitive }">{{ record.profile_name || '--' }}</span>
                    </div>
                    <!-- 第二行：服务器地址 -->
                    <div v-if="record.notification_server" class="notif-card-line2">
                      <span class="notif-card-address">{{ record.notification_server }}</span>
                    </div>
                    <!-- 第三行：ICCID（独占一行） -->
                    <div v-if="record.iccid" class="notif-card-line3">
                      <span class="notif-card-iccid" :class="{ masked: !showSensitive }">{{ record.iccid }}</span>
                    </div>
                    <!-- 第四行：时间 + 编号 + 状态标签 -->
                    <div class="notif-card-line4">
                      <span class="notif-card-time">{{ notifFormatTime(record.timestamp) }} #{{ record.seq_number }}</span>
                      <span
                        :class="notifHistoryStatusLabel(record.status)!.class"
                        class="notif-card-status-tag"
                      >{{ notifHistoryStatusLabel(record.status)!.text }}</span>
                    </div>
                    <!-- 第五行：事件标签 + EID 胶囊 -->
                    <div class="notif-card-line4">
                      <span class="notif-card-line4-event" :class="`notif-card-icon-${record.notification_type}`">{{ (record.notification_type || '').toUpperCase() }}</span>
                      <span v-if="record.eid" class="notif-card-line4-event notif-card-eid-tag">...{{ record.eid.slice(-8) }}</span>
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
      <span class="notif-footer-text">{{ footerText }}</span>
      <button
        v-if="notifTab === 'history' && notifHistoryRecords.length > 0"
        class="notif-clear-btn"
        :disabled="clearingHistory"
        @click="clearNotifHistory"
      >
        <el-icon v-if="clearingHistory" size="14" class="notif-spin"><Loading /></el-icon>
        <el-icon v-else size="14"><Delete24Regular /></el-icon>
        <span>清空消息</span>
      </button>
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
  overflow: hidden;
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

/* 提示卡片 — 参照 chip-card 样式 */
.settings-info-card {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--muted);
}

.settings-info-icon {
  width: 28px;
  height: 28px;
  border-radius: 5px;
  background: var(--background);
  border: 1px solid var(--border);
  color: var(--brand);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.settings-info-text {
  font-size: 12px;
  line-height: 1.6;
  color: var(--muted-foreground);
  padding-top: 4px;
}

/* 设置内容 — 参照 preview-profile-wrapper */
.notif-settings-wrapper {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  position: relative;
}

/* 设置内容滚动 — 参照 preview-profile-scroll */
.notif-settings-scroll {
  height: 100%;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
}

/* 列表 — 参照 preview-profile-wrapper */
.notif-list-wrapper {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  position: relative;
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

/* 倒计时 / 处理中提示条 */
.notif-countdown-bar {
  display: flex;
  align-items: center;
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
  /* 无额外样式，统一风格 */
}
.notif-countdown-bar.paused {
  /* 无额外样式，统一风格 */
}

/* 暂停/播放按钮 */
.notif-playpause-btn {
  width: 24px;
  height: 24px;
  border: none;
  border-radius: 50%;
  background: var(--brand);
  color: #fff;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.12s;
  flex-shrink: 0;
  margin-left: auto;
}
.notif-playpause-btn:hover {
  opacity: 0.85;
}
.notif-playpause-btn.paused {
  /* 无额外样式，统一风格 */
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
/* 图标列 — 纵向排列图标+事件标签 */
.notif-card-icon-col {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}
.notif-card-icon-label {
  width: 38px;
  text-align: center;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.2;
  border-radius: 4px;
  padding: 3px 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.notif-card-icon-label.notif-card-icon-install {
  background: rgba(59, 130, 246, 0.12);
  color: #3b82f6;
}
.notif-card-icon-label.notif-card-icon-enable {
  background: rgba(0, 188, 125, 0.12);
  color: var(--brand);
}
.notif-card-icon-label.notif-card-icon-disable {
  background: rgba(245, 158, 11, 0.12);
  color: #f59e0b;
}
.notif-card-icon-label.notif-card-icon-delete {
  background: rgba(239, 68, 68, 0.12);
  color: #ef4444;
}

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
.notif-card-profilename {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: filter 0.15s;
}
.notif-card-profilename.masked {
  filter: blur(3px);
  user-select: none;
}
/* 事件 tag 胶囊 */
.notif-event-tag {
  padding: 1px 8px;
  border-radius: 999px;
  font-size: 10px;
  font-weight: 600;
  flex-shrink: 0;
}
.notif-event-tag-install {
  background: rgba(59, 130, 246, 0.12);
  color: #3b82f6;
}
.notif-event-tag-enable {
  background: rgba(0, 188, 125, 0.12);
  color: var(--brand);
}
.notif-event-tag-disable {
  background: rgba(245, 158, 11, 0.12);
  color: #f59e0b;
}
.notif-event-tag-delete {
  background: rgba(239, 68, 68, 0.12);
  color: #ef4444;
}
.notif-event-tag-default {
  background: rgba(107, 114, 128, 0.12);
  color: var(--muted-foreground);
}
.notif-card-time {
  font-size: 11px;
  color: var(--muted-foreground);
  flex-shrink: 0;
}
.notif-card-seq {
  font-size: 11px;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
  flex-shrink: 0;
  margin-left: auto;
}
.notif-card-status-tag {
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 11px;
  flex-shrink: 0;
  margin-left: auto;
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
  flex: 1;
  min-width: 0;
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
}
.notif-card-line4-event {
  font-size: 11px;
  padding: 1px 4px;
  border-radius: 3px;
  flex-shrink: 0;
}
.notif-card-eid-tag {
  background: rgba(107, 114, 128, 0.12);
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
}
.notif-card-line4-event.notif-card-icon-install {
  background: rgba(59, 130, 246, 0.12);
  color: #3b82f6;
}
.notif-card-line4-event.notif-card-icon-enable {
  background: rgba(0, 188, 125, 0.12);
  color: var(--brand);
}
.notif-card-line4-event.notif-card-icon-disable {
  background: rgba(245, 158, 11, 0.12);
  color: #f59e0b;
}
.notif-card-line4-event.notif-card-icon-delete {
  background: rgba(239, 68, 68, 0.12);
  color: #ef4444;
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
  position: relative;
}

.notif-footer-text {
  flex: 1;
  text-align: center;
}

/* 清空消息按钮 — 绝对定位到右侧 */
.notif-clear-btn {
  position: absolute;
  right: 12px;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--background);
  color: var(--muted-foreground);
  font-size: 11px;
  cursor: pointer;
  transition: all 0.12s;
}
.notif-clear-btn:hover:not(:disabled) {
  border-color: #ef4444;
  color: #ef4444;
}
.notif-clear-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
