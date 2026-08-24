<script setup lang="ts">
import { computed } from 'vue'
import type { EsimChipInfo, EsimEUICCInfo } from '../types/api'
import { ArrowSync24Regular, Alert24Regular, Eye24Regular, EyeOff24Regular } from '@vicons/fluent'

const props = defineProps<{
  chipInfo: EsimChipInfo | null
  showSensitive: boolean
  refreshing?: boolean
  notificationsLoading?: boolean
  notificationCount?: number
}>()

const emit = defineEmits<{
  refresh: []
  'open-notifications': []
  'toggle-sensitive': []
}>()

const chipName = computed(() => props.chipInfo?.sku_name || 'eUICC')
const firstEid = computed<EsimEUICCInfo | null>(() => props.chipInfo?.eids?.[0] ?? null)

const eidDisplay = computed(() => firstEid.value?.eid || '')
const freeNvram = computed(() => firstEid.value?.free_nvram || '')
// TODO: API 暂无总容量字段，后期补
const totalCapacity = computed(() => '')
</script>

<template>
  <div class="chip-card">
    <!-- 头部：芯片名 + 操作按钮（参照 param-section-label 样式） -->
    <div class="chip-card-header">
      <span class="chip-card-name">{{ chipName }}</span>
      <div class="chip-card-actions">
        <button class="chip-icon-btn" :disabled="refreshing" title="刷新" @click="emit('refresh')">
          <el-icon size="16" :class="{ 'spin': refreshing }"><ArrowSync24Regular /></el-icon>
        </button>
        <button class="chip-icon-btn" :disabled="notificationsLoading" title="通知" @click="emit('open-notifications')">
          <el-icon size="16"><Alert24Regular /></el-icon>
          <span v-if="notificationCount && notificationCount > 0" class="chip-badge">{{ notificationCount > 99 ? '99+' : notificationCount }}</span>
        </button>
        <button class="chip-icon-btn" :title="showSensitive ? '隐藏' : '显示'" @click="emit('toggle-sensitive')">
          <el-icon size="16"><Eye24Regular v-if="showSensitive" /><EyeOff24Regular v-else /></el-icon>
        </button>
      </div>
    </div>
    <!-- EID + 空间信息 -->
    <div class="chip-card-body">
      <div class="chip-card-eid-row">
        <span class="chip-card-eid-label">EID</span>
        <span class="chip-card-eid-value" :class="{ masked: !showSensitive }">{{ eidDisplay }}</span>
      </div>
      <div class="chip-card-space-row">
        <span v-if="freeNvram">可用 {{ freeNvram }}</span>
        <span class="chip-card-space-sep">|</span>
        <span>总容量 {{ totalCapacity || '--' }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.chip-card {
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--card);
  overflow: hidden;
  flex-shrink: 0;
}

/* 头部 — 参照 CarrierPreviewPanel .param-section-label */
.chip-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 10px;
  background: var(--muted);
  border-bottom: 1px solid var(--border);
}

.chip-card-name {
  font-size: 11px;
  font-weight: 700;
  color: var(--muted-foreground);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chip-card-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
}

.chip-icon-btn {
  position: relative;
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
.chip-icon-btn:hover {
  background: var(--accent);
  color: var(--foreground);
}
.chip-icon-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.chip-icon-btn .spin {
  animation: chip-spin 0.8s linear infinite;
}

.chip-badge {
  position: absolute;
  top: -2px;
  right: -2px;
  min-width: 14px;
  height: 14px;
  padding: 0 3px;
  border-radius: 7px;
  background: #ef4444;
  color: #fff;
  font-size: 9px;
  font-weight: 700;
  line-height: 14px;
  text-align: center;
  pointer-events: none;
  box-shadow: 0 0 0 1.5px var(--card);
}

@keyframes chip-spin {
  to { transform: rotate(360deg); }
}

.chip-card-body {
  padding: 8px 10px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.chip-card-eid-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.chip-card-eid-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-foreground);
  flex-shrink: 0;
}

.chip-card-eid-value {
  font-size: 11px;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: filter 0.15s;
}
.chip-card-eid-value.masked {
  filter: blur(3px);
  user-select: none;
}

.chip-card-space-row {
  font-size: 11px;
  color: var(--muted-foreground);
  display: flex;
  align-items: center;
  gap: 4px;
}

.chip-card-space-sep {
  opacity: 0.4;
}
</style>
