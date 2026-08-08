<script setup lang="ts">
import { computed, ref, watch, onMounted } from 'vue'
import type { DeviceOverviewItem } from '../types/api'
import { ArrowSync24Regular, Power24Regular, Mail24Regular } from '@vicons/fluent'
import CarrierIcon from './CarrierIcon.vue'
import CountryFlag from './CountryFlag.vue'
import { getPlmnInfo, loadPlmnInfo, type PlmnInfoEntry } from '../composables/plmn-info'

const props = defineProps<{
  device: DeviceOverviewItem
  rotating: boolean
  rebooting: boolean
  reconnectingVoWiFi: boolean
}>()

const emit = defineEmits<{
  'copy-text': [value: string]
  'rotate-ip': []
  'reboot-modem': []
  'reconnect-vowifi': []
  'open-sms': []
}>()

// PLMN 信息
onMounted(() => loadPlmnInfo())

const mcc = computed(() => props.device?.modem?.native_mcc || '')
const mnc = computed(() => props.device?.modem?.native_mnc || '')
const carrierName = computed(() => props.device?.modem?.operator || props.device?.modem?.native_spn || '')
const plmnKey = computed(() => mcc.value && mnc.value ? `${mcc.value}-${mnc.value}` : '')
const plmnInfo = ref<PlmnInfoEntry | null>(null)

watch(plmnKey, (key) => {
  plmnInfo.value = key ? getPlmnInfo(key) : null
}, { immediate: true })

const countryName = computed(() => plmnInfo.value?.country?.name || '')
const countryIso = computed(() => plmnInfo.value?.country?.iso || '')
const countryCode = computed(() => plmnInfo.value?.country?.code || '')
</script>

<template>
  <div class="device-header">
    <div class="device-header-content">
      <div class="flex items-center gap-3 min-w-0">
        <CarrierIcon
          :mcc="mcc"
          :mnc="mnc"
          :name="carrierName"
          :size="38"
          class="flex-shrink-0"
        />
        <div class="min-w-0">
          <div class="device-header-name">{{ carrierName || device.name || device.id }}</div>
          <div class="device-header-meta">
            <span v-if="plmnKey" class="device-header-plmn">{{ mcc }}:{{ mnc }}</span>
            <span v-if="countryCode" class="device-header-code">+{{ countryCode }}</span>
            <CountryFlag v-if="countryIso" :iso="countryIso" :size="16" class="device-header-flag" />
            <span v-if="countryName" class="device-header-country">{{ countryName }}</span>
            <span class="device-header-sep">·</span>
            <span class="font-mono cursor-pointer hover:underline" @click="emit('copy-text', device.id)">{{ device.id }}</span>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-2">
        <el-button v-if="device?.vowifi_enabled" :loading="reconnectingVoWiFi" @click="emit('reconnect-vowifi')" size="small">
          <el-icon size="16"><ArrowSync24Regular /></el-icon>
          重连 VoWiFi
        </el-button>
        <el-button v-else :loading="rotating" :disabled="!device?.network_connected" @click="emit('rotate-ip')" size="small">
          <el-icon size="16"><ArrowSync24Regular /></el-icon>
          切换 IP
        </el-button>
        <el-button :loading="rebooting" @click="emit('reboot-modem')" class="hover:!text-red-600" size="small">
          <el-icon size="16"><Power24Regular /></el-icon>
          重启模组
        </el-button>
        <el-button @click="emit('open-sms')" size="small">
          <el-icon size="16"><Mail24Regular /></el-icon>
          短信
        </el-button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.device-header {
  min-height: 60px;
  padding: 8px 24px;
  display: flex;
  align-items: center;
}

.device-header-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  gap: 8px 12px;
  flex-wrap: wrap;
}

.device-header-name {
  font-size: 15px;
  font-weight: 700;
  color: var(--foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-header-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 1px;
  font-size: 12px;
  color: var(--muted-foreground);
}

.device-header-plmn {
  font-family: var(--oomol-font-mono);
}

.device-header-code {
  font-family: var(--oomol-font-mono);
  color: var(--brand);
  opacity: 0.8;
}

.device-header-flag {
  opacity: 0.9;
}

.device-header-country {
  opacity: 0.7;
}

.device-header-sep {
  opacity: 0.4;
}
</style>
