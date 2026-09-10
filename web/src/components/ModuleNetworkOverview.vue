<script setup lang="ts">
/**
 * ModuleNetworkOverview — 网络概览卡片（实时上下行速率双线图）
 *
 * 照搬 zashboard SpeedCharts 设计，展示当前设备的实时上下行速率。
 */
import { computed } from 'vue'
import TimeSeriesChart, { type ChartSeries, type ChartTooltipParam } from './TimeSeriesChart.vue'
import type { ChartPoint } from './SparklineChart.vue'
import type { DeviceOverviewItem } from '../types/api'

const props = defineProps<{
  device: DeviceOverviewItem | null
  downloadSpeedHistory?: ChartPoint[]
  uploadSpeedHistory?: ChartPoint[]
  trafficSpeedRx?: string
  trafficSpeedTx?: string
}>()

const chartsData = computed<ChartSeries[]>(() => [
  {
    name: '上传',
    data: props.uploadSpeedHistory || [],
  },
  {
    name: '下载',
    data: props.downloadSpeedHistory || [],
  },
])

function labelFormatter(value: number): string {
  if (value <= 0) return ''
  const units = ['B/s', 'KB/s', 'MB/s', 'GB/s']
  let v = value
  let i = 0
  while (v >= 1000 && i < units.length - 1) { v /= 1000; i++ }
  return `${v.toFixed(i === 0 ? 0 : 0)} ${units[i]}`
}

function getChartPointValue(point: ChartPoint): [number, number] {
  return Array.isArray(point.value) ? point.value : [point.name, 0]
}

function tooltipFormatter(params: ChartTooltipParam[]): string {
  return params
    .map((item) => {
      const [, value] = getChartPointValue(item.data)
      const units = ['B/s', 'KB/s', 'MB/s', 'GB/s']
      let v = value
      let i = 0
      while (v >= 1000 && i < units.length - 1) { v /= 1000; i++ }
      const formatted = `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
      return `<div style="display:flex;align-items:center;gap:4px;margin:2px 0;">
        <div style="width:8px;height:8px;border-radius:50%;background:${item.color};"></div>
        ${item.seriesName}: ${formatted}
      </div>`
    })
    .join('')
}
</script>

<template>
  <div class="network-overview-card">
    <div class="net-ov-chart">
      <TimeSeriesChart
        :data="chartsData"
        :label-formatter="labelFormatter"
        :tooltip-formatter="tooltipFormatter"
        :y-axis-floor="60000"
        :window-seconds="60"
        :show-pause-button="true"
      />
    </div>
  </div>
</template>

<style scoped>
.network-overview-card {
  flex: 1;
  min-width: 0;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--background);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.net-ov-chart {
  flex: 1;
  min-height: 80px;
  padding: 4px 8px 4px 0;
}
</style>
