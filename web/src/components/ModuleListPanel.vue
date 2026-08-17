<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { storeToRefs } from 'pinia'
import { ElMessage } from 'element-plus'
import { useDevicesStore } from '../stores/devices'
import { primaryLifecycleStatus, isRadioRegistered, isControlOnline } from '../utils/deviceLifecycle'
import type { DeviceMgmtListItem } from '../types/api'
import ListSkeleton from './ListSkeleton.vue'
import EmptyState from './EmptyState.vue'
import {
  Add24Regular,
  Search24Regular,
  UsbStick20Regular,
  Wifi124Regular,
  WifiOff24Regular,
  WifiWarning24Filled,
  ArrowSort24Regular
} from '@vicons/fluent'
import { WifiCalling3Round } from '@vicons/material'
import { Airplane } from '@vicons/ionicons5'
import { loadPlmnCatalog } from '../composables/plmn-catalog'
import { downloadIcon, getCachedIcon } from '../composables/useOperatorIcon'
import { useEventStream } from '../composables/useEventStream'
import ModemIcon from '../assets/svgs/modem.svg'
import ReaderIcon from '../assets/svgs/reader.svg'

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
  loadPlmnCatalog().then(() => {
    for (const d of list.value) {
      const mcc = d.modem?.native_mcc || ''
      const mnc = d.modem?.native_mnc || ''
      if (mcc && mnc && !getCachedIcon(mcc, mnc, d.modem?.native_spn)) {
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

// 次要状态文本（如 WiFi-Calling / 运营商·网络模式）
function secondaryStatusText(d: DeviceMgmtListItem): string {
  if (d?.vowifi_enabled) {
    const state = vowifiState(d)
    if (state === 'ready') return 'WiFi-Calling 已就绪'
    return 'WiFi-Calling 未就绪'
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

// 状态标签类型
function statusTagType(d: DeviceMgmtListItem): 'success' | 'warning' | 'danger' | 'info' {
  return primaryLifecycleStatus(d).tag
}

// 卡片状态背景色
function cardToneClass(d: DeviceMgmtListItem): string {
  const tone = primaryLifecycleStatus(d).tone
  if (tone === 'success') return 'tone-success'
  if (tone === 'warning') return 'tone-warning'
  if (tone === 'danger') return 'tone-danger'
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

// 信号强度颜色
function signalClass(d: DeviceMgmtListItem): string {
  const dbm = d?.modem?.signal_dbm
  if (dbm === undefined || dbm === null) return ''
  if (d?.esim_transport === 'pcsc' && dbm === 0) return 'no-modem'
  if (dbm >= -70) return 'good'
  if (dbm >= -90) return 'fair'
  return 'poor'
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

// 设备名首字母
function initials(name: string): string {
  return name.charAt(0).toUpperCase()
}
</script>

<template>
  <div class="module-list-panel">
    <!-- 搜索栏 + 添加按钮 -->
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
      <el-button size="small" type="primary" @click="emit('open-search')" class="!border-0 add-btn">
        <el-icon class="mr-1"><Add24Regular /></el-icon>
        <span>添加</span>
      </el-button>
    </div>

    <!-- 设备列表 -->
    <div class="list-scroll">
      <ListSkeleton v-if="loading && list.length === 0" :rows="4" />

      <EmptyState
        v-else-if="filteredDevices.length === 0"
        title="暂无设备"
        subtitle="点击「添加」扫描并添加设备"
      />

      <div v-else class="device-cards">
        <div
          v-for="item in filteredDevices"
          :key="item.id"
          class="device-card"
          :class="[
            { selected: item.id === props.selectedId },
            cardToneClass(item)
          ]"
          @click="handleSelect(item.id)"
        >
          <img :src="item.esim_transport === 'pcsc' ? ReaderIcon : ModemIcon" :alt="item.esim_transport === 'pcsc' ? 'reader' : 'modem'" class="device-card-icon-svg" />
          <div class="device-card-info">
            <!-- 第一行：WiFi图标 + 设备名 + 状态标签 -->
            <div class="device-card-name-row">
              <span class="device-card-name">{{ item.name }}</span>
              <!-- 飞行模式图标 -->
              <el-icon v-if="isFlightMode(item)" size="16" class="device-card-airplane-icon">
                <Airplane />
              </el-icon>
              <template v-else>
                <!-- 移动数据图标（网络开启且有数据连接时显示） -->
                <el-icon v-if="item.network_enabled && item.data_connected" size="16" class="device-card-data-icon" title="移动数据已连接">
                  <ArrowSort24Regular />
                </el-icon>
                <!-- 信号格（模组）-->
                <div v-if="signalBars(item.modem?.signal_dbm) > 0 && item.esim_transport !== 'pcsc'" class="signal-bars" title="信号强度">
                  <div
                    v-for="i in 5"
                    :key="i"
                    class="signal-bar"
                    :class="[
                      signalBars(item.modem?.signal_dbm) >= i ? signalBarColor(item) : '',
                      { dim: signalBars(item.modem?.signal_dbm) < i }
                    ]"
                  />
                </div>
                <!-- USB 图标（读卡器，无信号格）-->
                <el-icon v-else-if="item.esim_transport === 'pcsc'" size="16" class="device-card-usb-icon" :class="{ offline: !item.running || !item.healthy }">
                  <UsbStick20Regular />
                </el-icon>
              </template>
              <el-icon size="16" class="device-card-vowifi-icon" :class="vowifiState(item)">
                <WifiCalling3Round v-if="vowifiState(item) === 'ready'" />
                <WifiWarning24Filled v-else-if="vowifiState(item) === 'enabled-not-ready'" />
                <WifiOff24Regular v-else />
              </el-icon>
              <el-tag size="small" :type="statusTagType(item)">{{ primaryStatusText(item) }}</el-tag>
            </div>
            <!-- 第二行：interface + 信号 -->
            <div class="device-card-meta">
              <span class="device-card-identifier">{{ item.id }}</span>
              <span
                v-if="item.modem?.signal_dbm !== undefined && item.modem?.signal_dbm !== null"
                class="device-card-signal"
                :class="signalClass(item)"
              >{{ signalText(item) }}</span>
            </div>
            <!-- 第三行：次要状态 -->
            <div class="device-card-meta2">
              <span class="device-card-sub-status">{{ secondaryStatusText(item) }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
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

.add-btn {
  flex-shrink: 0;
}

.list-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 6px;
}

.device-cards {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.device-card {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.12s, border-color 0.12s;
}

.device-card:hover {
  background: var(--accent);
}

/* 状态背景色 — 亮色模式 (Tailwind emerald/amber/red) */
.device-card.tone-success {
  background: rgba(236, 253, 245, 0.7);
  border-color: #a7f3d0;
}

.device-card.tone-warning {
  background: rgba(255, 251, 235, 0.7);
  border-color: #fde68a;
}

.device-card.tone-danger {
  background: rgba(254, 242, 242, 0.7);
  border-color: #fecaca;
}

.device-card.tone-neutral {
  background: rgba(249, 250, 251, 0.7);
  border-color: #f3f4f6;
}

/* 状态背景色 — 暗色模式 */
html.dark .device-card.tone-success {
  background: rgba(16, 185, 129, 0.1);
  border-color: rgba(16, 185, 129, 0.2);
}

html.dark .device-card.tone-warning {
  background: rgba(245, 158, 11, 0.1);
  border-color: rgba(245, 158, 11, 0.2);
}

html.dark .device-card.tone-danger {
  background: rgba(239, 68, 68, 0.1);
  border-color: rgba(239, 68, 68, 0.2);
}

html.dark .device-card.tone-neutral {
  background: rgba(255, 255, 255, 0.05);
  border-color: rgba(255, 255, 255, 0.1);
}

/* 选中时 — 亮色 */
.device-card.tone-success.selected {
  border-color: #10b981;
}

.device-card.tone-warning.selected {
  border-color: #f59e0b;
}

.device-card.tone-danger.selected {
  border-color: #ef4444;
}

.device-card.tone-neutral.selected {
  border-color: #d1d5db;
}

/* 选中时 — 暗色 */
html.dark .device-card.tone-success.selected {
  border-color: #34d399;
}

html.dark .device-card.tone-warning.selected {
  border-color: #fbbf24;
}

html.dark .device-card.tone-danger.selected {
  border-color: #f87171;
}

html.dark .device-card.tone-neutral.selected {
  border-color: #4b5563;
}

.device-card-icon-svg {
  width: 38px;
  height: 38px;
  object-fit: contain;
  flex-shrink: 0;
}

.device-card-info {
  flex: 1;
  min-width: 0;
}

.device-card-name-row {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

/* 信号格（与详情页等比例缩放） */
.signal-bars {
  display: flex;
  align-items: flex-end;
  gap: 1px;
  height: 14px;
  flex-shrink: 0;
}

.signal-bar {
  width: 2px;
  border-radius: 1px;
  transition: all 0.3s;
}

.signal-bar:nth-child(1) { height: 20%; }
.signal-bar:nth-child(2) { height: 40%; }
.signal-bar:nth-child(3) { height: 60%; }
.signal-bar:nth-child(4) { height: 80%; }
.signal-bar:nth-child(5) { height: 100%; }

.signal-bar.bar-good {
  background: var(--brand);
}

.signal-bar.bar-fair {
  background: var(--warning);
}

.signal-bar.bar-warning {
  background: var(--warning);
}

.signal-bar.bar-danger {
  background: var(--destructive);
}

.signal-bar.bar-poor {
  background: var(--destructive);
}

.signal-bar.dim {
  background: var(--muted-foreground);
  opacity: 0.2;
}

.device-card-vowifi-icon {
  color: var(--muted-foreground);
  opacity: 0.4;
  flex-shrink: 0;
}

.device-card-usb-icon {
  color: var(--brand);
  flex-shrink: 0;
}

.device-card-usb-icon.offline {
  color: var(--destructive);
}

.device-card-airplane-icon {
  color: var(--warning);
  flex-shrink: 0;
}

.device-card-data-icon {
  color: var(--brand);
  opacity: 0.8;
  flex-shrink: 0;
}

.device-card-vowifi-icon.ready {
  color: var(--brand);
  opacity: 1;
}

.device-card-vowifi-icon.enabled-not-ready {
  color: var(--destructive);
  opacity: 1;
}

.device-card-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
}

.device-card-status-tag {
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 600;
  white-space: nowrap;
  flex-shrink: 0;
}

.device-card-status-tag.success {
  background: color-mix(in oklab, var(--brand) 15%, transparent);
  color: var(--brand);
}

.device-card-status-tag.warning {
  background: color-mix(in oklab, var(--warning) 15%, transparent);
  color: var(--warning);
}

.device-card-status-tag.danger {
  background: color-mix(in oklab, var(--destructive) 15%, transparent);
  color: var(--destructive);
}

.device-card-status-tag.info {
  background: var(--muted);
  color: var(--muted-foreground);
}

.device-card-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 1px;
}

.device-card-identifier {
  font-size: 11px;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-card-signal {
  font-size: 11px;
  font-family: var(--oomol-font-mono);
}

.device-card-signal.good {
  color: var(--brand);
}

.device-card-signal.fair {
  color: var(--warning);
}

.device-card-signal.warning {
  color: var(--warning);
}

.device-card-signal.danger {
  color: var(--destructive);
}

.device-card-signal.poor {
  color: var(--destructive);
  opacity: 0.7;
}

.device-card-signal.no-modem {
  color: var(--muted-foreground);
  opacity: 0.6;
}

.device-card-meta2 {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 1px;
}

.device-card-sub-status {
  font-size: 11px;
  color: var(--muted-foreground);
  opacity: 0.8;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
