<script setup lang="ts">
/**
 * DashboardChartsCard — Surge 风格实时流量监控卡片
 *
 * 照搬 zashboard ChartsCard 设计：3 卡片（上传/下载/连接数）
 * 每卡含 SparklineChart + 当前值 + 累计/概览。
 */
import SparklineChart, { type ChartPoint } from './SparklineChart.vue'
import { useDashboardTrafficStream } from '../composables/useDashboardTrafficStream'

const {
  dlSpeedParts,
  ulSpeedParts,
  totalRxStr,
  totalTxStr,
  downloadSpeedHistory,
  uploadSpeedHistory,
  connectionsHistory,
  connectionCount,
  onlineCount,
  deviceCount,
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

function connLabelFormatter(value: number): string {
  return `${Math.round(value)}`
}

function connTooltipFormatter(params: unknown[]): string {
  return (params as TooltipParam[])
    .map((item) => {
      const v = Array.isArray(item.value) ? item.value[1] : (item.value ?? 0)
      return `${item.seriesName || ''}: ${Math.round(Number(v) || 0)}`
    })
    .join('\n')
}
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

      <!-- 活跃连接数 -->
      <div class="chart-cell chart-cell-conn">
        <div class="chart-label">
          连接
          <span class="chart-dot" />
        </div>
        <div class="chart-value-row">
          <span class="chart-value-num">{{ connectionCount }}</span>
        </div>
        <div class="chart-sparkline">
          <SparklineChart
            :data="connectionsHistory as ChartPoint[]"
            :y-axis-floor="10"
            :window-seconds="60"
            name="连接"
            :label-formatter="connLabelFormatter"
            :tooltip-formatter="connTooltipFormatter"
          />
        </div>
        <div class="chart-footer">
          <span>在线 {{ onlineCount }}</span>
          <span>设备 {{ deviceCount }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.charts-card {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--card);
  padding: 16px;
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
  padding: 16px;
  background: var(--muted);
  min-width: 0;
  overflow: hidden;
}

.chart-cell-conn {
  grid-column: span 2;
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

.chart-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--brand);
  display: inline-block;
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

/* 宽屏 3 列 */
@container (min-width: 768px) {
  .charts-card-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .chart-cell-conn {
    grid-column: span 1;
  }
}

/* 窄屏 1 列 */
@container (max-width: 480px) {
  .charts-card-grid {
    grid-template-columns: 1fr;
  }

  .chart-cell-conn {
    grid-column: span 1;
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
