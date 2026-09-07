<script setup lang="ts">
import { computed } from 'vue'
import type { DeviceOverviewItem } from '../types/api'

const props = defineProps<{
  device: DeviceOverviewItem | null
}>()

// 读卡器模式：esim_transport === 'pcsc'
const isPCSC = computed(() => props.device?.esim_transport === 'pcsc')

// 模组模式下的卡槽数据（模拟，后续接入后端）
interface SlotInfo {
  index: number
  label: string
  active: boolean
}

// TODO: 接入后端数据
const slots = computed<SlotInfo[]>(() => {
  return [
    { index: 1, label: '(闲置)', active: false },
    { index: 2, label: '(闲置)', active: false },
  ]
})
</script>

<template>
  <div class="sim-management-card">
    <div class="sim-header">
      <span class="sim-title">物理卡槽</span>
    </div>
    <div class="sim-body">
      <!-- 模组模式：显示 SIM 图 + 卡槽信息 -->
      <template v-if="!isPCSC">
        <!-- 左侧：SIM 卡图标盒子（预留） -->
        <div class="sim-icon-area">
          <div class="sim-icon-box" />
        </div>
        <!-- 右侧：卡槽信息 -->
        <div class="sim-slots">
          <div v-for="slot in slots" :key="slot.index" class="sim-slot-card" :class="{ active: slot.active }">
            <span class="sim-slot-label">卡槽{{ slot.index }}</span>
            <span class="sim-slot-value">{{ slot.label }}</span>
          </div>
        </div>
      </template>
      <!-- 读卡器模式：占位 -->
      <div v-else class="sim-placeholder">
        <span class="sim-placeholder-text">读卡器模式（占位）</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.sim-management-card {
  flex: 1;
  min-width: 0;
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  background: var(--background);
}

/* Header */
.sim-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px;
  background: var(--muted);
  border-bottom: 1px solid var(--border);
}
.sim-title {
  font-size: 11px;
  font-weight: 700;
  color: var(--muted-foreground);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

/* Body */
.sim-body {
  display: flex;
  flex: 1;
  min-height: 0;
}

/* 左侧 SIM 图标盒子 */
.sim-icon-area {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 12px;
  border-right: 1px solid var(--border);
  flex-shrink: 0;
}
.sim-icon-box {
  width: 38px;
  height: 38px;
  border-radius: 6px;
  background: var(--muted);
  border: 1px solid var(--border);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--muted-foreground);
}
.sim-icon-box svg {
  width: 20px;
  height: 20px;
}

/* 右侧卡槽信息 */
.sim-slots {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px;
  justify-content: center;
}
.sim-slot-card {
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 6px 10px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 12px;
  background: var(--background);
}
.sim-slot-card.active {
  border-color: color-mix(in oklab, var(--brand) 30%, var(--border));
  background: color-mix(in oklab, var(--brand) 5%, var(--background));
}
.sim-slot-label {
  color: var(--muted-foreground);
  flex-shrink: 0;
}
.sim-slot-value {
  color: var(--foreground);
  text-align: right;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sim-slot-card.active .sim-slot-value {
  color: var(--brand);
}

/* 读卡器模式占位 */
.sim-placeholder {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}
.sim-placeholder-text {
  font-size: 12px;
  color: var(--muted-foreground);
}
</style>
