<script setup lang="ts">
import type { DeviceOverviewItem, CardPolicy } from '../types/api'
import ModuleOverviewStatus from './ModuleOverviewStatus.vue'
import ModuleOverviewDevice from './ModuleOverviewDevice.vue'
import ModuleOverviewOutboundProxy from './ModuleOverviewOutboundProxy.vue'
import type { ChartPoint } from './SparklineChart.vue'

defineProps<{
  device: DeviceOverviewItem | null
  policy: CardPolicy | null
  deviceOnline: boolean
  trafficSpeedRx?: string
  trafficSpeedTx?: string
  trafficMinuteRx?: string
  trafficMinuteTx?: string
  trafficMinuteRxBytes?: number
  trafficMinuteTxBytes?: number
  downloadSpeedHistory?: ChartPoint[]
  uploadSpeedHistory?: ChartPoint[]
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
      :traffic-speed-rx="trafficSpeedRx"
      :traffic-speed-tx="trafficSpeedTx"
      :traffic-minute-rx="trafficMinuteRx"
      :traffic-minute-tx="trafficMinuteTx"
      :traffic-minute-rx-bytes="trafficMinuteRxBytes"
      :traffic-minute-tx-bytes="trafficMinuteTxBytes"
      :download-speed-history="downloadSpeedHistory"
      :upload-speed-history="uploadSpeedHistory"
      @reconnect-vowifi="$emit('reconnect-vowifi')"
      @rotate-ip="$emit('rotate-ip')"
    />
    <ModuleOverviewOutboundProxy
      v-if="device?.data_connected"
      :device="device"
      @changed="$emit('changed')"
    />
    <ModuleOverviewDevice :device="device" :is-p-c-s-c="isPCSC" />
  </div>
</template>

<style scoped>
.ov-overview {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
</style>
