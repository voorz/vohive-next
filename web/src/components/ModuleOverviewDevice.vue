<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import type { DeviceOverviewItem, CarrierWebsheetInfo } from '../types/api'
import { useSensitiveVisibility } from '../composables/useSensitiveVisibility'
import { activeEsimProfileDisplayName } from './deviceOverviewActiveEsim'
import { getPlmnInfo, loadPlmnInfo, type PlmnInfoEntry } from '../composables/plmn-info'
import { Eye24Regular, EyeOff24Regular, Sim24Regular } from '@vicons/fluent'
import { copyToClipboard } from '../utils/clipboard'
import { devicesService } from '../services/devices'
import { ElMessage } from 'element-plus'
import CarrierWebsheetDialog from './CarrierWebsheetDialog.vue'

const props = defineProps<{
  device: DeviceOverviewItem | null
}>()

const showSensitive = useSensitiveVisibility()

const activeEsimProfile = computed(() => activeEsimProfileDisplayName(props.device))

const backendModeDisplay = computed(() => {
const m = props.device?.backend_mode
if (m === 'qmi') return 'QMI'
if (m === 'mbim') return 'MBIM'
if (m === 'at') return 'AT'
if (m === 'pcsc') return 'PC/SC'
return m || '--'
})

// 原运营商（SIM 卡原始运营商）
const plmnInfo = ref<PlmnInfoEntry | null>(null)

onMounted(() => loadPlmnInfo())

watch(
  () => [props.device?.modem?.native_mcc, props.device?.modem?.native_mnc],
  ([mcc, mnc]) => {
    const key = mcc && mnc ? `${mcc}-${mnc}` : ''
    plmnInfo.value = key ? getPlmnInfo(key) : null
  },
  { immediate: true }
)

const simOperatorDisplay = computed(() => {
  const mcc = props.device?.modem?.native_mcc || ''
  const mnc = props.device?.modem?.native_mnc || ''
  const plmn = mcc && mnc ? `${mcc}${mnc}` : ''
  const spn = props.device?.modem?.native_spn
  const name = spn || plmnInfo.value?.operators?.[0]?.brand || plmnInfo.value?.operators?.[0]?.operator || '--'
  return plmn ? `${name} | ${plmn}` : name
})

// E911
const e911Starting = ref(false)
const e911WebsheetOpen = ref(false)
const e911Websheet = ref<CarrierWebsheetInfo | null>(null)

async function openE911Websheet() {
  const id = props.device?.id
  if (!id || e911Starting.value) return
  e911Starting.value = true
  try {
    const result = await devicesService.startE911Websheet(id)
    if (!result.ok) throw new Error(result.error.message || 'E911地址设置页面打开失败')
    e911Websheet.value = result.data
    e911WebsheetOpen.value = true
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : 'E911地址设置页面打开失败')
  } finally {
    e911Starting.value = false
  }
}

function copyVal(val: string | undefined) {
  if (!val || val === '--') return
  void copyToClipboard(val)
}
</script>

<template>
  <div class="ov-card">
    <div class="ov-card-head">
      <div class="ov-icon-box">
        <el-icon size="14"><Sim24Regular /></el-icon>
      </div>
      <span class="ov-card-title">设备信息</span>
      <div class="ov-card-head-actions">
        <button class="ov-icon-btn" :title="showSensitive ? '隐藏' : '显示'" @click="showSensitive = !showSensitive">
          <el-icon size="16"><Eye24Regular v-if="showSensitive" /><EyeOff24Regular v-else /></el-icon>
        </button>
      </div>
    </div>
    <div class="ov-card-body">
      <div class="device-grid">
        <div class="device-field">
          <span class="device-field-label">IMEI</span>
          <span class="device-field-value copyable" :class="{ masked: !showSensitive }" @click="copyVal(device?.modem?.imei)">{{ device?.modem?.imei || '--' }}</span>
        </div>
        <div class="device-field">
          <span class="device-field-label">ICCID</span>
          <span class="device-field-value copyable" :class="{ masked: !showSensitive }" @click="copyVal(device?.modem?.iccid)">{{ device?.modem?.iccid || '--' }}</span>
        </div>
        <div class="device-field">
          <span class="device-field-label">IMSI</span>
          <span class="device-field-value copyable" :class="{ masked: !showSensitive }" @click="copyVal(device?.modem?.imsi)">{{ device?.modem?.imsi || '--' }}</span>
        </div>
        <div class="device-field">
          <span class="device-field-label">本机号码</span>
          <span class="device-field-value copyable" :class="{ masked: !showSensitive }" @click="copyVal(device?.local_phone)">{{ device?.local_phone || '--' }}</span>
        </div>
        <div class="device-field">
          <span class="device-field-label">原运营商</span>
          <span class="device-field-value copyable" @click="copyVal(simOperatorDisplay)">{{ simOperatorDisplay }}</span>
        </div>
        <div class="device-field">
          <span class="device-field-label">厂商</span>
          <span class="device-field-value copyable" @click="copyVal(device?.modem?.manufacturer || device?.manufacturer)">{{ device?.modem?.manufacturer || device?.manufacturer || '--' }}</span>
        </div>
        <div class="device-field">
          <span class="device-field-label">型号</span>
          <span class="device-field-value copyable" @click="copyVal(device?.modem?.model)">{{ device?.modem?.model || '--' }}</span>
        </div>
        <div class="device-field">
          <span class="device-field-label">固件版本</span>
          <span class="device-field-value copyable" @click="copyVal(device?.modem?.firmware)">{{ device?.modem?.firmware || '--' }}</span>
        </div>
        <div class="device-field">
          <span class="device-field-label">硬件版本</span>
          <span class="device-field-value copyable" @click="copyVal(device?.modem?.hardware_revision)">{{ device?.modem?.hardware_revision || '--' }}</span>
        </div>
        <div class="device-field">
          <span class="device-field-label">运行模式</span>
          <span class="device-field-value">{{ backendModeDisplay }}</span>
        </div>
        <div v-if="activeEsimProfile" class="device-field">
          <span class="device-field-label">当前 eSIM</span>
          <span class="device-field-value copyable" :class="{ masked: !showSensitive }" @click="copyVal(activeEsimProfile)">{{ activeEsimProfile }}</span>
        </div>
        <div v-if="device?.e911_setup_available" class="device-field full">
          <span class="device-field-label">E911 地址</span>
          <el-button size="small" type="primary" plain :loading="e911Starting" class="!border-0" @click="openE911Websheet">设置</el-button>
        </div>
      </div>
    </div>
  </div>

  <CarrierWebsheetDialog
    v-model="e911WebsheetOpen"
    :websheet="e911Websheet"
  />
</template>

<style scoped>
.ov-card {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--background);
  overflow: hidden;
}
.ov-card-head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
}
.ov-icon-box {
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
.ov-card-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
  flex: 1;
}
.ov-card-head-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}
.ov-icon-btn {
  width: 24px;
  height: 24px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--muted-foreground);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.12s;
}
.ov-icon-btn:hover { background: var(--accent); color: var(--foreground); }
.ov-card-body { padding: 14px; }

.device-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px 16px;
  position: relative;
}
.device-grid::before {
  content: '';
  position: absolute;
  left: 50%;
  top: 0;
  bottom: 0;
  width: 1px;
  background: var(--border);
}
.device-field {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
}
.device-field-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-foreground);
  flex-shrink: 0;
}
.device-field-value {
  font-size: 12px;
  color: var(--foreground);
  font-family: var(--oomol-font-mono);
  text-align: right;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: filter 0.15s;
}
.device-field-value.masked {
  filter: blur(4px);
  user-select: none;
}
.device-field-value.copyable {
  cursor: pointer;
}
.device-field-value.copyable:hover {
  color: var(--brand);
}
.device-field.full {
  grid-column: 1 / -1;
}
</style>
