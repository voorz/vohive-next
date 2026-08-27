<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Settings24Regular } from '@vicons/fluent'
import { devicesService } from '../services/devices'
import { useDevicesStore } from '../stores/devices'
import { storeToRefs } from 'pinia'
import { api } from '../stores/auth'
import type { DeviceConfigDTO, DeviceOverviewItem } from '../types/api'
import { isWwanQmiControlPath } from '../utils/deviceBackend'
import { ElMessage, ElMessageBox } from 'element-plus'

const props = defineProps<{
  deviceId: string
  device?: DeviceOverviewItem
}>()

const emit = defineEmits<{
  'device-deleted': []
}>()

const devicesStore = useDevicesStore()
const { config: storeConfig } = storeToRefs(devicesStore)

const editConfig = ref<DeviceConfigDTO | null>(null)
const editBaseline = ref('')
const editDirty = ref(false)
const saving = ref(false)
const deleting = ref(false)

watch(() => editConfig.value?.esim_transport, (val) => {
  // PC/SC 设备的 USB 路径在添加时已确定，编辑时只读展示
})

// 只读信息来自 device（运行时探测值优先）
const readonlyInfo = computed(() => [
  { label: '设备 ID', value: props.device?.id || props.deviceId },
  { label: '网卡接口', value: props.device?.interface || editConfig.value?.interface || '--' },
  { label: 'AT 端口', value: props.device?.at_port || editConfig.value?.at_port || '--' },
  { label: '控制设备', value: props.device?.control_device || editConfig.value?.control_device || '--' },
  { label: 'USB 路径', value: props.device?.usb_path || editConfig.value?.usb_path || '--' }
])

const activeControlDevice = computed(() => props.device?.control_device || editConfig.value?.control_device || '')
const isQMIBackendOnly = computed(() => isWwanQmiControlPath(activeControlDevice.value))
const isMBIMBackendOnly = computed(() => String(editConfig.value?.device_backend || '').toLowerCase() === 'mbim')

// 设备类型判断：esim_transport 为 pcsc 即为读卡器设备
const isPCSCDevice = computed(() => String(editConfig.value?.esim_transport || '').toLowerCase() === 'pcsc')

// eSIM 传输可选项：读卡器只有 PC/SC，模组有 AT/QMI/MBIM（不含 PC/SC）
const esimTransportOptions = computed(() => {
  if (isPCSCDevice.value) {
    return [{ label: 'PC/SC', value: 'pcsc' }]
  }
  return [
    { label: 'AT', value: 'at' },
    { label: 'QMI', value: 'qmi' },
    { label: 'MBIM', value: 'mbim' }
  ]
})

async function loadConfig() {
  const id = props.deviceId
  if (!id) {
    editConfig.value = null
    return
  }
  try {
    const result = await devicesStore.fetchConfig(id)
    if (!result.ok) return
    editConfig.value = JSON.parse(JSON.stringify(storeConfig.value || {})) as DeviceConfigDTO
    if (isQMIBackendOnly.value) {
      editConfig.value.device_backend = 'qmi'
    } else if (!editConfig.value.device_backend) {
      editConfig.value.device_backend = 'at'
    }
    editBaseline.value = JSON.stringify(editConfig.value)
    editDirty.value = false
  } catch {
    editConfig.value = null
  }
}

watch(
  () => props.deviceId,
  () => { void loadConfig() },
  { immediate: true }
)

watch(
  editConfig,
  (v) => {
    if (!v) { editDirty.value = false; return }
    const cur = JSON.stringify(v)
    if (!editBaseline.value) { editBaseline.value = cur; editDirty.value = false; return }
    editDirty.value = cur !== editBaseline.value
  },
  { deep: true }
)

async function handleSave() {
  if (!editConfig.value || !props.deviceId) return
  saving.value = true
  try {
    const result = await devicesService.updateConfig(props.deviceId, editConfig.value)
    if (!result.ok) throw new Error(result.error.message || '保存失败')
    if (result.data.warning) {
      ElMessage.warning(result.data.warning)
    } else if (result.data.requiresRestart) {
      ElMessage.warning('配置已保存，部分变更需重启服务后生效')
    } else {
      ElMessage.success('配置已保存')
    }
    editDirty.value = false
    editBaseline.value = JSON.stringify(editConfig.value)
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : '保存失败')
  } finally {
    saving.value = false
  }
}

async function handleDelete() {
  if (!props.deviceId) return
  const confirmed = await ElMessageBox.confirm(
    `确定删除设备 ${props.deviceId} 的配置？`,
    '确认删除',
    { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
  ).then(() => true).catch(() => false)
  if (!confirmed) return

  deleting.value = true
  try {
    const result = await devicesService.deleteManaged(props.deviceId)
    if (!result.ok) throw new Error(result.error.message || '删除失败')
    ElMessage.success('设备已删除')
    emit('device-deleted')
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : '删除失败')
  } finally {
    deleting.value = false
  }
}
</script>

<template>
  <div class="module-config-form">
    <!-- 头部 -->
    <div class="terminal-card-header">
      <div class="terminal-icon-box">
        <el-icon size="14"><Settings24Regular /></el-icon>
      </div>
      <div class="terminal-header-title">设备配置</div>
    </div>

    <!-- 内容区 -->
    <div v-if="editConfig" class="config-body">
      <!-- 只读信息卡片 -->
      <div class="info-card">
        <div v-for="item in readonlyInfo" :key="item.label" class="info-row">
          <span class="info-label">{{ item.label }}</span>
          <span class="info-value">{{ item.value || '--' }}</span>
        </div>
      </div>

      <!-- 可编辑表单 -->
      <div class="form-grid">
        <div class="field col-span-2">
          <label class="form-label">设备名称</label>
          <el-input v-model="editConfig.name" placeholder="显示名称" />
        </div>
        <div v-if="editConfig.esim_transport !== 'pcsc'" class="field">
          <label class="form-label">设备后端</label>
          <el-select v-model="editConfig.device_backend" class="!w-full" :disabled="isQMIBackendOnly || isMBIMBackendOnly">
            <el-option v-if="!isMBIMBackendOnly" label="AT (串口)" value="at" />
            <el-option v-if="!isMBIMBackendOnly" label="QMI" value="qmi" />
            <el-option v-if="isMBIMBackendOnly" label="MBIM" value="mbim" />
          </el-select>
        </div>
        <div class="field">
          <label class="form-label">eSIM 传输</label>
          <el-select v-model="editConfig.esim_transport" class="!w-full" :disabled="isPCSCDevice">
            <el-option v-for="opt in esimTransportOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
        </div>
          <div v-if="editConfig.esim_transport === 'pcsc'" class="field">
          <label class="form-label">PC/SC USB 路径</label>
          <el-input v-model="editConfig.pcsc_usb_path" class="!w-full" placeholder="USB 路径" disabled />
        </div>
      </div>
    </div>

    <!-- 底部操作栏 -->
    <div v-if="editConfig" class="config-footer">
      <button class="cfg-btn cfg-btn-save" :disabled="saving || !editDirty" @click="handleSave">
        {{ saving ? '保存中...' : '保存' }}
      </button>
      <button class="cfg-btn cfg-btn-delete" :disabled="deleting" @click="handleDelete">
        {{ deleting ? '删除中...' : '删除设备' }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.module-config-form {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--background);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}

.terminal-card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.terminal-icon-box {
  width: 28px;
  height: 28px;
  border-radius: 5px;
  background: var(--muted);
  border: 1px solid var(--border);
  color: var(--foreground);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.terminal-header-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
}

.config-footer {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 10px 14px;
  border-top: 1px solid var(--border);
  flex-shrink: 0;
}

.cfg-btn {
  padding: 5px 16px;
  font-size: 12px;
  font-weight: 600;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.15s;
  border: 1px solid var(--border);
  background: var(--background);
  color: var(--foreground);
  white-space: nowrap;
}

.cfg-btn:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.cfg-btn-save:not(:disabled):hover {
  background: var(--brand);
  border-color: var(--brand);
  color: #fff;
}

.cfg-btn-delete:not(:disabled):hover {
  background: var(--destructive);
  border-color: var(--destructive);
  color: #fff;
}

.config-body {
  padding: 16px;
  overflow-y: auto;
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* 只读信息卡片 */
.info-card {
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--muted);
  overflow: hidden;
}

.info-row {
  display: flex;
  align-items: center;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border);
}

.info-row:last-child {
  border-bottom: none;
}

.info-label {
  font-size: 11px;
  font-weight: 700;
  color: var(--muted-foreground);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  width: 100px;
  flex-shrink: 0;
}

.info-value {
  font-size: 13px;
  color: var(--foreground);
  font-family: var(--oomol-font-mono);
}

/* 可编辑表单 */
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.field.col-span-2 {
  grid-column: 1 / -1;
}

.form-label {
  font-size: 11px;
  font-weight: 700;
  color: var(--muted-foreground);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
</style>
