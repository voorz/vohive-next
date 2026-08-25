<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useEventStream } from '../composables/useEventStream'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useDevicesStore } from '../stores/devices'
import { devicesService } from '../services/devices'
import { isWwanQmiControlPath } from '../utils/deviceBackend'
import { getDeviceIcon } from '../utils/deviceIcon'
import type { DiscoveredDevice, DeviceConfigDTO } from '../types/api'
import {
  Search24Regular,
  ArrowSync24Regular,
  Add24Regular,
  PortMicroUsb24Regular
} from '@vicons/fluent'
import { systemService } from '../services/system'

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
onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  disconnectDiscoveryStream()
})

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

// 生成唯一设备名：若已有同名设备则自动加序号 -01/-02
function uniqueDeviceName(base: string): string {
  if (!base) return base
  const existingNames = new Set(store.list.map(d => d.name))
  if (!existingNames.has(base)) return base
  let seq = 1
  while (existingNames.has(`${base}-${String(seq).padStart(2, '0')}`)) {
    seq++
  }
  return `${base}-${String(seq).padStart(2, '0')}`
}

// 运行模式
const deviceBackend = ref<'at' | 'qmi' | 'mbim'>('at')

// 是否 PC/SC 设备
const isPCSC = computed(() => selectedDevice.value?.type === 'pcsc')

// 发现列表中是否存在 PC/SC 读卡器（用于控制驱动配置菜单的显示）
const hasPCSCReader = computed(() => discovered.value.some(d => d.type === 'pcsc'))

// 正在添加
const adding = ref(false)

// 驱动模式开关（true=原生 PC/SC 驱动, false=内置 USBFS 直连）
const useNativeDriver = ref(false)
const driverModeSaving = ref(false)

// PC/SC 驱动检测
const pcscDriverStatus = ref<{ pcscd_installed: boolean; libccid_installed: boolean; pcscd_active: boolean; all_ready: boolean; message: string } | null>(null)
const pcscDriverLoading = ref(false)
const pcscInstalling = ref(false)

async function checkPcscDriver() {
  pcscDriverLoading.value = true
  try {
    const res = await systemService.getPcscDriverStatus()
    if (res.ok) {
      pcscDriverStatus.value = res.data
    } else {
      pcscDriverStatus.value = null
    }
  } catch {
    pcscDriverStatus.value = null
  }
  pcscDriverLoading.value = false
}

// 加载当前驱动模式
async function loadDriverMode() {
  try {
    const res = await systemService.getServerConfig()
    if (res.ok) {
      const mode = res.data.pcsc_driver_mode || ''
      useNativeDriver.value = mode === 'pcscd' || mode === 'pcsc'
    }
  } catch { /* keep defaults */ }
}

// 切换驱动模式
// 开启：仅保存配置，显示驱动配置菜单（安装/启动由各按钮二次确认控制）
// 关闭：二次确认 → 停止 pcscd + 保存配置 → 切换为内置 USBFS 驱动
async function toggleDriverMode(val: string | number | boolean) {
  const enabled = Boolean(val)
  if (enabled) {
    // 开启：只保存配置 + 显示驱动菜单，不弹成功提示
    driverModeSaving.value = true
    try {
      const res = await systemService.saveServerConfig('7575', false, 'pcscd')
      if (!res.ok) throw new Error(res.error?.message || '保存失败')
      useNativeDriver.value = true
      await checkPcscDriver()
    } catch (e: any) {
      useNativeDriver.value = false
      ElMessage.error(e.message || '保存配置失败')
    } finally {
      driverModeSaving.value = false
    }
    return
  }
  // 关闭：二次确认
  const confirmed = await ElMessageBox.confirm(
    '关闭后将停止 pcscd 服务并切换到内置 USBFS 驱动。确定继续？',
    '切换驱动模式',
    { confirmButtonText: '确认切换', cancelButtonText: '取消', type: 'warning' }
  ).then(() => true).catch(() => false)
  if (!confirmed) {
    useNativeDriver.value = true // 恢复开关状态
    return
  }
  driverModeSaving.value = true
  try {
    // 先停止 pcscd
    try {
      await systemService.stopPcscDriver()
    } catch {
      // 停止失败不阻断
    }
    // 再保存配置
    const res = await systemService.saveServerConfig('7575', false, 'usbfs')
    if (!res.ok) throw new Error(res.error?.message || '保存失败')
    useNativeDriver.value = false
    pcscDriverStatus.value = null
    ElMessage.success('已切换到内置 USBFS 驱动模式')
  } catch (e: any) {
    useNativeDriver.value = true
    ElMessage.error(e.message || '切换失败')
  } finally {
    driverModeSaving.value = false
  }
}

async function installPcscDriver() {
  const confirmed = await ElMessageBox.confirm(
    '将安装 pcscd 和 libccid 驱动包。安装期间读卡器可能短暂不可用。确定继续？',
    '安装 PC/SC 驱动',
    { confirmButtonText: '安装', cancelButtonText: '取消', type: 'warning' }
  ).then(() => true).catch(() => false)
  if (!confirmed) return
  pcscInstalling.value = true
  try {
    const res = await systemService.installPcscDriver()
    if (res.ok && res.data.result) {
      pcscDriverStatus.value = res.data.result
      if (res.data.result.all_ready) {
        ElMessage.success('PC/SC 驱动安装成功')
        await scanDevices(false)
      } else {
        ElMessage.warning(res.data.result.message || '安装可能未完成')
      }
    } else {
      ElMessage.error('驱动安装失败')
    }
  } catch {
    ElMessage.error('驱动安装失败')
  }
  pcscInstalling.value = false
}

async function stopPcscDriver() {
  const confirmed = await ElMessageBox.confirm(
    '停止 pcscd 后读卡器将不可用，直到重新启动。确定停止？',
    '停止 pcscd',
    { confirmButtonText: '停止', cancelButtonText: '取消', type: 'warning' }
  ).then(() => true).catch(() => false)
  if (!confirmed) return
  try {
    const res = await systemService.stopPcscDriver()
    if (res.ok && res.data.result) {
      pcscDriverStatus.value = res.data.result
      ElMessage.success('pcscd 服务已停止')
    } else {
      ElMessage.error('停止 pcscd 失败')
    }
  } catch {
    ElMessage.error('停止 pcscd 失败')
  }
}

async function startPcscDriver() {
  const confirmed = await ElMessageBox.confirm(
    '启动 pcscd 后将通过原生 PC/SC 驱动连接读卡器。确定启动？',
    '启动 pcscd',
    { confirmButtonText: '启动', cancelButtonText: '取消', type: 'info' }
  ).then(() => true).catch(() => false)
  if (!confirmed) return
  try {
    const res = await systemService.startPcscDriver()
    if (res.ok && res.data.result) {
      pcscDriverStatus.value = res.data.result
    } else {
      ElMessage.error('启动 pcscd 失败')
    }
  } catch {
    ElMessage.error('启动 pcscd 失败')
  }
}

// 过滤已发现设备
const filteredDevices = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return discovered.value
  return discovered.value.filter(d =>
    (d.imei || '').includes(q) ||
    d.at_port.toLowerCase().includes(q) ||
    d.driver_name.toLowerCase().includes(q) ||
    d.usb_path.toLowerCase().includes(q) ||
    (d.net_interface || '').toLowerCase().includes(q) ||
    (d.manufacturer || '').toLowerCase().includes(q) ||
    (d.model || '').toLowerCase().includes(q) ||
    (d.chip_vendor || '').toLowerCase().includes(q) ||
    (d.firmware || '').toLowerCase().includes(q)
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
// showSuccess=true: 用户点击“重新扫描”按钮，触发后端 rescanAll（重扫+重连）
// showSuccess=false: 弹窗打开时，只轻量读取已发现设备列表，不触发 rescan
async function scanDevices(showSuccess = false) {
  if (showSuccess) {
    // 用户主动点击重新扫描 → 触发后端 rescanAll
    scanning.value = true
    try {
      await devicesService.rescanAll()
      const result = await store.fetchDiscovered()
      if (result.ok) {
        discovered.value = store.discovered
        ElMessage.success('设备重新扫描完成')
      }
    } catch {
      ElMessage.error('扫描设备失败')
    }
    scanning.value = false
  } else {
    // 弹窗打开 → 只轻量读取已发现设备
    scanning.value = true
    try {
      const result = await store.fetchDiscovered()
      if (result.ok) {
        discovered.value = store.discovered
      }
    } catch {
      // 静默失败
    }
    scanning.value = false
  }
}

// SSE 监听设备发现事件（弹窗打开时连接，关闭时断开）
const { connect: connectDiscoveryStream, disconnect: disconnectDiscoveryStream } = useEventStream<unknown>({
  path: '/devices/stream',
  eventName: '__none__', // 不监听 devices 事件（ModuleListPanel 已在监听）
  parse: () => null,
  onEvent: () => {},
  onRawEvent: (eventName: string) => {
    // 收到 discovered 事件 → 刷新发现列表（不触发后端 rescan，避免死循环）
    if (eventName === 'discovered') {
      refreshDiscovered()
    }
  }
})

// 仅刷新发现列表数据，不触发后端 rescan
async function refreshDiscovered() {
  try {
    const result = await store.fetchDiscovered()
    if (result.ok) {
      discovered.value = store.discovered
    }
  } catch {
    // 静默失败
  }
}

// 弹窗打开时自动扫描 + 启动 SSE 监听（不显示提示）
watch(() => props.modelValue, async (open) => {
  if (open) {
    searchQuery.value = ''
    selectedKey.value = ''
    await scanDevices(false)
    await loadDriverMode()
    if (useNativeDriver.value) {
      checkPcscDriver()
    }
    // 启动 SSE 监听，热插拔后自动刷新发现列表
    connectDiscoveryStream()
  } else {
    // 弹窗关闭时断开 SSE
    disconnectDiscoveryStream()
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
  // PC/SC 设备 ID 生成：后端通过 crc32(usb_path) 生成建议ID
  if (d.type === 'pcsc') {
    deviceId.value = d.suggested_id || `pcsc-${(d.serial || 'reader').replace(/[^a-zA-Z0-9]/g, '').slice(-8) || 'reader'}`
    // PC/SC: 优先 display_name（USB Product），回退 serial
    deviceName.value = uniqueDeviceName(d.display_name || d.serial || 'reader')
    return
  }
  deviceId.value = d.imei ? `modem-${d.imei.slice(-4)}` : (d.net_interface || d.at_port.split('/').pop() || d.at_port)
  // 模组: manufacturer（QMI/AT 来源）+ model 拼接，禁止回退 USB Product
  const brand = d.manufacturer || ''
  const model = d.model || ''
  deviceName.value = uniqueDeviceName(brand && model ? `${brand}-${model}` : brand)

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
    interface: d.net_interface || '',
    at_port: d.at_port || '',
    control_device: d.control_path || '',
    modem_imei: d.imei || '',
    usb_path: d.usb_path || '',
    device_backend: isPCSC.value ? 'at' : deviceBackend.value,
    esim_transport: isPCSC.value ? 'pcsc' : 'at',
    pcsc_usb_path: isPCSC.value ? (d.pcsc_usb_path || '') : undefined,
    network_enabled: !isPCSC.value,
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
  if (m === 'pcsc') return 'PC/SC'
  if (m === 'ecm') return 'ECM'
  if (m === 'rndis') return 'RNDIS'
  if (m === 'ncm') return 'NCM'
  return 'UNKNOWN'
}

// VID:PID 格式化
function vidPid(d: DiscoveredDevice): string {
  return `0x${d.vendor_id.toString(16).padStart(4, '0')}:0x${d.product_id.toString(16).padStart(4, '0')}`
}

// 从读卡器名称中提取括号内的序列号
// 例如 "ESTKme-RED (2051315E5056) 00 00" → "2051315E5056"
function extractReaderSN(reader: string): string {
  const m = reader.match(/\(([^)]+)\)/)
  return m ? m[1].trim() : ''
}

// mode 标签样式类
function modeTagClass(mode?: string): string {
  const m = String(mode || 'unknown').toLowerCase()
  if (m === 'qmi') return 'mode-qmi'
  if (m === 'mbim') return 'mode-mbim'
  if (m === 'pcsc') return 'mode-pcsc'
  if (m === 'ecm') return 'mode-ecm'
  if (m === 'rndis') return 'mode-rndis'
  if (m === 'ncm') return 'mode-ncm'
  return 'mode-unknown'
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
        clearable
      >
        <template #prefix>
          <el-icon><Search24Regular /></el-icon>
        </template>
      </el-input>
      <el-button type="primary" @click="scanDevices(true)" :disabled="scanning" class="!ml-2">
        <el-icon class="mr-1"><ArrowSync24Regular /></el-icon>
        <span>重新扫描</span>
      </el-button>
    </div>

    <!-- 驱动模式开关（仅有 PC/SC 读卡器时显示） -->
    <div v-if="hasPCSCReader" class="driver-mode-switch">
      <div class="driver-mode-info">
        <span class="driver-mode-title">使用原生PC/SC驱动</span>
        <span class="driver-mode-desc">应用已内置读卡器驱动，仅在遇到euicc兼容性问题时请开启</span>
      </div>
      <el-switch
        v-model="useNativeDriver"
        :loading="driverModeSaving"
        @change="toggleDriverMode"
      />
    </div>

    <!-- PC/SC 驱动管理卡片（仅原生模式时显示） -->
    <div v-if="useNativeDriver && pcscDriverStatus" class="pcsc-driver-card" :class="pcscDriverStatus.all_ready ? 'ready' : 'not-ready'">
      <div class="pcsc-driver-info">
        <div class="pcsc-driver-text">
          <span class="pcsc-driver-title">原生 PC/SC 驱动</span>
          <span class="pcsc-driver-status-line">
            <span class="driver-dot" :class="pcscDriverStatus.pcscd_active ? 'dot-on' : 'dot-off'"></span>
            pcscd: {{ pcscDriverStatus.pcscd_installed ? (pcscDriverStatus.pcscd_active ? '运行中' : '已安装未运行') : '未安装' }}
            <span class="driver-dot" :class="pcscDriverStatus.libccid_installed ? 'dot-on' : 'dot-off'"></span>
            libccid: {{ pcscDriverStatus.libccid_installed ? '已安装' : '未安装' }}
          </span>
        </div>
      </div>
      <div class="footer-actions">
        <el-button size="small" :loading="pcscInstalling" @click="installPcscDriver"
          :disabled="pcscDriverStatus.pcscd_installed && pcscDriverStatus.libccid_installed">
          {{ pcscDriverStatus.pcscd_installed && pcscDriverStatus.libccid_installed ? '已安装' : '安装' }}
        </el-button>
        <el-button size="small"
          :disabled="!pcscDriverStatus.pcscd_installed || !pcscDriverStatus.libccid_installed"
          @click="pcscDriverStatus.pcscd_active ? stopPcscDriver() : startPcscDriver()">
          {{ pcscDriverStatus.pcscd_active ? '停止' : '启动' }}
        </el-button>
      </div>
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
          <div class="discovered-card-icon">
            <img :src="getDeviceIcon({ type: d.type, manufacturer: d.manufacturer })" alt="device" class="discovered-card-icon-svg" />
          </div>

          <!-- 信息 -->
          <div class="discovered-card-info">
            <!-- 第一行：设备名称（加粗） -->
            <!-- 模组：manufacturer（QMI/AT 来源，如 Quectel）；PC/SC：display_name（USB Product，如 ESTKme-RED） -->
            <div class="discovered-card-name">
              <span class="meta-label">名称</span>
              <span class="device-name-text">{{ d.type === 'pcsc' ? (d.display_name || d.serial || '--') : (d.manufacturer || '--') }}</span>
              <span class="device-mode-tag" :class="modeTagClass(d.mode)">{{ modeText(d.mode) }}</span>
              <span v-if="d.degraded" class="status-tag status-degraded">降级</span>
              <span v-else-if="d.configured" class="status-tag status-added">已添加</span>
              <span v-else class="status-tag status-new">新设备</span>
            </div>
            <!-- 第二行：厂商 -->
            <div v-if="d.manufacturer" class="discovered-card-meta">
              <span class="meta-label">厂商</span>
              <span class="meta-item">{{ d.manufacturer }}</span>
            </div>
            <!-- 第三行：型号 -->
            <div v-if="d.model" class="discovered-card-meta">
              <span class="meta-label">型号</span>
              <span class="meta-item">{{ d.model }}</span>
            </div>
            <!-- 第四行：芯片厂商 -->
            <div v-if="d.chip_vendor" class="discovered-card-meta">
              <span class="meta-label">芯片</span>
              <span class="meta-item">{{ d.chip_vendor }}</span>
            </div>
            <!-- 第五行：标识信息 -->
            <div class="discovered-card-meta">
              <span class="meta-label">信息</span>
              <span v-if="d.imei" class="meta-item">IMEI: {{ d.imei }}</span>
              <span v-if="d.serial" class="meta-item">SN: {{ d.serial }}</span>
              <span v-if="d.vendor_id" class="meta-item">USB: {{ vidPid(d) }}</span>
            </div>
            <!-- 第六行：固件 -->
            <div v-if="d.firmware" class="discovered-card-meta">
              <span class="meta-label">固件</span>
              <span class="meta-item">{{ d.firmware }}</span>
            </div>
            <!-- 第七行：接口 -->
            <div v-if="d.type !== 'pcsc'" class="discovered-card-meta">
              <span class="meta-label">接口</span>
              <span v-if="d.at_port" class="meta-item">AT: {{ d.at_port }}</span>
              <span v-if="d.control_path" class="meta-item">CTL: {{ d.control_path }}</span>
              <span v-if="d.net_interface" class="meta-item">NET: {{ d.net_interface }}</span>
            </div>
            <!-- 第八行：USB 技术信息 -->
            <div v-if="d.info" class="discovered-card-meta">
              <span class="meta-label">USB</span>
              <span class="meta-item">{{ d.info }}</span>
            </div>
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
      <!-- PC/SC 设备提示 -->
      <div v-if="isPCSC" class="config-row">
        <div class="config-label">类型</div>
        <div class="config-input">
          <el-tag size="small" type="success">PC/SC 读卡器</el-tag>
          <span class="ml-2 text-xs text-gray-500">纯 eSIM 管理设备，无 modem 功能</span>
        </div>
      </div>
      <!-- Modem 后端模式选择 -->
      <div v-if="!isPCSC" class="config-row">
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

/* 驱动模式开关 */
.driver-mode-switch {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  border: 1px solid var(--border);
  border-radius: 6px;
  margin-bottom: 12px;
  background: var(--muted);
}

.driver-mode-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.driver-mode-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
}

.driver-mode-desc {
  font-size: 11px;
  color: var(--muted-foreground);
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
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.discovered-card {
  display: flex;
  align-items: stretch;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--muted);
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

/* 图标容器：固定正方形，不受卡片内容高度影响 */
.discovered-card-icon {
  width: 48px;
  height: 48px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.discovered-card-icon-svg {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.discovered-card-info {
  flex: 1;
  min-width: 0;
}

.discovered-card-name {
  font-size: 13px;
  line-height: 13px;
  font-weight: 600;
  color: var(--foreground);
  display: flex;
  align-items: center;
  gap: 6px;
}

.device-name-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
}

/* mode 标签 */
.device-mode-tag {
  flex-shrink: 0;
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.5px;
  white-space: nowrap;
}

.mode-qmi { background: color-mix(in oklab, var(--brand) 15%, transparent); color: var(--brand); }
.mode-mbim { background: color-mix(in oklab, var(--info, #3b82f6) 15%, transparent); color: var(--info, #3b82f6); }
.mode-pcsc { background: color-mix(in oklab, var(--warning) 15%, transparent); color: var(--warning); }
.mode-ecm { background: color-mix(in oklab, var(--muted-foreground) 15%, transparent); color: var(--muted-foreground); }
.mode-rndis { background: color-mix(in oklab, var(--muted-foreground) 15%, transparent); color: var(--muted-foreground); }
.mode-ncm { background: color-mix(in oklab, var(--muted-foreground) 15%, transparent); color: var(--muted-foreground); }
.mode-unknown { background: var(--muted); color: var(--muted-foreground); }

/* info 行 */
.discovered-card-info-line {
  margin-top: 0;
  font-size: 11px;
  line-height: 13px;
  font-family: var(--oomol-font-mono);
  color: var(--muted-foreground);
  opacity: 0.6;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.discovered-card-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 0;
  font-size: 11px;
  line-height: 13px;
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

.meta-label {
  font-size: 10px;
  font-weight: 600;
  color: var(--muted-foreground);
  opacity: 0.5;
  flex-shrink: 0;
  margin-right: 2px;
}

/* 状态标签 */
.status-tag {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 3px;
  font-size: 10px;
  font-weight: 600;
  white-space: nowrap;
}

.status-new {
  background: color-mix(in oklab, var(--brand) 15%, transparent);
  color: var(--brand);
}

.status-added {
  background: color-mix(in oklab, var(--muted-foreground) 15%, transparent);
  color: var(--muted-foreground);
  border: 1px solid color-mix(in oklab, var(--muted-foreground) 25%, transparent);
}

.status-degraded {
  background: color-mix(in oklab, var(--destructive) 15%, transparent);
  color: var(--destructive);
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

/* PC/SC 驱动检测卡片 */
.pcsc-driver-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  border-radius: 6px;
  margin-bottom: 12px;
  border: 1px solid var(--border);
}

.pcsc-driver-card.ready {
  background: color-mix(in oklab, var(--brand) 6%, var(--card));
  border-color: color-mix(in oklab, var(--brand) 20%, var(--border));
}

.pcsc-driver-card.not-ready {
  background: color-mix(in oklab, var(--destructive) 5%, var(--card));
  border-color: color-mix(in oklab, var(--destructive) 20%, var(--border));
}

.pcsc-driver-info {
  display: flex;
  align-items: center;
  gap: 10px;
}

.pcsc-driver-icon.is-ready {
  color: var(--brand);
}

.pcsc-driver-icon:not(.is-ready) {
  color: var(--destructive);
}

.pcsc-driver-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.pcsc-driver-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
}

.pcsc-driver-status-line {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 11px;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
}

.pcsc-driver-detail {
  font-size: 11px;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
  display: flex;
  align-items: center;
  gap: 8px;
}

.driver-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.driver-dot.dot-on {
  background: var(--brand);
}

.driver-dot.dot-off {
  background: var(--destructive);
}

.pcsc-driver-ready-text {
  font-size: 12px;
  font-weight: 600;
  color: var(--brand);
}
</style>
