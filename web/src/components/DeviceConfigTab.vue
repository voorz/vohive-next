<script setup lang="ts">
import { computed, watch } from 'vue'
import {
  Router24Regular,
  Delete24Regular,
  Save24Regular
} from '@vicons/fluent'
import type { DeviceConfigDTO, DeviceOverviewItem } from '../types/api'
import { isWwanQmiControlPath } from '../utils/deviceBackend'

const props = defineProps<{
  editConfig: DeviceConfigDTO | null
  deviceStatus?: DeviceOverviewItem | null
  saving: boolean
  deleting: boolean
}>()

const emit = defineEmits<{
  save: []
  delete: []
}>()

const activeControlDevice = computed(() => props.deviceStatus?.control_device || props.editConfig?.control_device)
const activeInterface = computed(() => props.deviceStatus?.interface || props.editConfig?.interface)
const activeATPort = computed(() => props.deviceStatus?.at_port || props.editConfig?.at_port)
const activeUsbPath = computed(() => props.deviceStatus?.usb_path || props.editConfig?.usb_path)

const isQMIBackendOnly = computed(() => isWwanQmiControlPath(activeControlDevice.value))
const isMBIMBackendOnly = computed(
  () => String(props.editConfig?.device_backend || '').toLowerCase() === 'mbim'
)

watch(
  isQMIBackendOnly,
  (locked) => {
    if (locked && props.editConfig) {
      props.editConfig.device_backend = 'qmi'
    }
  },
  { immediate: true }
)
</script>

<template>
  <div>
    <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-4">
      <div class="flex items-center gap-3">
        <div class="tab-icon-box">
          <el-icon size="20"><Router24Regular /></el-icon>
        </div>
        <div>
          <div class="tab-title">设备配置</div>
          <div class="tab-desc">配置存储在数据库中，部分字段可能需要重启生效</div>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <el-button type="danger" :loading="deleting" @click="emit('delete')" class="!border-0">
          <el-icon><Delete24Regular /></el-icon>
          删除设备
        </el-button>
        <el-button type="primary" :loading="saving" @click="emit('save')" class="!border-0">
          <el-icon><Save24Regular /></el-icon>
          保存配置
        </el-button>
      </div>
    </div>

    <div v-if="editConfig" class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <div class="space-y-1">
        <label class="text-xs font-bold text-gray-500 uppercase tracking-wider">ID</label>
        <el-input v-model="editConfig.id" disabled />
      </div>
      <div class="space-y-1">
        <label class="text-xs font-bold text-gray-500 uppercase tracking-wider">名称</label>
        <el-input v-model="editConfig.name" placeholder="显示名称" />
      </div>
      <div class="space-y-1">
        <label class="text-xs font-bold text-gray-500 uppercase tracking-wider">IMEI 绑定</label>
        <el-input v-model="editConfig.modem_imei" disabled placeholder="自动识别（添加时绑定）" />
      </div>
      <div class="space-y-1">
        <label class="text-xs font-bold text-gray-500 uppercase tracking-wider">设备路径</label>
        <el-input :model-value="activeUsbPath || ''" disabled placeholder="由系统自动探测" />
      </div>
      <div class="space-y-1">
        <label class="text-xs font-bold text-gray-500 uppercase tracking-wider">网卡接口</label>
        <el-input :model-value="activeInterface || ''" disabled placeholder="由系统自动探测" />
      </div>
      <div class="space-y-1">
        <label class="text-xs font-bold text-gray-500 uppercase tracking-wider">AT 端口</label>
        <el-input :model-value="activeATPort || ''" disabled placeholder="由系统自动探测" />
      </div>
      <div class="space-y-1">
        <label class="text-xs font-bold text-gray-500 uppercase tracking-wider">控制设备</label>
        <el-input :model-value="activeControlDevice || ''" disabled placeholder="由系统自动探测" />
      </div>
      <div class="ui-panel-muted p-3 space-y-2">
        <div class="flex items-center justify-between">
          <div>
            <div class="text-sm font-bold text-gray-800 dark:text-gray-100">设备运行模式</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">
              {{ isQMIBackendOnly ? '此类设备固定 QMI，AT 口仅用于终端'
                 : (isMBIMBackendOnly ? '此类设备固定 MBIM，AT 口仅用于终端'
                 : 'AT=传统串口 / QMI=纯 QMI') }}
            </div>
          </div>
          <el-select
            v-model="editConfig.device_backend"
            style="width: 120px"
            placeholder="AT"
            :disabled="isQMIBackendOnly || isMBIMBackendOnly"
          >
            <el-option v-if="!isMBIMBackendOnly" label="AT" value="at" :disabled="isQMIBackendOnly" />
            <el-option v-if="!isMBIMBackendOnly" label="QMI" value="qmi" :disabled="!activeControlDevice && editConfig.device_backend !== 'qmi'" />
            <el-option v-if="isMBIMBackendOnly" label="MBIM" value="mbim" />
          </el-select>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tab-icon-box {
  width: 38px;
  height: 38px;
  border-radius: 6px;
  background: var(--muted);
  border: 1px solid var(--border);
  color: var(--foreground);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.tab-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
}

.tab-desc {
  font-size: 12px;
  color: var(--muted-foreground);
  margin-top: 2px;
}
</style>
