<script setup lang="ts">
import { ref, computed } from 'vue'
import { ChevronDown24Regular, ChevronRight24Regular } from '@vicons/fluent'
import type { EsimChipInfo } from '../types/api'

const props = defineProps<{
  chipInfo: EsimChipInfo | null
}>()

const expanded = ref(false)

const info = computed(() => {
  const eid = props.chipInfo?.eids?.[0]
  if (!eid) return null
  return {
    manufacturer: eid.manufacturer || '--',
    certificates: eid.certificates?.join(', ') || '--',
    firmware: eid.firmware || props.chipInfo?.firmware || '--',
    serial: props.chipInfo?.serial_number || '--',
    deviceName: props.chipInfo?.sku_name || 'eUICC',
  }
})
</script>

<template>
  <div v-if="info" class="euicc-info">
    <!-- 标题行（可折叠） -->
    <button class="euicc-info-header" @click="expanded = !expanded">
      <span class="euicc-info-title">EUICC INFO</span>
      <span class="euicc-info-device">{{ info.deviceName }}</span>
      <el-icon size="14" class="euicc-info-arrow">
        <component :is="expanded ? ChevronDown24Regular : ChevronRight24Regular" />
      </el-icon>
    </button>

    <!-- 展开内容 -->
    <div v-if="expanded" class="euicc-info-body">
      <div class="euicc-info-row">
        <span class="euicc-info-label">生产商</span>
        <span class="euicc-info-value">{{ info.manufacturer }}</span>
      </div>
      <div class="euicc-info-row">
        <span class="euicc-info-label">证书</span>
        <span class="euicc-info-value">{{ info.certificates }}</span>
      </div>
      <div class="euicc-info-row">
        <span class="euicc-info-label">固件</span>
        <span class="euicc-info-value">{{ info.firmware }}</span>
      </div>
      <div class="euicc-info-row">
        <span class="euicc-info-label">SN</span>
        <span class="euicc-info-value mono">{{ info.serial }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.euicc-info {
  border: 1px solid var(--border);
  border-radius: 6px;
  overflow: hidden;
}

.euicc-info-header {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 8px 12px;
  border: none;
  background: var(--muted);
  cursor: pointer;
  transition: background 0.12s;
}
.euicc-info-header:hover {
  background: var(--accent);
}

.euicc-info-title {
  font-size: 11px;
  font-weight: 700;
  color: var(--muted-foreground);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.euicc-info-device {
  flex: 1;
  font-size: 12px;
  font-weight: 600;
  color: var(--foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.euicc-info-arrow {
  flex-shrink: 0;
  color: var(--muted-foreground);
}

.euicc-info-body {
  display: flex;
  flex-direction: column;
}

.euicc-info-row {
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 6px 12px;
  border-bottom: 1px solid var(--border);
}
.euicc-info-row:last-child {
  border-bottom: none;
}

.euicc-info-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-foreground);
  flex-shrink: 0;
  min-width: 48px;
}

.euicc-info-value {
  flex: 1;
  font-size: 12px;
  color: var(--foreground);
  word-break: break-all;
}
.euicc-info-value.mono {
  font-family: var(--oomol-font-mono);
}
</style>
