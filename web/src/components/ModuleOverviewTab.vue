<script setup lang="ts">
import type { DeviceOverviewItem } from '../types/api'
import ModuleOverviewStatus from './ModuleOverviewStatus.vue'
import ModuleOverviewDevice from './ModuleOverviewDevice.vue'
import ModuleOverviewNetwork from './ModuleOverviewNetwork.vue'

defineProps<{
  device: DeviceOverviewItem | null
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
    <ModuleOverviewDevice :device="device" />
    <ModuleOverviewNetwork
      v-if="!isPCSC"
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
