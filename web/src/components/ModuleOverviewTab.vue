<script setup lang="ts">
import type { DeviceOverviewItem, CardPolicy } from '../types/api'
import ModuleOverviewStatus from './ModuleOverviewStatus.vue'
import ModuleOverviewDevice from './ModuleOverviewDevice.vue'
import ModuleOverviewNetwork from './ModuleOverviewNetwork.vue'

defineProps<{
  device: DeviceOverviewItem | null
  policy: CardPolicy | null
  deviceOnline: boolean
  trafficSpeedRx?: string
  trafficSpeedTx?: string
  trafficMinuteRx?: string
  trafficMinuteTx?: string
  isPCSC?: boolean
  reconnectingVoWiFi?: boolean
  rotating?: boolean
}>()

defineEmits<{
  'reconnect-vowifi': []
  'rotate-ip': []
  changed: []
}>()
</script>

<template>
  <div class="ov-overview">
    <ModuleOverviewStatus
      :device="device"
      :reconnecting-vo-wi-fi="reconnectingVoWiFi"
      :rotating="rotating"
      @reconnect-vowifi="$emit('reconnect-vowifi')"
      @rotate-ip="$emit('rotate-ip')"
    />
    <ModuleOverviewDevice :device="device" :is-p-c-s-c="isPCSC" />
    <ModuleOverviewNetwork
      v-if="!isPCSC && !device?.vowifi_enabled && device?.data_connected"
      :device="device"
      :traffic-speed-rx="trafficSpeedRx"
      :traffic-speed-tx="trafficSpeedTx"
      :traffic-minute-rx="trafficMinuteRx"
      :traffic-minute-tx="trafficMinuteTx"
    />
  </div>
</template>

<style scoped>
.ov-overview {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
</style>
