<script setup lang="ts">
import { computed } from 'vue'
import type { DashboardDevice } from '../types/api'
import StatusLight from './StatusLight.vue'
import {
  Cellular3G24Regular,
  Cellular4G24Regular,
  Cellular5G24Regular,
  CellularData124Regular,
  Wifi124Regular,
  Globe24Regular,
  Sim24Regular
} from '@vicons/fluent'

const props = defineProps<{ device: DashboardDevice }>()
const emit = defineEmits<{
  'open-device': [id: string]
}>()

const displayNetworkMode = computed(() => {
  const mode = String(props.device?.network_mode || '').trim()
  const duplex = String(props.device?.network_duplex || '').trim()
  if (!mode) return ''
  return duplex ? `${duplex} ${mode}` : mode
})

const networkIcon = computed(() => {
  if (props.device?.vowifi_active) return Wifi124Regular
  const mode = displayNetworkMode.value
  if (!mode) return CellularData124Regular
  const m = String(mode).toUpperCase()
  if (m.includes('5G') || m.includes('NR')) return Cellular5G24Regular
  if (m.includes('4G') || m.includes('LTE')) return Cellular4G24Regular
  if (m.includes('3G') || m.includes('WCDMA') || m.includes('HSPA') || m.includes('UMTS')) return Cellular3G24Regular
  return CellularData124Regular
})

const networkColor = computed(() => {
  if (props.device?.vowifi_active) return 'var(--success)'
  const mode = displayNetworkMode.value
  if (!mode) return 'var(--muted-foreground)'
  const m = String(mode).toUpperCase()
  if (m.includes('5G') || m.includes('NR')) return 'var(--brand)'
  if (m.includes('4G') || m.includes('LTE')) return 'var(--info)'
  if (m.includes('3G')) return 'var(--warning)'
  return 'var(--muted-foreground)'
})

const networkModeText = computed(() => {
  const mode = displayNetworkMode.value
  if (!mode) return ''
  const parts = String(mode).trim().split(/\s+/).filter(Boolean)
  if (parts.length <= 1) return parts[0] || ''
  return parts[1] || ''
})

const hideNetworkModeOnNarrow = computed(() => {
  return networkModeText.value.toUpperCase() === 'LTE'
})

function hasValidSignalDbm(dbm: number | null | undefined): dbm is number {
  return typeof dbm === 'number' && Number.isFinite(dbm) && dbm !== 0 && dbm !== -999
}

function getSignalColor(dbm: number | null | undefined) {
  if (!hasValidSignalDbm(dbm)) return 'var(--muted-foreground)'
  if (dbm > -70) return 'var(--success)'
  if (dbm > -90) return 'var(--warning)'
  return 'var(--destructive)'
}

function getSignalBars(dbm: number | null | undefined) {
  if (!hasValidSignalDbm(dbm)) return 0
  if (dbm > -70) return 4
  if (dbm > -85) return 3
  if (dbm > -100) return 2
  return 1
}
</script>

<template>
  <button
    type="button"
    class="device-card ui-card ui-card-hover"
    @click="emit('open-device', device.id)"
  >
    <div class="device-card-inner">
      <div class="flex justify-between items-start mb-3">
        <div class="flex items-center gap-3">
          <div class="device-card-icon">
            <el-icon size="20"><Sim24Regular /></el-icon>
          </div>
          <div>
            <h3 class="device-card-title">{{ device.name || device.id }}</h3>
            <div class="flex items-center gap-1.5 mt-0.5">
              <StatusLight :tone="device.healthy ? 'success' : 'danger'" size="md" :animated="device.healthy" />
              <span
                class="text-xs font-medium"
                :style="{ color: device.healthy ? 'var(--success)' : 'var(--destructive)' }"
              >
                {{ device.healthy ? '在线' : '离线' }}
              </span>
            </div>
          </div>
        </div>
      </div>

      <div class="space-y-3">
        <div class="device-card-row">
          <div class="flex items-center gap-2 min-w-0">
            <div class="flex items-center gap-1.5">
              <el-icon size="18" :style="{ color: networkColor }">
                <component :is="networkIcon" />
              </el-icon>
              <span
                v-if="!device.vowifi_active && device.network_mode && networkModeText"
                class="text-[11px] font-bold tracking-tighter leading-none"
                :class="hideNetworkModeOnNarrow ? 'hidden xl:inline' : ''"
              >
                {{ networkModeText }}
              </span>
            </div>
            <span class="flex-1 min-w-0 text-sm font-medium whitespace-nowrap truncate" style="color: var(--foreground);">
              {{ device.vowifi_active ? 'Wi-Fi Calling' : (device.operator || '检测中...') }}
            </span>
          </div>
          <div v-if="!device.vowifi_active" class="flex items-center gap-1" title="信号强度">
            <div class="flex items-end gap-[2px] h-3">
              <div
                v-for="i in 4"
                :key="i"
                class="w-1 rounded-sm transition-all duration-500"
                :style="{
                  height: `${i * 25}%`,
                  background: getSignalBars(device.signal_dbm) >= i ? getSignalColor(device.signal_dbm) : 'var(--border)'
                }"
              />
            </div>
            <span class="text-xs font-mono ml-1 hidden xl:inline" style="color: var(--muted-foreground);">{{ device.signal_dbm }}dBm</span>
          </div>
        </div>

        <div class="flex justify-between items-center text-sm">
          <span class="flex items-center gap-1.5" style="color: var(--muted-foreground);">
            <el-icon><Globe24Regular /></el-icon> 公网 IP
          </span>
          <span class="font-mono font-bold" style="color: var(--brand);">{{ device.public_ip || '---' }}</span>
        </div>
      </div>
    </div>
  </button>
</template>

<style scoped>
.device-card {
  display: block;
  width: 100%;
  overflow: hidden;
  text-align: left;
  cursor: pointer;
  transition: transform 200ms ease, box-shadow 200ms ease;
}

.device-card:focus-visible {
  outline: 2px solid var(--ring);
  outline-offset: 2px;
}

.device-card-inner {
  padding: 12px;
}

.device-card-icon {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--muted);
  color: var(--foreground);
}

.device-card-title {
  font-weight: 660;
  font-size: 15px;
  line-height: 1.3;
  color: var(--foreground);
}

.device-card-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--muted);
}
</style>
