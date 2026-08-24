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

const props = defineProps<{
  visible: boolean
  deviceId: string
}>()

const emit = defineEmits<{
  'update:visible': [val: boolean]
  'count-change': [count: number]
}>()

// F1: 先展示缓存数据（秒开），后台刷新后替换
const items = ref<NotificationItemWithStatus[]>([])
const loading = ref(false)
const retryingSeq = ref<number | null>(null)

// F2: 状态标签样式
const statusLabel = (status: NotificationStatus) => {
  switch (status) {
    case 'sent': return { text: '已发送', class: 'notif-status-sent' }
    case 'failed': return { text: '发送失败', class: 'notif-status-failed' }
    default: return null
  }
}

// F1: 打开弹窗时先展示缓存，再后台刷新
watch(() => props.visible, async (open) => {
  if (open) {
    // 秒开：先展示缓存数据
    items.value = getCachedNotifications(props.deviceId)
    // 后台刷新
    await fetchNotifications()
  }
})

// F1: 设备切换时加载缓存
watch(() => props.deviceId, () => {
  items.value = getCachedNotifications(props.deviceId)
})

async function fetchNotifications() {
  loading.value = true
  const result = await devicesService.getEsimNotifications(props.deviceId)
  try {
    if (!result.ok) throw result.error
    // F1: 后台刷新后替换缓存数据（合并本地状态）
    items.value = refreshNotificationCache(props.deviceId, result.data as EsimNotificationItem[])
    // F4: 通知父组件更新红点
    emit('count-change', items.value.length)
  } catch (e: unknown) {
    ElMessage.error(errorMessage(e, '获取通知列表失败'))
  } finally {
    loading.value = false
  }
}

async function retryNotification(item: NotificationItemWithStatus) {
  if (!item.can_retry || retryingSeq.value !== null) return
  retryingSeq.value = item.sequence_number
  try {
    const result = await devicesService.retryEsimNotification(props.deviceId, item.sequence_number, item.aid_hex || '')
    if (!result.ok) throw result.error
    retryingSeq.value = null
    ElMessage.success(result.data.message)

    // F2 + F3: 局部更新状态，不重新拉全量
    handleNotificationRetryResult(props.deviceId, item.sequence_number, true)
    // F3: 从列表中移除已发送的通知
    items.value = items.value.filter(i => i.sequence_number !== item.sequence_number)
    // F4: 通知父组件更新红点
    emit('count-change', items.value.length)
  } catch (e: unknown) {
    // F2: 标记为失败
    handleNotificationRetryResult(props.deviceId, item.sequence_number, false)
    retryingSeq.value = null
    ElMessage.error(errorMessage(e, '通知重试发送失败'))
    // 更新本地 item 状态
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
    title="通知列表"
    width="min(460px, 90vw)"
  >
    <!-- F1: 有缓存数据时，loading 覆盖在内容上方（不替换内容） -->
    <div v-if="items.length === 0 && loading" class="notif-loading">
      <div class="notif-loading-spinner" />
      <span>正在加载通知...</span>
    </div>
    <div v-else-if="items.length === 0 && !loading" class="notif-empty">
      当前没有可展示的通知
    </div>
    <div v-else class="notif-list">
      <!-- F1: loading 时显示半透明遮罩 -->
      <div v-if="loading" class="notif-refreshing-mask" />
      <div v-for="item in items" :key="item.sequence_number" class="notif-item">
        <div class="notif-item-main">
          <div class="notif-item-header">
            <span class="notif-seq">#{{ item.sequence_number }}</span>
            <span class="notif-event">{{ formatEsimNotificationEvent(item.event) }}</span>
            <!-- F2: 状态标签 -->
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
  </el-dialog>
</template>

<style scoped>
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

/* F1: 刷新时的半透明遮罩 */
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

/* F2: 状态标签 */
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
