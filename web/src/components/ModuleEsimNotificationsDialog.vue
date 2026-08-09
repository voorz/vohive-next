<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { devicesService } from '../services/devices'
import { errorMessage } from '../services/http'
import type { EsimNotificationItem } from '../types/api'
import {
  formatEsimNotificationEvent,
  reconcileEsimNotificationDialogState
} from './deviceEsimNotifications'

const props = defineProps<{
  visible: boolean
  deviceId: string
}>()

const emit = defineEmits<{
  'update:visible': [val: boolean]
}>()

const loading = ref(false)
const items = ref<EsimNotificationItem[]>([])
const retryingSeq = ref<number | null>(null)

watch(() => props.visible, async (open) => {
  if (open) {
    await fetchNotifications()
  }
})

async function fetchNotifications() {
  loading.value = true
  const result = await devicesService.getEsimNotifications(props.deviceId)
  try {
    if (!result.ok) throw result.error
    items.value = result.data
  } catch (e: unknown) {
    ElMessage.error(errorMessage(e, '获取通知列表失败'))
  } finally {
    loading.value = false
  }
}

async function retryNotification(item: EsimNotificationItem) {
  if (!item.can_retry || retryingSeq.value !== null) return
  retryingSeq.value = item.sequence_number
  try {
    const result = await devicesService.retryEsimNotification(props.deviceId, item.sequence_number, item.aid_hex || '')
    if (!result.ok) throw result.error
    retryingSeq.value = null
    ElMessage.success(result.data.message)
    const refreshed = await devicesService.getEsimNotifications(props.deviceId)
    if (!refreshed.ok) {
      ElMessage.warning(refreshed.error.message || '通知已发送，但刷新列表失败')
      return
    }
    const nextState = reconcileEsimNotificationDialogState({
      isOpen: props.visible,
      items: items.value,
      refreshedItems: refreshed.data,
      retriedSequenceNumber: item.sequence_number
    })
    emit('update:visible', nextState.isOpen)
    items.value = nextState.items
    retryingSeq.value = nextState.retryingSequenceNumber
  } catch (e: unknown) {
    retryingSeq.value = null
    ElMessage.error(errorMessage(e, '通知重试发送失败'))
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
    <div v-if="loading" class="notif-loading">
      正在加载...
    </div>
    <div v-else-if="items.length === 0" class="notif-empty">
      当前没有可展示的通知
    </div>
    <div v-else class="notif-list">
      <div v-for="item in items" :key="item.sequence_number" class="notif-item">
        <div class="notif-item-main">
          <div class="notif-item-header">
            <span class="notif-seq">#{{ item.sequence_number }}</span>
            <span class="notif-event">{{ formatEsimNotificationEvent(item.event) }}</span>
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
          :disabled="!item.can_retry || retryingSeq === item.sequence_number"
          @click="retryNotification(item)"
        >
          {{ retryingSeq === item.sequence_number ? '...' : '重发' }}
        </button>
      </div>
    </div>
  </el-dialog>
</template>

<style scoped>
.notif-loading,
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
}

.notif-item {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  padding: 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
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
