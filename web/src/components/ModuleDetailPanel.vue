<script setup lang="ts">
import { computed, ref, watch, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useDevicesStore } from '../stores/devices'
import ListSkeleton from './ListSkeleton.vue'
import EmptyState from './EmptyState.vue'
import CountryFlag from './CountryFlag.vue'
import ModemIcon from '../assets/svgs/modem.svg'
import ReaderIcon from '../assets/svgs/reader.svg'
import ModuleAtTerminal from './ModuleAtTerminal.vue'
import ModuleUssdTerminal from './ModuleUssdTerminal.vue'
import ModuleCardPolicy from './ModuleCardPolicy.vue'
import ModuleConfigForm from './ModuleConfigForm.vue'
import ModuleSmsTab from './ModuleSmsTab.vue'
import ModuleOverviewTab from './ModuleOverviewTab.vue'
import ModuleVoiceTab from './ModuleVoiceTab.vue'
import { getPlmnInfo, loadPlmnInfo, type PlmnInfoEntry } from '../composables/plmn-info'
import { ArrowSync24Regular, Add24Regular } from '@vicons/fluent'
import { cardsService } from '../services/cards'
import type { CardPolicy } from '../types/api'
import { devicesService } from '../services/devices'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useOverviewStream } from '../composables/useOverviewStream'

const props = defineProps<{
  selectedId?: string
}>()

const emit = defineEmits<{
  'open-search': []
  'select': [id: string]
  'device-deleted': []
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
  return ''
})
const deviceDisplayName = computed(() => detail.value?.name || '未选择')
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

// PC/SC 读卡器设备：无 modem 控制面
const isPCSC = computed(() => detail.value?.esim_transport === 'pcsc')

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

const reconnectingVoWiFi = ref(false)
const rebooting = ref(false)
const rotating = ref(false)
const togglingVoWiFi = ref(false)

async function toggleVoWiFi(val: string | number | boolean) {
  if (!detail.value?.id) return
  const id = detail.value.id
  const enabled = !!val
  togglingVoWiFi.value = true
  try {
    const result = enabled
      ? await devicesService.enableVoWiFi(id)
      : await devicesService.disableVoWiFi(id)
    if (!result.ok) throw new Error(result.error.message || '操作失败')
    void store.fetchDetail(id).catch(() => {})
    void store.fetchList().catch(() => {})
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : '操作失败')
  } finally {
    togglingVoWiFi.value = false
  }
}

async function rotateIP() {
  if (!detail.value?.id) return
  const id = detail.value.id
  if (!detail.value?.network_connected) {
    ElMessage.warning('设备网络未连接，请先启动网络')
    return
  }
  const confirmed = await ElMessageBox.confirm(
    `确定对设备 ${id} 发起 IP 轮换？这将断开当前网络并重新获取 IP。`,
    '确认轮换 IP',
    { confirmButtonText: '立即轮换', cancelButtonText: '取消', type: 'warning' }
  ).then(() => true).catch(() => false)
  if (!confirmed) return

  rotating.value = true
  try {
    const result = await devicesService.rotateIP(id)
    if (!result.ok) throw new Error(result.error.message || '轮换失败')
    ElMessage.success('轮换请求已发送')
    void store.fetchDetail(id).catch(() => {})
    void store.fetchList().catch(() => {})
    setTimeout(() => {
      void store.fetchDetail(id).catch(() => {})
      void store.fetchList().catch(() => {})
    }, 1500)
  } catch (e: unknown) {
    if (e !== 'cancel' && e !== undefined) {
      ElMessage.error(e instanceof Error ? e.message : '轮换失败')
    }
  } finally {
    rotating.value = false
  }
}

async function rebootModem() {
  if (!detail.value?.id) return
  const id = detail.value.id
  const confirmed = await ElMessageBox.confirm(
    `确定对设备 ${id} 发送重启模组指令？设备将在此期间脱网和失联数秒。`,
    '确认重启',
    { confirmButtonText: '立即重启', cancelButtonText: '取消', type: 'warning' }
  ).then(() => true).catch(() => false)
  if (!confirmed) return

  rebooting.value = true
  try {
    const result = await devicesService.rebootModem(id)
    if (!result.ok) throw new Error(result.error.message || '指令下发失败')
    ElMessage.success('重启指令已送达，设备正在重新启动')
    void store.fetchDetail(id).catch(() => {})
    void store.fetchList().catch(() => {})
    setTimeout(() => {
      void store.fetchDetail(id).catch(() => {})
      void store.fetchList().catch(() => {})
    }, 5000)
  } catch (e: unknown) {
    if (e !== 'cancel' && e !== undefined) {
      ElMessage.error(e instanceof Error ? e.message : '指令下发失败')
    }
  } finally {
    rebooting.value = false
  }
}

async function reconnectVoWiFi() {
  if (!detail.value?.id) return
  const id = detail.value.id
  const confirmed = await ElMessageBox.confirm(
    `确定对设备 ${id} 发起 VoWiFi 环境的重新连接拨号？这将在后台重新注册 IMS 链路。`,
    '重连 VoWiFi',
    { confirmButtonText: '确定重连', cancelButtonText: '取消', type: 'info' }
  ).then(() => true).catch(() => false)
  if (!confirmed) return

  reconnectingVoWiFi.value = true
  try {
    const result = await devicesService.reconnectVoWiFi(id)
    if (!result.ok) throw new Error(result.error.message || '重连请求失败')
    ElMessage.success('已触发重连指令，VoWiFi 服务正在重启...')
    void store.fetchDetail(id).catch(() => {})
    void store.fetchList().catch(() => {})
    setTimeout(() => {
      void store.fetchDetail(id).catch(() => {})
      void store.fetchList().catch(() => {})
    }, 4000)
  } catch (e: unknown) {
    if (e !== 'cancel' && e !== undefined) {
      ElMessage.error(e instanceof Error ? e.message : '重连请求失败')
    }
  } finally {
    reconnectingVoWiFi.value = false
  }
}

// 当前 Tab
const activeTab = ref('overview')

// Tab 列表（PC/SC 设备隐藏 AT/USSD）
const allTabs = [
  { name: 'overview', label: '概览' },
  { name: 'voice', label: '通话' },
  { name: 'sms', label: '短信' },
  { name: 'at', label: 'AT' },
  { name: 'ussd', label: 'USSD' },
  { name: 'card', label: '控制' },
  { name: 'config', label: '配置' }
]
const tabs = computed(() =>
  isPCSC.value ? allTabs.filter(t => t.name !== 'at' && t.name !== 'ussd') : allTabs
)

// 切换设备时若当前 Tab 已被隐藏，回退到概览
watch([tabs, () => detail.value?.id], () => {
  if (!tabs.value.some(t => t.name === activeTab.value)) {
    activeTab.value = 'overview'
  }
})

function initials(name: string): string {
  return name.charAt(0).toUpperCase()
}

function onDeviceDeleted() {
  emit('device-deleted')
}

// ---- SSE Overview Stream + 实时流量 ----
const { trafficSpeedRx, trafficSpeedTx, rollingMinuteRx, rollingMinuteTx } = useOverviewStream({
  deviceId: () => props.selectedId,
  detail,
})
</script>

<template>
  <div class="module-detail-panel">
    <!-- 详情头部 (60px) -->
    <div v-if="detail" class="detail-header">
      <!-- 窄屏下拉选择器 + 添加按钮 + 重启模组 -->
      <div class="detail-header-narrow">
        <img :src="detail?.esim_transport === 'pcsc' ? ReaderIcon : ModemIcon" :alt="detail?.esim_transport === 'pcsc' ? 'reader' : 'modem'" class="device-icon-svg narrow-logo" />
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
        <el-button type="primary" @click="emit('open-search')" class="action-btn-narrow">
          <el-icon class="mr-1"><Add24Regular /></el-icon>
          <span>添加</span>
        </el-button>
        <button class="reboot-btn-custom" :disabled="rebooting || isPCSC" @click="rebootModem">
          <el-icon class="mr-1"><ArrowSync24Regular /></el-icon>
          <span>重启模组</span>
        </button>
      </div>
      <!-- 宽屏：图标盒子 + 设备名 + 详细信息 + 重启模组 -->
      <div class="detail-header-wide">
        <img :src="detail?.esim_transport === 'pcsc' ? ReaderIcon : ModemIcon" :alt="detail?.esim_transport === 'pcsc' ? 'reader' : 'modem'" class="device-icon-svg" />
        <div class="detail-header-info">
          <div class="detail-header-name">{{ deviceDisplayName }}</div>
          <div class="detail-header-meta">
            <span v-if="operatorName" class="detail-header-operator">{{ operatorName }}</span>
            <span v-if="nativePlmn" class="detail-header-plmn">{{ nativePlmn }}</span>
            <span v-if="countryCode" class="detail-header-code">+{{ countryCode }}</span>
            <CountryFlag v-if="countryIso" :iso="countryIso" :size="16" class="detail-header-flag" />
            <span v-if="countryName" class="detail-header-country">{{ countryName }}</span>
          </div>
        </div>
        <button class="reboot-btn-custom reboot-btn" :disabled="rebooting || isPCSC" @click="rebootModem">
          <el-icon class="mr-1"><ArrowSync24Regular /></el-icon>
          <span>重启模组</span>
        </button>
      </div>
    </div>

    <!-- Tab 切换 -->
    <div v-if="detail" class="tab-bar">
        <el-radio-group v-model="activeTab">
          <el-radio-button
            v-for="tab in tabs"
            :key="tab.name"
            :value="tab.name"
          >{{ tab.label }}</el-radio-button>
        </el-radio-group>
      </div>

      <!-- Tab 内容区 -->
    <div v-if="detail" class="tab-content">
        <!-- 概览 -->
        <div v-if="activeTab === 'overview'" class="tab-pane">
          <ModuleOverviewTab
            :device="detail"
            :traffic-speed-rx="trafficSpeedRx"
            :traffic-speed-tx="trafficSpeedTx"
            :traffic-minute-rx="rollingMinuteRx"
            :traffic-minute-tx="rollingMinuteTx"
            :is-p-c-s-c="isPCSC"
            :reconnecting-vo-wi-fi="reconnectingVoWiFi"
            :rotating="rotating"
            :toggling-vo-wi-fi="togglingVoWiFi"
            @reconnect-vowifi="reconnectVoWiFi"
            @rotate-ip="rotateIP"
            @toggle-vowifi="toggleVoWiFi"
          />
        </div>

        <!-- 通话 -->
        <div v-else-if="activeTab === 'voice'" class="tab-pane">
          <ModuleVoiceTab :device-id="detail.id" />
        </div>

        <!-- 短信 -->
        <div v-else-if="activeTab === 'sms'" class="tab-pane">
          <ModuleSmsTab :device-id="detail.id" />
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
            :is-p-c-s-c="isPCSC"
            @policy-changed="onCardPolicyChanged"
          />
        </div>

        <!-- 配置 -->
        <div v-else-if="activeTab === 'config'" class="tab-pane">
          <ModuleConfigForm :device-id="detail.id" :device="detail" @device-deleted="onDeviceDeleted" />
                </div>
    </div>

    <!-- 无选中设备时占位头部 -->
    <div v-else class="detail-header detail-header-empty">
      <span style="color: var(--muted-foreground); font-size: 13px;">未选择设备</span>
    </div>

    <!-- 加载/空状态 -->
    <ListSkeleton v-if="loading && !detail" :rows="3" />
    <EmptyState
      v-else-if="!detail"
      title="选择一个设备"
      subtitle="从左侧列表选择设备查看详情"
    />
  </div>
</template>

<style scoped>
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
  padding: 0 12px;
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

.action-btn-narrow {
  flex-shrink: 0;
}

.detail-header-wide {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;
}

.reboot-btn {
  flex-shrink: 0;
  margin-left: auto;
}

/* 重启模组按钮（透明红框，hover 反转） */
.reboot-btn-custom {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border: 1px solid #ef4444;
  border-radius: 6px;
  background: transparent;
  color: #ef4444;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.12s;
}
.reboot-btn-custom:hover:not(:disabled) {
  background: #ef4444;
  color: #fff;
}
.reboot-btn-custom:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* 图标盒子 */
.device-icon-svg {
  width: 38px;
  height: 38px;
  object-fit: contain;
  flex-shrink: 0;
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

.detail-header-operator {
  font-size: 12px;
  color: var(--foreground);
  opacity: 0.8;
  font-weight: 500;
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

/* Tab 切换 */
.tab-bar {
display: flex;
padding: 12px;
border-bottom: 1px solid var(--border);
flex-shrink: 0;
}
.tab-bar :deep(.el-radio-group) {
flex: 1;
width: 100%;
display: flex;
}
.tab-bar :deep(.el-radio-button) {
flex: 1;
min-width: 0;
}
.tab-bar :deep(.el-radio-button__inner) {
width: 100%;
text-align: center;
padding-left: 4px;
padding-right: 4px;
white-space: nowrap;
overflow: hidden;
text-overflow: ellipsis;
}

/* Tab 内容区 */
.tab-content {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 12px;
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

.detail-header-empty {
  justify-content: center;
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
