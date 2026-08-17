<script setup lang="ts">
/**
 * SparklineChart — Surge 风格迷你折线图（面积渐变）
 *
 * 数据格式：{ name: number(timestamp), value: [number, number] }[]
 * 照搬 zashboard SparklineChart 设计，适配 vohive ECharts 懒加载模式。
 */
import { computed, onMounted, onBeforeUnmount, ref, shallowRef, watch } from 'vue'

export type ChartPoint = { name: number; value: [number, number] }

const props = withDefaults(defineProps<{
  data: ChartPoint[]
  yAxisFloor?: number
  color?: 'primary' | 'info'
  name?: string
  windowSeconds?: number
  labelFormatter?: (value: number) => string
  tooltipFormatter?: (value: unknown[]) => string
}>(), {
  yAxisFloor: 1,
  color: 'primary',
  windowSeconds: 60,
})

const chartRef = ref<HTMLElement>()
const chartInstance = shallowRef<unknown>(null)
const echartsRef = shallowRef<unknown>(null)

// ---- 颜色（适配 vohive 暗色/亮色主题）----
const colors = computed(() => {
  const isDark = document.documentElement.classList.contains('dark')
  if (isDark) {
    return {
      line: props.color === 'info' ? '#38bdf8' : '#00BC7D',
      area: props.color === 'info' ? 'rgba(56,189,248,0.15)' : 'rgba(0,188,125,0.15)',
      text: 'rgba(255,255,255,0.5)',
      bg: '#1a1a1a',
    }
  }
  return {
    line: props.color === 'info' ? '#0284c7' : '#00a364',
    area: props.color === 'info' ? 'rgba(2,132,199,0.12)' : 'rgba(0,163,100,0.12)',
    text: 'rgba(0,0,0,0.45)',
    bg: '#ffffff',
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
  const compMod = comps as unknown as { GridComponent: unknown; TooltipComponent: unknown }
  coreMod.use([rendererMod.CanvasRenderer, chartMod.LineChart, compMod.GridComponent, compMod.TooltipComponent])
  echartsRef.value = core
  return core
}

function buildOption() {
  const echarts = echartsRef.value as unknown as { graphic: { LinearGradient: new (...args: unknown[]) => unknown } }
  const latestPoint = props.data.at(-1)
  const latest = latestPoint ? latestPoint.value[0] : Date.now()
  const c = colors.value

  return {
    animationDurationUpdate: 1000,
    animationEasingUpdate: 'linear',
    grid: { left: 0, top: 0, right: props.labelFormatter ? 30 : 0, bottom: 0 },
    tooltip: props.tooltipFormatter
      ? {
          show: true,
          trigger: 'axis',
          backgroundColor: c.bg,
          borderColor: c.bg,
          confine: true,
          padding: [0, 5],
          textStyle: { color: c.text, fontSize: 11 },
          formatter: props.tooltipFormatter,
        }
      : { show: false },
    xAxis: {
      type: 'time',
      show: false,
      min: latest - (props.windowSeconds - 1) * 1000,
      max: latest - 1000,
    },
    yAxis: {
      type: 'value',
      show: true,
      position: 'right',
      splitNumber: 2,
      min: 0,
      max: (val: { max: number }) => Math.max(val.max, props.yAxisFloor),
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: { show: false },
      axisLabel: props.labelFormatter
        ? {
            show: true,
            inside: false,
            fontSize: 9,
            color: c.text,
            margin: 4,
            formatter: (value: number) => (value === 0 ? '' : props.labelFormatter!(value)),
          }
        : { show: false },
    },
    series: [
      {
        type: 'line',
        name: props.name,
        symbol: 'none',
        smooth: true,
        lineStyle: { width: 1.5 },
        data: props.data,
        color: c.line,
        emphasis: { disabled: true },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: c.line },
            { offset: 1, color: c.area },
          ]),
        },
      },
    ],
  }
}

function renderChart() {
  if (!chartInstance.value || !echartsRef.value) return
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
watch(colors, renderChart)

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  const inst = chartInstance.value as { dispose?: () => void } | null
  inst?.dispose?.()
  chartInstance.value = null
})
</script>

<template>
  <div ref="chartRef" class="sparkline-chart" />
</template>

<style scoped>
.sparkline-chart {
  width: 100%;
  height: 100%;
  min-height: 56px;
  min-width: 0;
  overflow: hidden;
}
</style>
