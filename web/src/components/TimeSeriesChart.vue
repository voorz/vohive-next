<script setup lang="ts">
/**
 * TimeSeriesChart — 双线时间序列面积图
 *
 * 照搬 zashboard TimeSeriesChart 设计，适配 vohive ECharts 懒加载模式。
 * 支持多条数据线，面积渐变，暂停/播放按钮。
 */
import { computed, onMounted, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import type { ChartPoint } from './SparklineChart.vue'

export type ChartSeries = {
  name: string
  data: ChartPoint[]
}

export type ChartTooltipParam = {
  data: ChartPoint
  seriesName: string
  color: string
}

const props = withDefaults(defineProps<{
  data: ChartSeries[]
  labelFormatter: (value: number) => string
  tooltipFormatter: (value: ChartTooltipParam[]) => string
  yAxisFloor?: number
  windowSeconds?: number
  showPauseButton?: boolean
}>(), {
  windowSeconds: 60,
  showPauseButton: true,
})

const chartRef = ref<HTMLElement>()
const isPaused = ref(false)
const chartInstance = shallowRef<unknown>(null)
const echartsRef = shallowRef<unknown>(null)

// ---- 颜色（适配 vohive 暗色/亮色主题）----
const themeColors = computed(() => {
  const isDark = document.documentElement.classList.contains('dark')
  if (isDark) {
    return {
      primary60: '#00BC7D',
      primary30: 'rgba(0,188,125,0.15)',
      info60: '#38bdf8',
      info30: 'rgba(56,189,248,0.15)',
      baseContent10: 'rgba(255,255,255,0.08)',
      baseContent: 'rgba(255,255,255,0.5)',
      base70: '#1a1a1a',
    }
  }
  return {
    primary60: '#00a364',
    primary30: 'rgba(0,163,100,0.12)',
    info60: '#0284c7',
    info30: 'rgba(2,132,199,0.12)',
    baseContent10: 'rgba(0,0,0,0.06)',
    baseContent: 'rgba(0,0,0,0.45)',
    base70: '#ffffff',
  }
})

// ---- ECharts 懒加载 ----
async function ensureECharts() {
  if (echartsRef.value) return echartsRef.value
  const [core, renderers, charts, comps] = await Promise.all([
    import('echarts/core'),
    import('echarts/renderers'),
    import('echarts/charts'),
    import('echarts/components'),
  ])
  const coreMod = core as unknown as { use: (mods: unknown[]) => void; init: (el: HTMLElement) => unknown }
  const rendererMod = renderers as unknown as { CanvasRenderer: unknown }
  const chartMod = charts as unknown as { LineChart: unknown }
  const compMod = comps as unknown as { GridComponent: unknown; TooltipComponent: unknown; LegendComponent: unknown }
  coreMod.use([rendererMod.CanvasRenderer, chartMod.LineChart, compMod.GridComponent, compMod.TooltipComponent, compMod.LegendComponent])
  echartsRef.value = core
  return core
}

function buildOption() {
  const echarts = echartsRef.value as unknown as { graphic: { LinearGradient: new (...args: unknown[]) => unknown } }
  const c = themeColors.value
  const lastPoint = props.data[0]?.data.at(-1) as ChartPoint | undefined
  const latest = lastPoint ? (Array.isArray(lastPoint.value) ? lastPoint.value[0] : lastPoint.name) : Date.now()

  return {
    animationDurationUpdate: 1000,
    animationEasingUpdate: 'linear',
    legend: {
      bottom: 0,
      data: props.data.map((item) => item.name),
      textStyle: {
        color: c.baseContent,
        fontSize: 10,
      },
    },
    grid: { left: 40, top: 15, right: 8, bottom: 25 },
    tooltip: {
      show: true,
      trigger: 'axis',
      backgroundColor: c.base70,
      borderColor: c.base70,
      borderRadius: 8,
      confine: true,
      padding: [0, 3],
      textStyle: {
        color: c.baseContent,
        fontSize: 11,
      },
      formatter: props.tooltipFormatter,
    },
    xAxis: {
      type: 'time',
      min: latest - (props.windowSeconds - 1) * 1000,
      max: latest - 1000,
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: { show: false },
      axisLabel: { show: false },
    },
    yAxis: {
      type: 'value',
      splitNumber: 4,
      min: 0,
      max: props.yAxisFloor === undefined
        ? undefined
        : (value: { max: number }) => Math.max(value.max, props.yAxisFloor!),
      axisTick: { show: false },
      axisLine: { show: false },
      splitLine: {
        show: true,
        lineStyle: {
          type: 'dashed',
          color: c.baseContent10,
        },
      },
      axisLabel: {
        formatter: props.labelFormatter,
        color: c.baseContent,
        fontSize: 10,
        align: 'left' as const,
        padding: [0, 0, 0, -35],
      },
    },
    series: props.data.map((item, index) => {
      const isLast = index === props.data.length - 1
      const lineColor = isLast ? c.primary60 : c.info60
      const areaColor = isLast ? c.primary30 : c.info30

      return {
        name: item.name,
        type: 'line',
        data: item.data,
        symbol: 'none',
        smooth: true,
        color: lineColor,
        emphasis: { disabled: true },
        lineStyle: { width: 1 },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: lineColor },
            { offset: 1, color: areaColor },
          ]),
        },
      }
    }),
  }
}

function renderChart() {
  if (!chartInstance.value || !echartsRef.value || isPaused.value) return
  const inst = chartInstance.value as { setOption: (opt: unknown) => void }
  inst.setOption(buildOption())
}

let resizeObserver: ResizeObserver | null = null

onMounted(async () => {
  if (!chartRef.value) return
  try {
    await ensureECharts()
    const core = echartsRef.value as unknown as { init: (el: HTMLElement) => { setOption: (opt: unknown) => void; resize: () => void } }
    chartInstance.value = core.init(chartRef.value)
    renderChart()
    resizeObserver = new ResizeObserver(() => {
      const inst = chartInstance.value as { resize: () => void } | null
      inst?.resize()
    })
    resizeObserver.observe(chartRef.value)
  } catch { /* 图表加载失败静默 */ }
})

watch(() => props.data, renderChart, { deep: true })
watch(themeColors, renderChart)
watch(isPaused, (val) => { if (!val) renderChart() })

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  const inst = chartInstance.value as { dispose?: () => void } | null
  inst?.dispose?.()
  chartInstance.value = null
})
</script>

<template>
  <div class="ts-chart-wrap">
    <div ref="chartRef" class="ts-chart" />
    <button
      v-if="showPauseButton"
      class="ts-pause-btn"
      @click="isPaused = !isPaused"
    >
      {{ isPaused ? '▶' : '⏸' }}
    </button>
  </div>
</template>

<style scoped>
.ts-chart-wrap {
  position: relative;
  width: 100%;
  height: 100%;
  overflow: hidden;
}
.ts-chart {
  width: 100%;
  height: 100%;
  min-height: 80px;
}
.ts-pause-btn {
  position: absolute;
  right: 2px;
  bottom: 0;
  border: none;
  background: transparent;
  color: var(--muted-foreground);
  cursor: pointer;
  font-size: 14px;
  padding: 2px 4px;
  opacity: 0.6;
  transition: opacity 0.15s;
}
.ts-pause-btn:hover {
  opacity: 1;
}
</style>
