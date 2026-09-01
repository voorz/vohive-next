<script setup lang="ts">
import { ref, computed } from 'vue'
import { ChevronDown24Regular, ChevronRight24Regular } from '@vicons/fluent'
import type { EsimChipInfo, EsimEUICCInfo } from '../types/api'

const props = defineProps<{
  chipInfo: EsimChipInfo | null
}>()

// 每个 EID 的折叠状态，key 为 eid 字符串
const expandedMap = ref<Record<string, boolean>>({})

const eidList = computed<EsimEUICCInfo[]>(() => {
  return props.chipInfo?.eids ?? []
})

function isExpanded(eid: string): boolean {
  return expandedMap.value[eid] ?? false
}

function toggleExpanded(eid: string) {
  expandedMap.value[eid] = !isExpanded(eid)
}

function buildInfo(eidInfo: EsimEUICCInfo, index: number) {
  const specParts = [
    eidInfo.spec,
    eidInfo.spec_guess ? `(${eidInfo.spec_guess})` : '',
    eidInfo.spec_confidence ? `[${eidInfo.spec_confidence}]` : '',
  ].filter(Boolean).join(' ')
  return {
    title: `EUICC INFO(${index + 1})`,
    eidFull: eidInfo.eid || '--',
    manufacturer: eidInfo.manufacturer || '--',
    certificates: eidInfo.certificates?.join(', ') || '--',
    firmware: eidInfo.firmware || '--',
    serial: props.chipInfo?.serial_number || '--',
    deviceName: props.chipInfo?.sku_name || 'eUICC',
    spec: specParts || '--',
    infoSource: eidInfo.info_source || '--',
    infoVersion: eidInfo.info_version || '--',
    infoError: eidInfo.info_error || '',
    sasAccreditation: eidInfo.sas_accreditation_number || '--',
    defaultSmdp: eidInfo.default_smdp_address || '--',
    rootDs: eidInfo.root_ds_address || '--',
    freeNvram: eidInfo.free_nvram || '--',
  }
}
</script>

<template>
  <div v-if="eidList.length > 0" class="euicc-info-list">
    <div
      v-for="(eidInfo, index) in eidList"
      :key="eidInfo.eid || index"
      class="euicc-info"
    >
      <!-- 标题行（可折叠） -->
      <button class="euicc-info-header" @click="toggleExpanded(eidInfo.eid || String(index))">
        <span class="euicc-info-title">{{ buildInfo(eidInfo, index).title }}</span>
        <span class="euicc-info-device">{{ buildInfo(eidInfo, index).deviceName }}</span>
        <el-icon size="14" class="euicc-info-arrow">
          <component :is="isExpanded(eidInfo.eid || String(index)) ? ChevronDown24Regular : ChevronRight24Regular" />
        </el-icon>
      </button>

      <!-- 展开内容 -->
      <div v-if="isExpanded(eidInfo.eid || String(index))" class="euicc-info-body">
        <div class="euicc-info-row">
          <span class="euicc-info-label">EID</span>
          <span class="euicc-info-value mono">{{ eidInfo.eid }}</span>
        </div>
        <div class="euicc-info-row">
          <span class="euicc-info-label">AID</span>
          <span class="euicc-info-value mono">{{ eidInfo.aid }}</span>
        </div>
        <div class="euicc-info-row">
          <span class="euicc-info-label">生产商</span>
          <span class="euicc-info-value">{{ buildInfo(eidInfo, index).manufacturer }}</span>
        </div>
        <div class="euicc-info-row">
          <span class="euicc-info-label">证书</span>
          <span class="euicc-info-value">{{ buildInfo(eidInfo, index).certificates }}</span>
        </div>
        <div class="euicc-info-row">
          <span class="euicc-info-label">固件</span>
          <span class="euicc-info-value">{{ buildInfo(eidInfo, index).firmware }}</span>
        </div>
        <div class="euicc-info-row">
          <span class="euicc-info-label">SN</span>
          <span class="euicc-info-value mono">{{ buildInfo(eidInfo, index).serial }}</span>
        </div>
        <div class="euicc-info-row">
          <span class="euicc-info-label">规格</span>
          <span class="euicc-info-value mono">{{ buildInfo(eidInfo, index).spec }}</span>
        </div>
        <div class="euicc-info-row">
          <span class="euicc-info-label">剩余空间</span>
          <span class="euicc-info-value mono">{{ buildInfo(eidInfo, index).freeNvram }}</span>
        </div>
        <div class="euicc-info-row">
          <span class="euicc-info-label">信息来源</span>
          <span class="euicc-info-value mono">{{ buildInfo(eidInfo, index).infoSource }}</span>
        </div>
        <div class="euicc-info-row">
          <span class="euicc-info-label">版本</span>
          <span class="euicc-info-value mono">{{ buildInfo(eidInfo, index).infoVersion }}</span>
        </div>
        <div v-if="buildInfo(eidInfo, index).infoError" class="euicc-info-row">
          <span class="euicc-info-label">诊断</span>
          <span class="euicc-info-value" style="color: var(--destructive);">{{ buildInfo(eidInfo, index).infoError }}</span>
        </div>
        <div class="euicc-info-row">
          <span class="euicc-info-label">SAS</span>
          <span class="euicc-info-value mono">{{ buildInfo(eidInfo, index).sasAccreditation }}</span>
        </div>
        <div class="euicc-info-row">
          <span class="euicc-info-label">SM-DP+</span>
          <span class="euicc-info-value mono">{{ buildInfo(eidInfo, index).defaultSmdp }}</span>
        </div>
        <div class="euicc-info-row">
          <span class="euicc-info-label">SM-DS</span>
          <span class="euicc-info-value mono">{{ buildInfo(eidInfo, index).rootDs }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.euicc-info-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

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
  flex-shrink: 0;
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

.euicc-info-eid-short {
  font-size: 10px;
  font-family: var(--oomol-font-mono);
  color: var(--muted-foreground);
  opacity: 0.7;
  flex-shrink: 0;
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
  min-width: 56px;
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
