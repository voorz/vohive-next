<script setup lang="ts">
import { computed } from 'vue'
import type { EsimChipInfo, EsimEUICCInfo } from '../types/api'
import { ArrowSync24Regular, Eye24Regular, EyeOff24Regular } from '@vicons/fluent'
import { HardwareChipOutline } from '@vicons/ionicons5'

const props = defineProps<{
  chipInfo: EsimChipInfo | null
  showSensitive: boolean
  refreshing?: boolean
  totalCapacityBytes?: number
  usedCapacityBytes?: number
  freeCapacityBytes?: number
  usagePercent?: number
  capacityFormatted?: string
}>()

const emit = defineEmits<{
  refresh: []
  'toggle-sensitive': []
}>()

const chipName = computed(() => props.chipInfo?.sku_name || 'eUICC')
const firstEid = computed<EsimEUICCInfo | null>(() => props.chipInfo?.eids?.[0] ?? null)

const eidDisplay = computed(() => firstEid.value?.eid || '')

// 容量展示：优先用后端计算好的，回退用 freeNvram
const usedDisplay = computed(() => {
  const bytes = props.usedCapacityBytes ?? 0
  if (bytes > 0) return formatBytes(bytes)
  return '--'
})
const freeDisplay = computed(() => {
  // 优先用后端的 free_capacity_bytes，回退用 eUICC 上报的 free_nvram
  const bytes = props.freeCapacityBytes ?? 0
  if (bytes > 0) return formatBytes(bytes)
  return firstEid.value?.free_nvram || '--'
})
const totalDisplay = computed(() => {
  // 优先用后端的 capacity_formatted，回退用 totalCapacityBytes 计算
  if (props.capacityFormatted) return props.capacityFormatted
  const bytes = props.totalCapacityBytes ?? 0
  if (bytes > 0) return formatBytes(bytes)
  return '--'
})
// 剩余容量低于 80kb 时标记为容量不足
const freeCapacityLow = computed(() => {
  const bytes = props.freeCapacityBytes ?? 0
  if (bytes > 0 && bytes < 80 * 1024) return true
  // 回退检查 free_nvram 文本（如 "338.00 kb"）
  const freeNvram = firstEid.value?.free_nvram || ''
  if (bytes === 0 && freeNvram) {
    const num = parseFloat(freeNvram)
    const unit = freeNvram.toLowerCase()
    if (unit.includes('b') && !unit.includes('kb') && !unit.includes('mb')) return num < 80
    if (unit.includes('kb')) return num < 80
  }
  return false
})

const usageDisplay = computed(() => {
  const pct = props.usagePercent ?? 0
  if (pct > 0) return pct.toFixed(1) + '%'
  return '--'
})

// 字节转人类可读：kb 值 ≥1000 时转 mb
function formatBytes(bytes: number): string {
  if (bytes >= 1000 * 1024) return (bytes / (1024 * 1024)).toFixed(2) + ' mb'
  if (bytes >= 1024) return (bytes / 1024).toFixed(1) + ' kb'
  return bytes + ' b'
}
</script>

<template>
  <div class="chip-card">
    <!-- 头部：芯片名 + 操作按钮（参照 param-section-label 样式） -->
    <div class="chip-card-header">
      <div class="chip-card-title">
        <el-icon size="14" class="chip-card-icon"><HardwareChipOutline /></el-icon>
        <span class="chip-card-name">{{ chipName }}</span>
      </div>
      <div class="chip-card-actions">
        <button class="chip-icon-btn" :disabled="refreshing" title="刷新" @click="emit('refresh')">
          <el-icon size="16" :class="{ 'spin': refreshing }"><ArrowSync24Regular /></el-icon>
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
      <div class="chip-card-capacity">
        <div class="chip-card-cap-item">
          <span class="chip-card-cap-label">总容量</span>
          <span class="chip-card-cap-value">{{ totalDisplay }}</span>
        </div>
        <div class="chip-card-cap-item">
          <span class="chip-card-cap-label">剩余</span>
          <el-popover
            :visible="freeCapacityLow"
            trigger="hover"
            placement="bottom"
            :width="240"
            :popper-style="{ textAlign: 'center' }"
            content="容量不足，请确保充足的容量以避免炸卡！"
          >
            <template #reference>
              <span class="chip-card-cap-value" :class="{ 'cap-warn': freeCapacityLow }">{{ freeDisplay }}</span>
            </template>
          </el-popover>
        </div>
        <div class="chip-card-cap-item">
          <span class="chip-card-cap-label">已用</span>
          <span class="chip-card-cap-value">{{ usedDisplay }}</span>
        </div>
        <div class="chip-card-cap-item">
          <span class="chip-card-cap-label">使用率</span>
          <span class="chip-card-cap-value chip-card-usage">{{ usageDisplay }}</span>
        </div>
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

.chip-card-title {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
}

.chip-card-icon {
  color: var(--muted-foreground);
  flex-shrink: 0;
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
  justify-content: space-between;
  gap: 6px;
}

.chip-card-eid-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-foreground);
  flex-shrink: 0;
}

.chip-card-eid-value {
  font-size: 12px;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
  text-align: right;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: filter 0.15s;
}
.chip-card-eid-value.masked {
  filter: blur(3px);
  user-select: none;
}

.chip-card-capacity {
  display: flex;
  width: 100%;
  gap: 4px;
}

.chip-card-cap-item {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1px;
  min-width: 0;
  background: var(--muted);
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 4px 2px;
}

.chip-card-cap-label {
  font-size: 10px;
  color: var(--muted-foreground);
  opacity: 0.7;
}

.chip-card-cap-value {
  font-size: 11px;
  font-weight: 600;
  color: var(--foreground);
  font-family: var(--oomol-font-mono);
  white-space: nowrap;
}

.chip-card-usage {
  color: var(--brand);
}

.cap-warn {
  color: #ef4444;
}
</style>
