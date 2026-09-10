<script setup lang="ts">
/**
 * DashboardChartsCard — Surge 风格实时流量监控卡片
 *
 * 4 卡片：上传 / 下载 / CPU / 内存
 * 统一风格：标签 + 大数字 + SparklineChart + 底部信息
 */
import { computed, ref, watch } from 'vue'
import SparklineChart, { type ChartPoint } from './SparklineChart.vue'
import { useDashboardTrafficStream } from '../composables/useDashboardTrafficStream'

const {
  dlSpeedParts,
  ulSpeedParts,
  totalRxStr,
  totalTxStr,
  downloadSpeedHistory,
  uploadSpeedHistory,
  hostPerf,
} = useDashboardTrafficStream()

// ---- tooltip/label formatters ----

function speedLabelFormatter(value: number): string {
  if (value <= 0) return ''
  const units = ['B/s', 'KB/s', 'MB/s', 'GB/s']
  let v = value
  let i = 0
  while (v >= 1000 && i < units.length - 1) { v /= 1000; i++ }
  return `${v.toFixed(i === 0 ? 0 : 0)} ${units[i]}`
}

type TooltipParam = {
  axisValue?: string | number
  value?: number | [number, number]
  seriesName?: string
  color?: string
}

function speedTooltipFormatter(params: unknown[]): string {
  return (params as TooltipParam[])
    .map((item) => {
      const v = Array.isArray(item.value) ? item.value[1] : (item.value ?? 0)
      return `${item.seriesName || ''}: ${speedLabelFormatter(Number(v) || 0)}`
    })
    .join('\n')
}

function percentLabelFormatter(value: number): string {
  return `${(value || 0).toFixed(1)}%`
}

function percentTooltipFormatter(params: unknown[]): string {
  return (params as TooltipParam[])
    .map((item) => {
      const v = Array.isArray(item.value) ? item.value[1] : (item.value ?? 0)
      return `${item.seriesName || ''}: ${(Number(v) || 0).toFixed(1)}%`
    })
    .join('\n')
}

// ---- 宿主机数据 ----

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let val = Math.abs(bytes)
  let i = 0
  while (val >= 1024 && i < units.length - 1) { val /= 1024; i++ }
  return `${val.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

const cpuPercent = computed(() => hostPerf.value?.cpu_percent ?? 0)
const memPercent = computed(() => hostPerf.value?.memory_percent ?? 0)
const cpuFooter = computed(() => `使用率 ${cpuPercent.value.toFixed(1)}%`)
const memFooter = computed(() => {
  const p = hostPerf.value
  if (!p || p.memory_total_bytes <= 0) return '--'
  return `${formatBytes(p.memory_used_bytes)} / ${formatBytes(p.memory_total_bytes)}`
})

// CPU/内存历史滚动窗口
const WINDOW = 62
function makeInit(): ChartPoint[] {
  const now = Date.now()
  return new Array(WINDOW).fill(0).map((_, i) => ({
    name: now - (WINDOW - 1 - i) * 10000,
    value: [now - (WINDOW - 1 - i) * 10000, 0] as [number, number]
  }))
}

const cpuHistory = ref<ChartPoint[]>(makeInit())
const memHistory = ref<ChartPoint[]>(makeInit())

watch(hostPerf, (p) => {
  if (!p) return
  const ts = Date.now()
  cpuHistory.value = [...cpuHistory.value.slice(-WINDOW + 1), { name: ts, value: [ts, p.cpu_percent] }]
  memHistory.value = [...memHistory.value.slice(-WINDOW + 1), { name: ts, value: [ts, p.memory_percent] }]
})
</script>

<template>
  <div class="charts-card">
    <div class="charts-card-grid">
      <!-- 上传速度 -->
      <div class="chart-cell">
        <div class="chart-label">上传</div>
        <div class="chart-value-row">
          <span class="chart-value-num">{{ ulSpeedParts.value }}</span>
          <span class="chart-value-unit">{{ ulSpeedParts.unit }}/s</span>
        </div>
        <div class="chart-sparkline">
          <SparklineChart
            :data="uploadSpeedHistory as ChartPoint[]"
            :y-axis-floor="60000"
            :window-seconds="60"
            color="info"
            name="上传"
            :label-formatter="speedLabelFormatter"
            :tooltip-formatter="speedTooltipFormatter"
          />
        </div>
        <div class="chart-footer">总量 {{ totalTxStr }}</div>
      </div>

      <!-- 下载速度 -->
      <div class="chart-cell">
        <div class="chart-label">下载</div>
        <div class="chart-value-row">
          <span class="chart-value-num">{{ dlSpeedParts.value }}</span>
          <span class="chart-value-unit">{{ dlSpeedParts.unit }}/s</span>
        </div>
        <div class="chart-sparkline">
          <SparklineChart
            :data="downloadSpeedHistory as ChartPoint[]"
            :y-axis-floor="60000"
            :window-seconds="60"
            name="下载"
            :label-formatter="speedLabelFormatter"
            :tooltip-formatter="speedTooltipFormatter"
          />
        </div>
        <div class="chart-footer">总量 {{ totalRxStr }}</div>
      </div>

      <!-- CPU 使用率 -->
      <div class="chart-cell">
        <div class="chart-label">CPU</div>
        <div class="chart-value-row">
          <span class="chart-value-num">{{ cpuPercent.toFixed(1) }}</span>
          <span class="chart-value-unit">%</span>
        </div>
        <div class="chart-sparkline">
          <SparklineChart
            :data="cpuHistory as ChartPoint[]"
            :y-axis-floor="10"
            :window-seconds="60"
            color="primary"
            name="CPU"
            :label-formatter="percentLabelFormatter"
            :tooltip-formatter="percentTooltipFormatter"
          />
        </div>
        <div class="chart-footer">{{ cpuFooter }}</div>
      </div>

      <!-- 内存使用率 -->
      <div class="chart-cell">
        <div class="chart-label">内存</div>
        <div class="chart-value-row">
          <span class="chart-value-num">{{ memPercent.toFixed(1) }}</span>
          <span class="chart-value-unit">%</span>
        </div>
        <div class="chart-sparkline">
          <SparklineChart
            :data="memHistory as ChartPoint[]"
            :y-axis-floor="10"
            :window-seconds="60"
            color="info"
            name="内存"
            :label-formatter="percentLabelFormatter"
            :tooltip-formatter="percentTooltipFormatter"
          />
        </div>
        <div class="chart-footer">{{ memFooter }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.charts-card {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--card);
  padding: 12px;
  container-type: inline-size;
}

.charts-card-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.chart-cell {
  display: flex;
  flex-direction: column;
  gap: 6px;
  border-radius: 12px;
  padding: 12px;
  background: var(--muted);
  min-width: 0;
  overflow: hidden;
}

.chart-label {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--muted-foreground);
  display: flex;
  align-items: center;
  gap: 6px;
}

.chart-value-row {
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.chart-value-num {
  font-size: 28px;
  font-weight: 200;
  font-variant-numeric: tabular-nums;
  color: var(--foreground);
  line-height: 1.1;
}

.chart-value-unit {
  font-size: 13px;
  color: var(--muted-foreground);
}

.chart-sparkline {
  margin-top: 4px;
  height: 56px;
  min-width: 0;
  overflow: hidden;
}

.chart-footer {
  font-size: 11px;
  color: var(--muted-foreground);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

/* 宽屏 4 列 */
@container (min-width: 768px) {
  .charts-card-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}

/* 窄屏 1 列 */
@container (max-width: 480px) {
  .charts-card-grid {
    grid-template-columns: 1fr;
  }

  .chart-cell {
    padding: 12px;
  }

  .chart-value-num {
    font-size: 24px;
  }
}

/* 极窄屏紧凑 */
@container (max-width: 360px) {
  .charts-card {
    padding: 10px;
  }

  .charts-card-grid {
    gap: 8px;
  }

  .chart-cell {
    padding: 10px;
  }

  .chart-value-num {
    font-size: 22px;
  }

  .chart-sparkline {
    height: 48px;
  }
}
</style>
