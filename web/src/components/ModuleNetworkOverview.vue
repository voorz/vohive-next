<script setup lang="ts">
import type { DeviceOverviewItem } from '../types/api'

defineProps<{
  device: DeviceOverviewItem | null
  trafficSpeedRx?: string
  trafficSpeedTx?: string
  trafficMinuteRx?: string
  trafficMinuteTx?: string
}>()
</script>

<template>
  <div class="network-overview-card">
    <!-- TODO: 网络数据实时上下行展现 -->
    <div class="placeholder-text">网络概览（占位）</div>
  </div>
</template>

<style scoped>
.network-overview-card {
  flex: 1;
  min-width: 0;
  border: 1px solid var(--border);
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100%;
  background: var(--background);
}
.placeholder-text {
  font-size: 12px;
  color: var(--muted-foreground);
}
</style>
