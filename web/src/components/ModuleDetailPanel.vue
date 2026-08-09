<script setup lang="ts">
import { computed, ref, watch, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useDevicesStore } from '../stores/devices'
import ListSkeleton from './ListSkeleton.vue'
import EmptyState from './EmptyState.vue'
import CountryFlag from './CountryFlag.vue'
import CarrierIcon from './CarrierIcon.vue'
import ModuleAtTerminal from './ModuleAtTerminal.vue'
import ModuleUssdTerminal from './ModuleUssdTerminal.vue'
import ModuleCardPolicy from './ModuleCardPolicy.vue'
import ModuleConfigForm from './ModuleConfigForm.vue'
import { getPlmnInfo, loadPlmnInfo, type PlmnInfoEntry } from '../composables/plmn-info'
import { ArrowSync24Regular } from '@vicons/fluent'
import { cardsService } from '../services/cards'
import type { CardPolicy } from '../types/api'

const props = defineProps<{
  selectedId?: string
}>()

const emit = defineEmits<{
  'open-search': []
  'select': [id: string]
}>()

const store = useDevicesStore()
const { list, detail, loading } = storeToRefs(store)

const operatorName = computed(() => {
  const spn = detail.value?.modem?.native_spn
  if (spn) return spn
  const op = detail.value?.modem?.operator
  if (op) return op
  const brand = plmnInfo.value?.operators?.[0]?.brand
  if (brand) return brand
  const operator = plmnInfo.value?.operators?.[0]?.operator
  if (operator) return operator
  return detail.value?.name || '未选择'
})
const selectedImei = computed(() => detail.value?.modem?.imei || '')

// PLMN 信息（SIM 卡原始 PLMN，非当前接入网络）
const plmnInfo = ref<PlmnInfoEntry | null>(null)

onMounted(() => loadPlmnInfo())

watch(() => [detail.value?.modem?.native_mcc, detail.value?.modem?.native_mnc], ([mcc, mnc]) => {
  const key = mcc && mnc ? `${mcc}-${mnc}` : ''
  plmnInfo.value = key ? getPlmnInfo(key) : null
}, { immediate: true })

const nativePlmn = computed(() => {
  const mcc = detail.value?.modem?.native_mcc
  const mnc = detail.value?.modem?.native_mnc
  return mcc && mnc ? `${mcc}:${mnc}` : ''
})
const countryName = computed(() => plmnInfo.value?.country?.name || '')
const countryIso = computed(() => plmnInfo.value?.country?.iso || '')
const countryCode = computed(() => plmnInfo.value?.country?.code || '')
const nativeSpn = computed(() => detail.value?.modem?.native_spn || '')

// 卡策略
const cardPolicy = ref<CardPolicy | null>(null)

async function fetchCardPolicy(iccid: string | undefined) {
  if (!iccid) {
    cardPolicy.value = null
    return
  }
  const result = await cardsService.getPolicy(iccid)
  if (result.ok) {
    cardPolicy.value = result.data
  }
}

watch(() => detail.value?.modem?.iccid, (iccid) => { void fetchCardPolicy(iccid) }, { immediate: true })

async function onCardPolicyChanged() {
  await fetchCardPolicy(detail.value?.modem?.iccid)
}

// 当前 Tab
const activeTab = ref('overview')

// Tab 列表
const tabs = [
  { name: 'overview', label: '概览' },
  { name: 'at', label: 'AT终端' },
  { name: 'ussd', label: 'USSD' },
  { name: 'card', label: '卡策略' },
  { name: 'config', label: '配置' }
]

function initials(name: string): string {
  return name.charAt(0).toUpperCase()
}
</script>

<template>
  <div class="module-detail-panel">
    <!-- 详情头部 (60px) -->
    <div class="detail-header">
      <!-- 窄屏下拉选择器 + 添加按钮 -->
      <div class="detail-header-narrow">
        <div class="detail-header-icon-box">
          {{ initials(operatorName) }}
        </div>
        <el-select
          :model-value="props.selectedId"
          @change="(v: string) => emit('select', v)"
          placeholder="选择设备"
          class="!w-full"
        >
          <el-option
            v-for="d in list"
            :key="d.id"
            :label="d.name"
            :value="d.id"
          />
        </el-select>
        <el-button size="small" type="primary" @click="emit('open-search')" class="!border-0 add-btn-narrow">
          <el-icon class="mr-1"><ArrowSync24Regular /></el-icon>
          <span>添加</span>
        </el-button>
      </div>
      <!-- 宽屏：图标盒子 + 设备名 + 详细信息 -->
      <div class="detail-header-wide">
        <CarrierIcon :mcc="detail?.modem?.native_mcc || ''" :mnc="detail?.modem?.native_mnc || ''" :name="nativeSpn" :size="38" />
        <div class="detail-header-info">
          <div class="detail-header-name">{{ operatorName }}</div>
          <div class="detail-header-meta">
            <span v-if="nativePlmn" class="detail-header-plmn">{{ nativePlmn }}</span>
            <span v-if="countryCode" class="detail-header-code">+{{ countryCode }}</span>
            <CountryFlag v-if="countryIso" :iso="countryIso" :size="16" class="detail-header-flag" />
            <span v-if="countryName" class="detail-header-country">{{ countryName }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 第二层容器 -->
    <div v-if="detail" class="detail-content">
      <div class="edit-area-wrap">
        <div class="detail-inner">
      <!-- WiFi Calling 行 -->
      <div class="vowifi-row">
        <div class="vowifi-row-left">
          <span class="vowifi-label">WiFi Calling</span>
          <span class="vowifi-status-dot" :class="{ on: detail.vowifi_enabled }" />
          <span class="vowifi-status-text">{{ detail.vowifi_enabled ? '已启用' : '未启用' }}</span>
        </div>
        <button class="vowifi-reset-btn" :disabled="!detail.vowifi_enabled">
          <span class="vr-text">
            <el-icon size="14"><ArrowSync24Regular /></el-icon>
            <span>重启 VoWiFi</span>
          </span>
        </button>
      </div>

      <!-- Tab 切换 -->
      <div class="tab-bar">
        <button
          v-for="tab in tabs"
          :key="tab.name"
          class="tab-item"
          :class="{ active: activeTab === tab.name }"
          @click="activeTab = tab.name"
        >
          <span>{{ tab.label }}</span>
        </button>
      </div>

      <!-- Tab 内容区 -->
      <div class="tab-content">
        <!-- 概览 -->
        <div v-if="activeTab === 'overview'" class="tab-pane">
          <div class="content-placeholder">
            运行状态（单卡片纵向排列）
          </div>
        </div>

        <!-- AT 终端 -->
        <div v-else-if="activeTab === 'at'" class="tab-pane">
          <ModuleAtTerminal
            :device-id="detail.id"
            :backend-mode="detail.backend_mode"
            :at-port="detail.at_port"
            :running="detail.running"
          />
        </div>

        <!-- USSD -->
        <div v-else-if="activeTab === 'ussd'" class="tab-pane">
          <ModuleUssdTerminal
            :device-id="detail.id"
            :vowifi-active="detail.vowifi_enabled"
          />
        </div>

        <!-- 卡策略 -->
        <div v-else-if="activeTab === 'card'" class="tab-pane">
          <ModuleCardPolicy
            :device-id="detail.id"
            :iccid="detail.modem?.iccid"
            :policy="cardPolicy"
            :device-online="detail.running"
            @policy-changed="onCardPolicyChanged"
          />
        </div>

        <!-- 配置 -->
        <div v-else-if="activeTab === 'config'" class="tab-pane">
          <ModuleConfigForm :device-id="detail.id" :device="detail" />
        </div>
      </div>
        </div>
      </div>
    </div>

    <!-- 加载/空状态 -->
    <ListSkeleton v-else-if="loading && !detail" :rows="3" />
    <EmptyState
      v-else
      title="选择一个设备"
      subtitle="从左侧列表选择设备查看详情"
    />
  </div>
</template>

<style scoped>
@import '../assets/button/Reset-vowifi.css';

.module-detail-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  padding: 0;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--card);
  overflow: hidden;
}

/* 头部 — 60px 统一高度 */
.detail-header {
  height: 60px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 16px;
  border-bottom: 1px solid var(--border);
  overflow: hidden;
}

.detail-header-narrow {
  display: none;
  flex: 1;
  align-items: center;
  gap: 8px;
}

.detail-header-narrow .el-select {
  flex: 1;
}

.add-btn-narrow {
  flex-shrink: 0;
}

.detail-header-wide {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;
}

/* 图标盒子 */
.detail-header-icon-box {
  width: 38px;
  height: 38px;
  border-radius: 6px;
  background: var(--background);
  border: 1px solid var(--border);
  color: var(--foreground);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  font-weight: 700;
  font-size: 16px;
}

.detail-header-info {
  flex: 1;
  min-width: 0;
}

.detail-header-name {
  font-size: 15px;
  font-weight: 700;
  color: var(--foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.detail-header-imei {
  font-size: 12px;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
}

.detail-header-plmn {
  font-size: 12px;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
}

.detail-header-code {
  font-family: var(--oomol-font-mono);
  color: var(--brand);
  opacity: 0.8;
}

.detail-header-flag {
  opacity: 0.9;
}

.detail-header-country {
  opacity: 0.7;
}

.detail-header-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 1px;
  font-size: 12px;
  color: var(--muted-foreground);
}

.activation-status {
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
  background: var(--muted);
  color: var(--muted-foreground);
}

.detail-content {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.edit-area-wrap {
  flex: 1;
  min-height: 0;
  padding: 10px;
}

/* 第二层容器 */
.detail-inner {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--card);
  overflow: hidden;
}

/* WiFi Calling 行 */
.vowifi-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 14px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.vowifi-row-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.vowifi-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
}

.vowifi-status-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: var(--muted-foreground);
  opacity: 0.3;
}

.vowifi-status-dot.on {
  background: var(--brand);
  opacity: 1;
}

.vowifi-status-text {
  font-size: 12px;
  color: var(--muted-foreground);
}

/* Tab 切换 */
.tab-bar {
  display: flex;
  gap: 2px;
  padding: 6px 14px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.tab-item {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 5px 12px;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--muted-foreground);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.12s;
}

.tab-item:hover {
  background: var(--accent);
  color: var(--foreground);
}

.tab-item.active {
  background: var(--background);
  border-color: var(--border);
  color: var(--foreground);
  box-shadow: var(--console-shadow-sm);
}

/* Tab 内容区 */
.tab-content {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 14px;
}

.tab-pane {
  display: flex;
  flex-direction: column;
  gap: 10px;
  height: 100%;
  min-height: 0;
}

.content-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 200px;
  color: var(--muted-foreground);
  font-size: 13px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--muted);
}

/* 响应式：窄屏显示下拉选择器 */
@media (max-width: 768px) {
  .detail-header-narrow {
    display: flex;
  }
  .detail-header-wide {
    display: none;
  }
}
</style>
