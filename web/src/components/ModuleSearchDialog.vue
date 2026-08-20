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

// 是否 PC/SC 设备
const isPCSC = computed(() => selectedDevice.value?.type === 'pcsc')

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

// 切换驱动模式（仅保存配置 + 热切换，不自动 stop/start）
async function toggleDriverMode(val: string | number | boolean) {
  const enabled = Boolean(val)
  driverModeSaving.value = true
  try {
    const mode = enabled ? 'pcscd' : 'usbfs'
    const res = await systemService.saveServerConfig('7575', false, mode)
    if (!res.ok) throw new Error(res.error?.message || '保存失败')
    useNativeDriver.value = enabled
    if (enabled) {
      await checkPcscDriver()
    }
    ElMessage.success(enabled ? '已切换到原生 PC/SC 驱动模式' : '已切换到内置 USBFS 驱动模式')
  } catch (e: any) {
    useNativeDriver.value = !enabled
    ElMessage.error(e.message || '切换驱动模式失败')
  } finally {
    driverModeSaving.value = false
  }
}

async function installPcscDriver() {
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
    await loadDriverMode()
    if (useNativeDriver.value) {
      checkPcscDriver()
    }
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
  // PC/SC 设备 ID 生成：从读卡器名称中提取括号内的完整序列号
  if (d.type === 'pcsc') {
    const readerName = d.pcsc_reader || 'reader'
    // 从 USB identity 中提取完整 SN 作为 ID
    const sn = d.serial || extractReaderSN(readerName)
    deviceId.value = sn ? `pcsc-${sn}` : `pcsc-${readerName.replace(/[^a-zA-Z0-9]/g, '').slice(-8) || 'reader'}`
    deviceName.value = d.display_name || readerName
    return
  }
  deviceId.value = d.imei ? `modem-${d.imei.slice(-4)}` : (d.net_interface || d.at_port.split('/').pop() || d.at_port)
  deviceName.value = d.display_name || ''

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
    pcsc_reader: isPCSC.value ? (d.pcsc_reader || '') : undefined,
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

    <!-- 驱动模式开关 -->
    <div class="driver-mode-switch">
      <div class="driver-mode-info">
        <span class="driver-mode-title">使用原生读卡器驱动</span>
        <span class="driver-mode-desc">开启后通过系统 pcscd 服务驱动读卡器，关闭则使用内置 USBFS 直连</span>
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
        <el-button size="small" :loading="pcscInstalling" @click="installPcscDriver" :disabled="pcscDriverStatus.all_ready">
          {{ pcscDriverStatus.all_ready ? '已安装' : '安装' }}
        </el-button>
        <el-button size="small" @click="pcscDriverStatus.pcscd_active ? stopPcscDriver() : startPcscDriver()">
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
          <img v-if="d.type === 'pcsc'" src="../assets/svgs/reader.svg" alt="reader" class="discovered-card-icon-svg" />
          <img v-else src="../assets/svgs/modem.svg" alt="modem" class="discovered-card-icon-svg" />

          <!-- 信息 -->
          <div class="discovered-card-info">
            <!-- 名称行 -->
            <div class="discovered-card-name">
              <span class="device-name-text">{{ d.display_name || (d.type === 'pcsc' ? d.pcsc_reader : d.net_interface) || '--' }}</span>
              <span class="device-mode-tag" :class="modeTagClass(d.mode)">{{ modeText(d.mode) }}</span>
              <span v-if="d.degraded" class="status-tag status-degraded">降级</span>
              <span v-else-if="d.configured" class="status-tag status-added">已添加</span>
              <span v-else class="status-tag status-new">新设备</span>
            </div>
            <!-- 副标题 -->
            <div class="discovered-card-meta">
              <template v-if="d.type === 'pcsc'">
                <span v-if="d.imei" class="meta-item">IMEI: {{ d.imei }}</span>
                <span v-if="d.manufacturer" class="meta-item">{{ d.manufacturer }}</span>
                <span v-if="d.vendor_id" class="meta-item">USB: {{ vidPid(d) }}</span>
              </template>
              <template v-else>
                <span v-if="d.imei" class="meta-item">IMEI: {{ d.imei }}</span>
                <span v-if="d.manufacturer" class="meta-item">{{ d.manufacturer }}</span>
                <span class="meta-item">AT: {{ d.at_port || '--' }}</span>
                <span class="meta-item">{{ vidPid(d) }}</span>
              </template>
            </div>
            <!-- info 行 -->
            <div v-if="d.info" class="discovered-card-info-line">
              {{ d.info }}
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

.discovered-card-icon-svg {
  width: 40px;
  height: 40px;
  object-fit: contain;
  flex-shrink: 0;
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
  margin-top: 2px;
  font-size: 10px;
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
  background: var(--muted);
  color: var(--muted-foreground);
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
