<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useDevicesStore } from '../stores/devices'
import { devicesService } from '../services/devices'
import { isWwanQmiControlPath } from '../utils/deviceBackend'
import type { DiscoveredDevice, DeviceConfigDTO } from '../types/api'
import {
  Search24Regular,
  ArrowSync24Regular,
  Add24Regular,
  Check24Regular,
  PortMicroUsb24Regular
} from '@vicons/fluent'

const props = defineProps<{
  modelValue: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  add: [deviceIds: string[]]
}>()

const store = useDevicesStore()

// 响应式宽度
const isNarrow = ref(typeof window !== 'undefined' && window.innerWidth <= 768)
const dialogWidth = computed(() => isNarrow.value ? '90%' : '680px')
function handleResize() {
  isNarrow.value = window.innerWidth <= 768
}
onMounted(() => window.addEventListener('resize', handleResize))
onUnmounted(() => window.removeEventListener('resize', handleResize))

// 搜索
const searchQuery = ref('')

// 扫描状态
const scanning = ref(false)
const discovered = ref<DiscoveredDevice[]>([])

// 选中设备
const selectedKey = ref('')
const selectedDevice = computed(() => discovered.value.find(d => d.discovery_key === selectedKey.value) || null)

// 设备ID
const deviceId = ref('')

// 设备名称（可选）
const deviceName = ref('')

// 运行模式
const deviceBackend = ref<'at' | 'qmi' | 'mbim'>('at')

// 正在添加
const adding = ref(false)

// 过滤已发现设备
const filteredDevices = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return discovered.value
  return discovered.value.filter(d =>
    (d.imei || '').includes(q) ||
    d.at_port.toLowerCase().includes(q) ||
    d.driver_name.toLowerCase().includes(q) ||
    d.usb_path.toLowerCase().includes(q) ||
    (d.net_interface || '').toLowerCase().includes(q)
  )
})

// 判断设备后端模式约束
const isQMIBackendOnly = computed(() => isWwanQmiControlPath(selectedDevice.value?.control_path))
const isMBIMBackendOnly = computed(() => String(selectedDevice.value?.mode || '').toLowerCase() === 'mbim')
const isBackendLocked = computed(() => isQMIBackendOnly.value || isMBIMBackendOnly.value)

// 后端模式说明
const backendHint = computed(() => {
  if (isQMIBackendOnly.value) return '此类 WWAN QMI 设备固定 QMI，AT 口仅用于终端'
  if (isMBIMBackendOnly.value) return '此类设备固定 MBIM，AT 口仅用于终端'
  return 'AT=传统串口 / QMI=纯 QMI'
})

// 扫描设备（showSuccess: 是否显示成功提示）
async function scanDevices(showSuccess = false) {
  scanning.value = true
  try {
    await devicesService.rescanAll()
    const result = await store.fetchDiscovered()
    if (result.ok) {
      discovered.value = store.discovered
      if (showSuccess) ElMessage.success('设备重新扫描完成')
    }
  } catch {
    ElMessage.error('扫描设备失败')
  }
  scanning.value = false
}

// 弹窗打开时自动扫描（不显示提示）
watch(() => props.modelValue, async (open) => {
  if (open) {
    searchQuery.value = ''
    selectedKey.value = ''
    await scanDevices(false)
  }
})

// 选中设备
function selectDevice(d: DiscoveredDevice) {
  if (d.degraded) {
    ElMessage.warning('无法读取该设备 IMEI（可能控制口挂死），暂不可添加')
    return
  }
  if (d.configured) return
  selectedKey.value = d.discovery_key
  deviceId.value = d.imei ? `modem-${d.imei.slice(-4)}` : (d.net_interface || d.at_port.split('/').pop() || d.at_port)
  deviceName.value = ''

  // 自动选择后端模式
  const mode = String(d.mode || '').toLowerCase()
  if (mode === 'mbim') {
    deviceBackend.value = 'mbim'
  } else if (isWwanQmiControlPath(d.control_path) || (mode === 'qmi' && d.control_path)) {
    deviceBackend.value = 'qmi'
  } else {
    deviceBackend.value = 'at'
  }
}

// 添加设备
async function handleAdd() {
  if (!selectedDevice.value) {
    ElMessage.warning('请选择一个设备')
    return
  }
  adding.value = true

  const d = selectedDevice.value
  const config: DeviceConfigDTO = {
    id: deviceId.value,
    name: deviceName.value || deviceId.value,
    interface: d.net_interface,
    at_port: d.at_port,
    control_device: d.control_path,
    modem_imei: d.imei,
    usb_path: d.usb_path,
    device_backend: deviceBackend.value,
    esim_transport: 'at',
    network_enabled: true,
    vowifi_enabled: false
  }

  try {
    const result = await devicesService.addManaged(config)
    if (result.ok) {
      const warning = result.data.warning
      const started = result.data.started
      if (warning) {
        ElMessage.warning(warning)
      } else if (started === true) {
        ElMessage.success('设备已添加并开始接管')
      } else {
        ElMessage.success('设备配置已添加')
      }
      d.configured = true
      d.configured_id = config.id
      emit('add', [config.id])
      selectedKey.value = ''
    }
  } catch {
    ElMessage.error('添加设备失败')
  }
  adding.value = false
}

function handleClose() {
  emit('update:modelValue', false)
}

// 设备模式标签
function modeText(mode?: string): string {
  const m = String(mode || 'unknown').toLowerCase()
  if (m === 'qmi') return 'QMI'
  if (m === 'mbim') return 'MBIM'
  if (m === 'ecm') return 'ECM'
  if (m === 'rndis') return 'RNDIS'
  if (m === 'ncm') return 'NCM'
  return 'UNKNOWN'
}

function modeColor(mode?: string): string {
  if (mode === 'qmi') return 'qmi'
  if (mode === 'mbim') return 'mbim'
  return 'other'
}

// 驱动名首字母
function driverInitial(driver: string): string {
  return driver.charAt(0).toUpperCase()
}

// VID:PID 格式化
function vidPid(d: DiscoveredDevice): string {
  return `0x${d.vendor_id.toString(16).padStart(4, '0')}:0x${d.product_id.toString(16).padStart(4, '0')}`
}
</script>

<template>
  <el-dialog
    :model-value="modelValue"
    @update:model-value="handleClose"
    title="添加设备"
    :width="dialogWidth"
    :close-on-click-modal="false"
    class="module-search-dialog"
  >
    <!-- 搜索框 + 重新扫描 -->
    <div class="search-bar">
      <el-input
        v-model="searchQuery"
        placeholder="搜索 IMEI / 接口 / 驱动 / 端口"
        size="large"
        clearable
      >
        <template #prefix>
          <el-icon><Search24Regular /></el-icon>
        </template>
      </el-input>
      <el-button size="large" @click="scanDevices(true)" :disabled="scanning" class="!ml-2">
        <el-icon class="mr-1"><ArrowSync24Regular /></el-icon>
        <span>重新扫描</span>
      </el-button>
    </div>

    <!-- 设备列表 -->
    <div class="search-results">
      <!-- 扫描中 -->
      <div v-if="scanning && discovered.length === 0" class="search-loading">
        <el-icon size="28" class="spin"><ArrowSync24Regular /></el-icon>
        <span>正在扫描设备...</span>
      </div>

      <!-- 无设备 -->
      <div v-else-if="filteredDevices.length === 0 && !searchQuery.trim()" class="search-empty">
        <el-icon size="28"><PortMicroUsb24Regular /></el-icon>
        <span>未发现可用设备</span>
        <span class="search-empty-sub">请确认模组已通过 USB 连接</span>
      </div>

      <!-- 搜索无结果 -->
      <div v-else-if="filteredDevices.length === 0" class="search-empty">
        <span>未找到匹配的设备</span>
      </div>

      <!-- 设备卡片列表 -->
      <div v-else class="device-list">
        <div
          v-for="d in filteredDevices"
          :key="d.discovery_key"
          class="discovered-card"
          :class="{
            configured: d.configured,
            degraded: d.degraded,
            selected: selectedKey === d.discovery_key
          }"
          @click="selectDevice(d)"
        >
          <!-- 图标 -->
          <div class="discovered-card-icon">{{ driverInitial(d.driver_name) }}</div>

          <!-- 信息 -->
          <div class="discovered-card-info">
            <div class="discovered-card-name">
              {{ d.net_interface || '--' }} · {{ d.driver_name || '--' }}
              <span class="meta-mode" :class="modeColor(d.mode)">{{ modeText(d.mode) }}</span>
            </div>
            <div class="discovered-card-meta">
              <span v-if="d.imei" class="meta-item">IMEI: {{ d.imei }}</span>
              <span class="meta-item">AT: {{ d.at_port || '--' }}</span>
              <span class="meta-item">{{ vidPid(d) }}</span>
              <span v-if="d.degraded" class="meta-degraded">降级</span>
            </div>
          </div>

          <!-- 状态 -->
          <div class="discovered-card-status">
            <el-icon v-if="d.configured" size="16" class="status-added"><Check24Regular /></el-icon>
            <el-icon v-else-if="d.degraded" size="16" class="status-degraded" />
            <el-icon v-else-if="selectedKey === d.discovery_key" size="16" class="status-selected"><Check24Regular /></el-icon>
          </div>
        </div>
      </div>
    </div>

    <!-- 选中设备配置区 -->
    <div v-if="selectedDevice && !selectedDevice.configured" class="add-config-area">
      <div class="config-row">
        <div class="config-label">设备ID</div>
        <el-input v-model="deviceId" size="small" placeholder="输入设备ID" class="config-input" />
      </div>
      <div class="config-row">
        <div class="config-label">设备名称<span class="config-label-optional">可选</span></div>
        <el-input v-model="deviceName" size="small" placeholder="留空则使用ID" class="config-input" />
      </div>
      <div class="config-row">
        <div class="config-label">
          <div class="config-label-title">运行模式</div>
          <div class="config-label-hint">{{ backendHint }}</div>
        </div>
        <el-select
          v-model="deviceBackend"
          size="small"
          :disabled="isBackendLocked"
          class="config-select"
        >
          <el-option v-if="!isMBIMBackendOnly" label="AT" value="at" :disabled="isQMIBackendOnly" />
          <el-option v-if="!isMBIMBackendOnly" label="QMI" value="qmi" :disabled="!selectedDevice.control_path" />
          <el-option v-if="isMBIMBackendOnly" label="MBIM" value="mbim" />
        </el-select>
      </div>
    </div>

    <!-- 底部 -->
    <template #footer>
      <div class="dialog-footer">
        <span class="device-count" v-if="discovered.length > 0">
          共 {{ discovered.length }} 个设备
        </span>
        <span v-else class="device-count placeholder" />
        <div class="footer-actions">
          <el-button @click="handleClose">取消</el-button>
      <el-button
        type="primary"
        :loading="adding"
        :disabled="!selectedDevice || selectedDevice.configured || !deviceId.trim()"
        @click="handleAdd"
        class="!border-0"
      >
            <el-icon class="mr-1"><Add24Regular /></el-icon>
            <span>添加</span>
          </el-button>
        </div>
      </div>
    </template>
  </el-dialog>
</template>

<style scoped>
.search-bar {
  display: flex;
  align-items: center;
  margin-bottom: 12px;
}

.search-bar .el-input {
  flex: 1;
}

.search-results {
  max-height: 40vh;
  min-height: 200px;
  overflow-y: auto;
  border: 1px solid var(--border);
  border-radius: 6px;
}

.search-loading,
.search-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 200px;
  color: var(--muted-foreground);
  font-size: 13px;
}

.search-empty-sub {
  font-size: 12px;
  opacity: 0.7;
}

.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* 设备列表 */
.device-list {
  padding: 4px;
}

.discovered-card {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid transparent;
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.12s, border-color 0.12s;
}

.discovered-card:hover {
  background: var(--accent);
}

.discovered-card.selected {
  background: color-mix(in oklab, var(--brand) 8%, var(--card));
  border-color: color-mix(in oklab, var(--brand) 30%, var(--border));
}

.discovered-card.configured {
  opacity: 0.5;
  cursor: default;
}

.discovered-card.degraded {
  cursor: not-allowed;
}

.discovered-card-icon {
  width: 32px;
  height: 32px;
  border-radius: 6px;
  background: var(--background);
  border: 1px solid var(--border);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
  flex-shrink: 0;
  margin-top: 2px;
}

.discovered-card-info {
  flex: 1;
  min-width: 0;
}

.discovered-card-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
  display: flex;
  align-items: center;
  gap: 6px;
}

.discovered-card-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 3px;
  font-size: 11px;
  color: var(--muted-foreground);
}

.discovered-card-path {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 2px;
  font-size: 11px;
  color: var(--muted-foreground);
  opacity: 0.7;
}

.meta-item {
  font-family: var(--oomol-font-mono);
}

.meta-mode {
  padding: 1px 5px;
  border-radius: 3px;
  font-weight: 600;
  text-transform: uppercase;
  font-size: 10px;
}

.meta-mode.qmi {
  background: color-mix(in oklab, var(--brand) 15%, transparent);
  color: var(--brand);
}

.meta-mode.mbim {
  background: color-mix(in oklab, var(--warning) 15%, transparent);
  color: var(--warning);
}

.meta-mode.other {
  background: var(--muted);
  color: var(--muted-foreground);
}

.meta-degraded {
  padding: 1px 5px;
  border-radius: 3px;
  font-weight: 600;
  font-size: 10px;
  background: color-mix(in oklab, var(--destructive) 15%, transparent);
  color: var(--destructive);
}

.discovered-card-status {
  flex-shrink: 0;
  margin-top: 2px;
}

.status-added {
  color: var(--muted-foreground);
}

.status-selected {
  color: var(--brand);
}

/* 配置区 */
.add-config-area {
  margin-top: 12px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--muted);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.config-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.config-label {
  flex-shrink: 0;
  min-width: 80px;
}

.config-label-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
}

.config-label-hint {
  font-size: 11px;
  color: var(--muted-foreground);
  margin-top: 1px;
}

.config-label-optional {
  font-size: 10px;
  font-weight: 400;
  color: var(--muted-foreground);
  margin-left: 4px;
}

.config-input {
  flex: 1;
}

.config-select {
  width: 120px;
}

/* 底部 */
.dialog-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.device-count {
  font-size: 13px;
  font-weight: 600;
  color: var(--brand);
}

.device-count.placeholder {
  width: 1px;
}

.footer-actions {
  display: flex;
  gap: 8px;
}
</style>
