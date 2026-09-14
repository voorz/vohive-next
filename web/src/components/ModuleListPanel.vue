<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useDevicesStore } from '../stores/devices'
import { primaryLifecycleStatus, isRadioRegistered, isControlOnline } from '../utils/deviceLifecycle'
import type { DeviceMgmtListItem } from '../types/api'
import ListSkeleton from './ListSkeleton.vue'
import {
  Add24Regular,
  Search24Regular,
  UsbStick20Regular,
  WifiOff24Regular,
  WifiWarning24Filled,
  ArrowSort24Regular
} from '@vicons/fluent'
import { WifiCalling3Round, SignalCellularNoSimTwotone, Md3GMobiledataSharp, Md4GMobiledataSharp, Md4GPlusMobiledataSharp, Md5GSharp } from '@vicons/material'
import { Airplane, HardwareChipSharp } from '@vicons/ionicons5'
import { loadPlmnCatalog } from '../composables/plmn-catalog'
import { downloadIcon, getCachedIcon } from '../composables/useOperatorIcon'
import { useEventStream } from '../composables/useEventStream'

const props = defineProps<{
  selectedId?: string
}>()

const emit = defineEmits<{
  'open-search': []
  'select': [id: string]
}>()

const store = useDevicesStore()
const { list, loading } = storeToRefs(store)

const searchText = ref('')

// SSE 实时设备列表流
const { connect: connectStream, disconnect: disconnectStream } = useEventStream<{ devices: DeviceMgmtListItem[] }>({
  path: '/devices/stream',
  eventName: 'devices',
  parse: (payload: string) => JSON.parse(payload),
  onEvent: (data) => {
    if (data.devices) {
      store.setList(data.devices)
    }
  }
})

onMounted(() => {
  // SSE 实时订阅设备列表
  connectStream()
  // 加载 PLMN catalog 并下载运营商图标
  loadPlmnCatalog().then(async () => {
    for (const d of list.value) {
      const mcc = d.modem?.native_mcc || ''
      const mnc = d.modem?.native_mnc || ''
      if (mcc && mnc && !(await getCachedIcon(mcc, mnc, d.modem?.native_spn))) {
        downloadIcon(mcc, mnc, d.modem?.native_spn).then(result => {
          if (result) {
            window.dispatchEvent(new CustomEvent('vohive-icon-updated', { detail: { mcc, mnc } }))
          }
        })
      }
    }
  })
})
onUnmounted(() => {
  disconnectStream()
})

const filteredDevices = computed(() => {
  const q = searchText.value.trim().toLowerCase()
  if (!q) return list.value
  return list.value.filter(d =>
    d.name.toLowerCase().includes(q) ||
    (d.modem?.imei || '').includes(q) ||
    d.id.toLowerCase().includes(q)
  )
})

function handleSelect(id: string) {
  emit('select', id)
}

// 主要状态文本（设备状态：在线/离线/启动中等，不含网络模式）
function primaryStatusText(d: DeviceMgmtListItem): string {
  return primaryLifecycleStatus(d).label
}

// 状态标签类型（随设备状态变化）
function statusTagType(d: DeviceMgmtListItem): 'success' | 'warning' | 'danger' | 'info' {
  return primaryLifecycleStatus(d).tag
}

// 次要状态文本（如 WiFi-Calling / 运营商·网络模式）
function secondaryStatusText(d: DeviceMgmtListItem): string {
if (d?.vowifi_enabled) {
const state = vowifiState(d)
if (state === 'ready') return 'WiFi-Calling 已就绪'
// 显示实时状态
const rt = d?.vowifi_runtime
if (rt?.phase === 'recover_failed') {
  // recover_failed 时卡片只显示倒计时，错误原因在设备概览页展示
  if (rt?.retry_in_seconds && rt.retry_in_seconds > 0) return `${rt.retry_in_seconds}s 后重试`
  return 'VoWiFi 启动失败'
}
if (rt?.stage_label) return rt.stage_label
if (rt?.last_reason) return rt.last_reason
return 'WiFi-Calling 启动中...'
}
  // PC/SC 读卡器无 modem，不具备驻网能力
  if (d?.esim_transport === 'pcsc') return '未启用'
  if (isRadioRegistered(d)) {
    const op = d?.modem?.operator || '--'
    const mode = [d?.modem?.network_duplex, d?.modem?.network_mode].filter(Boolean).join(' ') || '--'
    return `${op} · ${mode}`
  }
  if (!isControlOnline(d)) return '控制面恢复中'
  if (d.registration_state_label === 'searching') return '搜索网络中'
  if (d.registration_state_label === 'denied') return '驻网被拒'
  return '未驻网'
}

// 卡片状态背景色：仅两种 — 品牌色（已驻网/已注册/VoWiFi已注册）或素色（其他）
function cardToneClass(d: DeviceMgmtListItem): string {
  // VoWiFi 已注册（全部就绪）
  if (d?.vowifi_enabled) {
    const rt = d?.vowifi_runtime
    if (rt) {
      const all = [rt.sim_ready, rt.access_ready, rt.tunnel_ready, rt.ims_ready, rt.sms_ready, rt.call_ready]
      if (all.every(Boolean)) return 'tone-brand'
    }
    return 'tone-neutral'
  }
  // 蜂窝已驻网/已注册
  if (isRadioRegistered(d)) return 'tone-brand'
  return 'tone-neutral'
}

// 信号强度格式化
function signalText(d: DeviceMgmtListItem): string {
  const dbm = d?.modem?.signal_dbm
  if (dbm === undefined || dbm === null) return ''
  // PC/SC 读卡器无 modem，信号为 0 时显示“No Modem”
  if (d?.esim_transport === 'pcsc' && dbm === 0) return 'No Modem'
  return `${dbm}dBm`
}

// 信号强度胶囊颜色 class（根据强度值变化）
function signalPillClass(d: DeviceMgmtListItem): string {
  const dbm = d?.modem?.signal_dbm
  if (dbm === undefined || dbm === null) return 'pill-info'
  if (dbm === 0 || dbm === -999 || !Number.isFinite(dbm)) return 'pill-info'
  if (d?.registration_state_label === 'searching') return 'pill-warning'
  if (d?.registration_state_label === 'denied') return 'pill-danger'
  if (dbm >= -85) return 'pill-success'
  if (dbm >= -100) return 'pill-warning'
  return 'pill-danger'
}

// 信号格数（与详情页统一：5 格）
function signalBars(dbm?: number): number {
  if (dbm === undefined || dbm === null || dbm === 0 || dbm === -999 || !Number.isFinite(dbm)) return 0
  if (dbm >= -75) return 5
  if (dbm >= -85) return 4
  if (dbm >= -95) return 3
  if (dbm >= -105) return 2
  return 1
}

// 信号格颜色（与详情页统一）
function signalBarColor(d: DeviceMgmtListItem): string {
  const dbm = d?.modem?.signal_dbm
  if (dbm === undefined || dbm === null) return ''
  // 搜索网络中 → 橙色
  if (d?.registration_state_label === 'searching') return 'bar-warning'
  // 驻网被拒 → 红色
  if (d?.registration_state_label === 'denied') return 'bar-danger'
  if (dbm >= -85) return 'bar-good'
  if (dbm >= -100) return 'bar-fair'
  return 'bar-poor'
}

// 飞行模式
function isFlightMode(d: DeviceMgmtListItem): boolean {
  return !!d?.flight_mode
}

// VoWiFi 状态
function vowifiState(d: DeviceMgmtListItem): 'off' | 'enabled-not-ready' | 'ready' {
  if (!d?.vowifi_enabled) return 'off'
  const rt = d?.vowifi_runtime
  if (!rt) return 'enabled-not-ready'
  const all = [rt.sim_ready, rt.access_ready, rt.tunnel_ready, rt.ims_ready, rt.sms_ready, rt.call_ready]
  if (all.every(Boolean)) return 'ready'
  return 'enabled-not-ready'
}

function simIconColor(d: DeviceMgmtListItem): string {
  if (d.euicc_available === true) return 'var(--brand)'
  if (d.euicc_available === false) return 'var(--destructive)'
  return 'var(--muted-foreground)'
}

// VoWiFi 6格就绪状态
function readinessItems(d: DeviceMgmtListItem) {
  const rt = d?.vowifi_runtime
  return [
    { key: 'SIM',    ready: rt?.sim_ready },
    { key: 'Access', ready: rt?.access_ready },
    { key: 'Tunnel', ready: rt?.tunnel_ready },
    { key: 'IMS',    ready: rt?.ims_ready },
    { key: 'SMS',    ready: rt?.sms_ready },
    { key: 'Call',   ready: rt?.call_ready },
  ]
}

// 设备状态图标（替代品牌图标，按优先级判断）
function deviceStatusIcon(d: DeviceMgmtListItem) {
  // PC/SC 读卡器：无 modem，只判断 VoWiFi 状态
  if (d?.esim_transport === 'pcsc') {
    if (vowifiState(d) === 'ready') return WifiCalling3Round
    if (vowifiState(d) === 'enabled-not-ready') return WifiWarning24Filled
    return null
  }
  // 模组：VoWiFi 已就绪 → 最高优先级
  if (vowifiState(d) === 'ready') return WifiCalling3Round
  // VoWiFi 启用但未就绪
  if (vowifiState(d) === 'enabled-not-ready') return WifiWarning24Filled
  // 未驻网
  if (!isRadioRegistered(d)) return SignalCellularNoSimTwotone
  // 按网络制式
  const mode = String(d?.modem?.network_mode || '').toUpperCase()
  if (mode.includes('NR5G') || mode.includes('5G')) return Md5GSharp
  if (mode.includes('LTE') && mode.includes('PLUS')) return Md4GPlusMobiledataSharp
  if (mode.includes('LTE') || mode.includes('4G')) return Md4GMobiledataSharp
  if (mode.includes('UMTS') || mode.includes('GSM') || mode.includes('CDMA') || mode.includes('3G')) return Md3GMobiledataSharp
  return null
}

// 设备类型标签：读卡器 / 4G模组 / 5G模组
function deviceTypeTag(d: DeviceMgmtListItem): string {
  if (d?.esim_transport === 'pcsc') return '读卡器'
  if (d?.modem?.nr5g_bands && d.modem.nr5g_bands.length > 0) return '5G模组'
  return '4G模组'
}

</script>

<template>
  <div class="module-list-panel">
    <!-- 搜索栏 -->
    <div class="list-search">
      <el-input
        v-model="searchText"
        placeholder="搜索设备 / IMEI"
        clearable
        autocomplete="off"
      >
        <template #prefix>
          <el-icon><Search24Regular /></el-icon>
        </template>
      </el-input>
    </div>

    <!-- 设备列表 -->
    <div class="list-scroll">
      <ListSkeleton v-if="loading && list.length === 0" :rows="4" />

      <div v-else class="device-cards">
        <div
          v-for="item in filteredDevices"
          :key="item.id"
          class="vohive-rattlesnake-parent"
          :class="[
            { selected: item.id === props.selectedId },
            cardToneClass(item)
          ]"
          @click="handleSelect(item.id)"
        >
          <div class="vohive-rattlesnake-card">
            <!-- 顶部：状态图标 -->
            <div class="vohive-rattlesnake-top-bar">
              <div class="vohive-rattlesnake-icons">
                <el-icon v-if="isFlightMode(item)" size="20" class="vohive-rattlesnake-airplane">
                  <Airplane />
                </el-icon>
                <template v-else>
                  <el-icon v-if="item.network_enabled && item.data_connected" size="20" class="vohive-rattlesnake-data" title="移动数据已连接">
                    <ArrowSort24Regular />
                  </el-icon>
                  <div v-if="signalBars(item.modem?.signal_dbm) > 0 && item.esim_transport !== 'pcsc'" class="vohive-rattlesnake-signal-bars" title="信号强度">
                    <div
                      v-for="i in 5"
                      :key="i"
                      class="vohive-rattlesnake-signal-bar"
                      :class="[
                        signalBars(item.modem?.signal_dbm) >= i ? signalBarColor(item) : '',
                        { dim: signalBars(item.modem?.signal_dbm) < i }
                      ]"
                    />
                  </div>
                  <el-icon v-else-if="item.esim_transport === 'pcsc'" size="20" class="vohive-rattlesnake-usb" :class="{ offline: !item.running || !item.healthy }">
                    <UsbStick20Regular />
                  </el-icon>
                </template>
                <el-icon size="20" class="vohive-rattlesnake-sim" :style="{ color: simIconColor(item) }" :title="item.euicc_available === true ? 'eUICC 可用' : item.euicc_available === false ? 'eUICC 不可用' : 'eUICC 状态未知'">
                  <HardwareChipSharp />
                </el-icon>
                <el-icon size="20" class="vohive-rattlesnake-vowifi" :class="vowifiState(item)">
                  <WifiCalling3Round v-if="vowifiState(item) === 'ready'" />
                  <WifiWarning24Filled v-else-if="vowifiState(item) === 'enabled-not-ready'" />
                  <WifiOff24Regular v-else />
                </el-icon>
              </div>
              <span class="vohive-rattlesnake-type-tag" :class="{ 'type-reader': item.esim_transport === 'pcsc', 'type-5g': item.esim_transport !== 'pcsc' && item.modem?.nr5g_bands && item.modem.nr5g_bands.length > 0 }">{{ deviceTypeTag(item) }}</span>
            </div>
            <div class="vohive-rattlesnake-content-box">
              <!-- 设备名 -->
              <span class="vohive-rattlesnake-card-title">{{ item.name }}</span>
              <!-- 设备信息（分行） -->
              <p class="vohive-rattlesnake-card-content">
                <span class="vohive-rattlesnake-card-id">{{ item.id }}</span><br>
                <span class="vohive-rattlesnake-card-status">{{ secondaryStatusText(item) }}</span><br>
                <span
                  v-if="item.modem?.signal_dbm !== undefined && item.modem?.signal_dbm !== null"
                  class="vohive-rattlesnake-signal-pill"
                  :class="signalPillClass(item)"
                >{{ signalText(item) }}</span>
                <span class="vohive-rattlesnake-status-pill" :class="'pill-' + statusTagType(item)">{{ primaryStatusText(item) }}</span>
              </p>
              <!-- VoWiFi 6格就绪进度条（底部） -->
              <div v-if="item.vowifi_enabled" class="vohive-rattlesnake-readiness">
                <div
                  v-for="ri in readinessItems(item)"
                  :key="ri.key"
                  class="vohive-rattlesnake-readiness-bar"
                  :class="{ ready: ri.ready === true, 'not-ready': ri.ready === false }"
                />
              </div>
              <!-- 设备状态图标（居右下，替代品牌图标） -->
              <el-icon v-if="deviceStatusIcon(item)" size="64" class="vohive-rattlesnake-device-icon">
                <component :is="deviceStatusIcon(item)" />
              </el-icon>
            </div>
          </div>
        </div>
        <!-- 虚线占位添加区 -->
        <div class="add-device-placeholder" @click="emit('open-search')">
          <el-icon size="32"><Add24Regular /></el-icon>
          <span class="add-device-text">添加设备</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../assets/card/hungry-rattlesnake-3.css';

.module-list-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--card);
  overflow: hidden;
}

.list-search {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 60px;
  padding: 0 12px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.list-search .el-input {
  flex: 1;
}

.list-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 12px;
}

.device-cards {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

/* ===== 覆盖原始 CSS 的固定宽度，适配左栏 ===== */
.vohive-rattlesnake-parent {
  width: 100%;
  cursor: pointer;
}

/* ===== 强制卡片宽高比 ===== */
.vohive-rattlesnake-card {
  overflow: hidden;
  display: flex;
  flex-direction: column;
  position: relative;
}

/* ===== content-box 填满 card 除 padding-top 外的剩余空间 ===== */
.vohive-rattlesnake-content-box {
  flex: 1;
  position: relative;
  padding: 0 12px 12px 12px;
}

/* ===== 选中状态 ===== */
.vohive-rattlesnake-parent.selected .vohive-rattlesnake-card {
border-color: rgba(0, 188, 125, 0.6);
}

/* ===== 状态背景色 (仅两种：品牌色/素色) ===== */
.vohive-rattlesnake-parent.tone-brand .vohive-rattlesnake-content-box {
  background: rgba(0, 188, 125, 0.12);
}

.vohive-rattlesnake-parent.tone-neutral .vohive-rattlesnake-content-box {
  background: rgba(255, 255, 255, 0.03);
}

/* ===== 选中时 card border 加强 ===== */
.vohive-rattlesnake-parent.tone-brand.selected .vohive-rattlesnake-card {
  border-color: rgba(0, 188, 125, 0.6);
}

.vohive-rattlesnake-parent.tone-neutral.selected .vohive-rattlesnake-card {
  border-color: #4b5563;
}

/* ===== 左上角图标区 ===== */
.vohive-rattlesnake-icons {
  display: flex;
  align-items: center;
  gap: 4px;
}

.vohive-rattlesnake-airplane {
  color: #da9f00;
  flex-shrink: 0;
}

.vohive-rattlesnake-data {
  color: #00bc7d;
  opacity: 0.8;
  flex-shrink: 0;
}

.vohive-rattlesnake-sim {
  flex-shrink: 0;
  transition: color 0.15s;
}

.vohive-rattlesnake-vowifi {
  color: #999999;
  opacity: 0.4;
  flex-shrink: 0;
}

.vohive-rattlesnake-vowifi.ready {
  color: #00bc7d;
  opacity: 1;
}

.vohive-rattlesnake-vowifi.enabled-not-ready {
  color: #ff3b30;
  opacity: 1;
}

.vohive-rattlesnake-usb {
  color: #00bc7d;
  flex-shrink: 0;
}

.vohive-rattlesnake-usb.offline {
  color: #ff3b30;
}

/* ===== 信号格（适配绿色背景，用深色） ===== */
.vohive-rattlesnake-signal-bars {
  display: flex;
  align-items: flex-end;
  gap: 1px;
  height: 20px;
  padding: 2px;
  flex-shrink: 0;
}

.vohive-rattlesnake-signal-bar {
  width: 4px;
  border-radius: 1px;
  transition: all 0.3s;
}

.vohive-rattlesnake-signal-bar:nth-child(1) { height: 20%; }
.vohive-rattlesnake-signal-bar:nth-child(2) { height: 40%; }
.vohive-rattlesnake-signal-bar:nth-child(3) { height: 60%; }
.vohive-rattlesnake-signal-bar:nth-child(4) { height: 80%; }
.vohive-rattlesnake-signal-bar:nth-child(5) { height: 100%; }

/* 信号格颜色（固定暗色模式） */
.vohive-rattlesnake-signal-bar.bar-good {
  background: #00bc7d;
}

.vohive-rattlesnake-signal-bar.bar-fair {
  background: #da9f00;
}

.vohive-rattlesnake-signal-bar.bar-warning {
  background: #da9f00;
}

.vohive-rattlesnake-signal-bar.bar-danger {
  background: #ff3b30;
}

.vohive-rattlesnake-signal-bar.bar-poor {
  background: #ff3b30;
}

.vohive-rattlesnake-signal-bar.dim {
  background: #999999;
  opacity: 0.2;
}

/* ===== 设备状态图标（content-box 内，垂直居中，若隐若现） ===== */
.vohive-rattlesnake-device-icon {
  position: absolute;
  top: 50%;
  right: 12px;
  transform: translateY(-50%);
  width: 64px;
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  margin: 0;
  opacity: 0.1;
  z-index: 5;
}

/* ===== 顶部状态栏：图标 ===== */
.vohive-rattlesnake-top-bar {
  position: absolute;
  top: 12px;
  left: 12px;
  right: 12px;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
}


/* ===== 设备名 ===== */
.vohive-rattlesnake-content-box .vohive-rattlesnake-card-title {
  display: inline-block;
  transform: none !important;
}

.vohive-rattlesnake-content-box .vohive-rattlesnake-card-title:hover {
  transform: none !important;
}

/* ===== 设备信息弱化文本 ===== */
.vohive-rattlesnake-card-content {
  margin-bottom: 8px;
  line-height: 20px;
}

.vohive-rattlesnake-card-content .vohive-rattlesnake-card-id {
  color: rgba(235, 235, 235, 0.45);
  font-size: 11px;
}

.vohive-rattlesnake-card-content .vohive-rattlesnake-card-status {
  color: rgba(235, 235, 235, 0.6);
  font-size: 11px;
}

/* ===== 胶囊标签：类型（top-bar 内，右侧） ===== */
.vohive-rattlesnake-type-tag {
  flex-shrink: 0;
  height: 20px;
  display: flex;
  align-items: center;
  padding: 0 6px;
  border-radius: 0;
  font-size: 9px;
  font-weight: 600;
  white-space: nowrap;
  background: rgba(0, 188, 125, 0.2);
  color: #00bc7d;
  border: 1px solid rgba(0, 188, 125, 0.3);
  backdrop-filter: blur(4px);
}

.vohive-rattlesnake-type-tag.type-reader {
  background: rgba(218, 159, 0, 0.2);
  color: #da9f00;
  border: 1px solid rgba(218, 159, 0, 0.3);
}

.vohive-rattlesnake-type-tag.type-5g {
  background: rgba(168, 85, 247, 0.2);
  color: #a855f7;
  border: 1px solid rgba(168, 85, 247, 0.3);
}

/* 状态胶囊标签 */
.vohive-rattlesnake-status-pill {
display: inline-flex;
align-items: center;
height: 20px;
padding: 0 8px;
border-radius: 9999px;
font-size: 10px;
font-weight: 600;
white-space: nowrap;
}

.vohive-rattlesnake-status-pill.pill-success {
  background: rgba(0, 188, 125, 0.2);
  color: #00bc7d;
  border: 1px solid rgba(0, 188, 125, 0.3);
}

.vohive-rattlesnake-status-pill.pill-warning {
  background: rgba(218, 159, 0, 0.2);
  color: #da9f00;
  border: 1px solid rgba(218, 159, 0, 0.3);
}

.vohive-rattlesnake-status-pill.pill-danger {
  background: rgba(255, 59, 48, 0.2);
  color: #ff3b30;
  border: 1px solid rgba(255, 59, 48, 0.3);
}

.vohive-rattlesnake-status-pill.pill-info {
  background: rgba(255, 255, 255, 0.1);
  color: #999999;
  border: 1px solid rgba(255, 255, 255, 0.2);
}

/* ===== 信号强度胶囊标签 ===== */
.vohive-rattlesnake-signal-pill {
display: inline-flex;
align-items: center;
height: 20px;
padding: 0 6px;
border-radius: 9999px;
font-size: 10px;
font-weight: 600;
font-family: monospace;
white-space: nowrap;
margin-right: 4px;
}

.vohive-rattlesnake-signal-pill.pill-success {
  background: rgba(0, 188, 125, 0.2);
  color: #00bc7d;
  border: 1px solid rgba(0, 188, 125, 0.3);
}

.vohive-rattlesnake-signal-pill.pill-warning {
  background: rgba(218, 159, 0, 0.2);
  color: #da9f00;
  border: 1px solid rgba(218, 159, 0, 0.3);
}

.vohive-rattlesnake-signal-pill.pill-danger {
  background: rgba(255, 59, 48, 0.2);
  color: #ff3b30;
  border: 1px solid rgba(255, 59, 48, 0.3);
}

.vohive-rattlesnake-signal-pill.pill-info {
  background: rgba(255, 255, 255, 0.1);
  color: #999999;
  border: 1px solid rgba(255, 255, 255, 0.2);
}

/* ===== VoWiFi 6格就绪进度条（底部） ===== */
.vohive-rattlesnake-readiness {
  position: absolute;
  bottom: 12px;
  left: 12px;
  right: 12px;
  display: flex;
  gap: 3px;
}

.vohive-rattlesnake-readiness-bar {
  flex: 1;
  height: 4px;
  border-radius: 2px;
  background: rgba(255, 255, 255, 0.1);
}

.vohive-rattlesnake-readiness-bar.ready {
  background: #00bc7d;
}

.vohive-rattlesnake-readiness-bar.not-ready {
  background: #ff3b30;
}

/* ===== 虚线占位添加区 ===== */
.add-device-placeholder {
height: 90px;
display: flex;
flex-direction: column;
align-items: center;
justify-content: center;
gap: 4px;
border: 2px dashed var(--border);
border-radius: 4px;
background: var(--muted);
cursor: pointer;
transition: all 0.2s;
color: var(--muted-foreground);
}

.add-device-text {
font-size: 12px;
font-weight: 500;
}

.add-device-placeholder:hover {
  border-color: rgba(0, 188, 125, 0.4);
  background: rgba(0, 188, 125, 0.05);
  color: #00bc7d;
}
</style>
