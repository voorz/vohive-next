<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { EsimChipInfo, EsimEUICCInfo } from '../types/api'
import { ArrowSync24Regular, Eye24Regular, EyeOff24Regular } from '@vicons/fluent'
import { HardwareChipOutline } from '@vicons/ionicons5'

const props = defineProps<{
  chipInfo: EsimChipInfo | null
  showSensitive: boolean
  refreshing?: boolean
}>()

const emit = defineEmits<{
  refresh: []
  'toggle-sensitive': []
}>()

const chipName = computed(() => props.chipInfo?.sku_name || 'eUICC')
const eidList = computed<EsimEUICCInfo[]>(() => props.chipInfo?.eids ?? [])
const hasMultipleEids = computed(() => eidList.value.length > 1)

// 当前选中的 EID 索引（多 EID 时可循环切换）
const selectedEidIndex = ref(0)

// 当 chipInfo 变化时重置索引
watch(() => props.chipInfo, () => {
  selectedEidIndex.value = 0
})

const currentEidInfo = computed<EsimEUICCInfo | null>(() => {
  if (eidList.value.length === 0) return null
  return eidList.value[selectedEidIndex.value] ?? eidList.value[0]
})

const eidDisplay = computed(() => currentEidInfo.value?.eid || '')

// EID 行标签：恢复为标准 "EID"
const eidLabel = 'EID'

// 容量展示：从当前选中的 EID 独立获取
const usedDisplay = computed(() => {
  const bytes = currentEidInfo.value?.used_capacity_bytes ?? 0
  if (bytes > 0) return formatBytes(bytes)
  return '--'
})
const freeDisplay = computed(() => {
  const bytes = currentEidInfo.value?.free_nvram_bytes ?? 0
  if (bytes > 0) return formatBytes(bytes)
  return currentEidInfo.value?.free_nvram || '--'
})
const totalDisplay = computed(() => {
  const bytes = currentEidInfo.value?.total_capacity_bytes ?? 0
  if (bytes > 0) return formatBytes(bytes)
  return '--'
})
// 剩余容量低于 80kb 时标记为容量不足
const freeCapacityLow = computed(() => {
  const bytes = currentEidInfo.value?.free_nvram_bytes ?? 0
  if (bytes > 0 && bytes < 80 * 1024) return true
  const freeNvram = currentEidInfo.value?.free_nvram || ''
  if (bytes === 0 && freeNvram) {
    const num = parseFloat(freeNvram)
    const unit = freeNvram.toLowerCase()
    if (unit.includes('b') && !unit.includes('kb') && !unit.includes('mb')) return num < 80
    if (unit.includes('kb')) return num < 80
  }
  return false
})

const usageDisplay = computed(() => {
  const used = currentEidInfo.value?.used_capacity_bytes ?? 0
  const total = currentEidInfo.value?.total_capacity_bytes ?? 0
  if (total > 0) return ((used / total) * 100).toFixed(1) + '%'
  return '--'
})

// 切换到指定 EID
function selectEid(index: number) {
  selectedEidIndex.value = index
}
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
        <template v-if="hasMultipleEids">
          <button v-for="(eid, idx) in eidList" :key="eid.eid || idx" class="chip-icon-btn" :class="{ 'active': idx === selectedEidIndex }" :title="`EID${idx + 1}`" @click="selectEid(idx)">
            <span class="chip-eid-num">{{ idx + 1 }}</span>
          </button>
        </template>
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
        <span class="chip-card-eid-label">{{ eidLabel }}</span>
        <span class="chip-card-eid-value" :class="{ masked: !showSensitive }">{{ eidDisplay }}</span>
      </div>
      <div class="chip-card-capacity">
        <div class="chip-card-cap-item">
          <span class="chip-card-cap-label">总容量</span>
          <span class="chip-card-cap-value">{{ totalDisplay }}</span>
        </div>
        <div class="chip-card-cap-item">
          <span class="chip-card-cap-label">剩余</span>
          <el-tooltip
            :disabled="!freeCapacityLow"
            placement="bottom"
            :popper-style="{ textAlign: 'center', width: '240px' }"
            content="容量不足，请确保充足的容量以避免炸卡！"
          >
            <template #default>
              <span class="chip-card-cap-value" :class="{ 'cap-warn': freeCapacityLow }">{{ freeDisplay }}</span>
            </template>
          </el-tooltip>
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
  color: var(--brand);
}

/* EID 数字按钮 */
.chip-eid-num {
  font-size: 11px;
  font-weight: 700;
  font-family: var(--oomol-font-mono);
  line-height: 1;
}
.chip-icon-btn.active {
  background: var(--brand);
  color: #fff;
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
