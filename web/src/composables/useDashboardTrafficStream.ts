/**
 * useDashboardTrafficStream — 仪表盘全局实时流量 SSE composable
 *
 * 订阅 /dashboard/overview/stream，维护 60 秒滚动窗口的
 * 上传/下载速度历史 + 连接数历史，可在仪表盘和二级页面复用。
 */
import { ref, watchEffect, onMounted, onBeforeUnmount, type Ref } from 'vue'
import { useEventStream } from './useEventStream'
import type { ChartPoint } from '../components/SparklineChart.vue'

// ---- 格式化工具 ----

function formatBytes(bytes: number, opts?: { binary?: boolean; suffix?: string }): string {
  const base = opts?.binary ? 1024 : 1000
  const units = opts?.binary
    ? ['B', 'KiB', 'MiB', 'GiB', 'TiB']
    : ['B', 'KB', 'MB', 'GB', 'TB']
  let val = Math.abs(bytes)
  let i = 0
  while (val >= base && i < units.length - 1) { val /= base; i++ }
  const suffix = opts?.suffix || ''
  return `${val.toFixed(i === 0 ? 0 : 1)} ${units[i]}${suffix}`
}

function splitBytes(bytes: number): { value: string; unit: string } {
  const str = formatBytes(bytes)
  const match = str.match(/^([\d.]+)\s*(.*)$/)
  return match ? { value: match[1], unit: match[2] } : { value: str, unit: '' }
}

// ---- 滚动窗口 ----

const WINDOW_SECONDS = 60
const BUFFER_POINTS = 2
const SAVED_POINTS = WINDOW_SECONDS + BUFFER_POINTS

function makeInitHistory(): ChartPoint[] {
  const now = Date.now()
  return new Array(SAVED_POINTS).fill(0).map((_, i) => {
    const ts = now - (SAVED_POINTS - 1 - i) * 1000
    return { name: ts, value: [ts, 0] as [number, number] }
  })
}

function pushHistory(history: ChartPoint[], timestamp: number, value: number): ChartPoint[] {
  history.push({ name: timestamp, value: [timestamp, value] })
  return history.slice(-SAVED_POINTS)
}

// ---- SSE 事件类型 ----

type TrafficEvent = {
  rx_bps: number
  tx_bps: number
  rx_delta_bytes: number
  tx_delta_bytes: number
  total_rx_bytes: number
  total_tx_bytes: number
  device_count: number
  active_devices?: string[]
  timestamp: string
  status: string
}

type OverviewEvent = {
  device_count: number
  online_count: number
  connection_count: number
}

// ---- composable ----

export type DashboardTrafficStreamReturn = {
  downloadSpeed: Ref<number>
  uploadSpeed: Ref<number>
  dlSpeedParts: Ref<{ value: string; unit: string }>
  ulSpeedParts: Ref<{ value: string; unit: string }>
  totalRx: Ref<number>
  totalTx: Ref<number>
  totalRxStr: Ref<string>
  totalTxStr: Ref<string>
  downloadSpeedHistory: Ref<ChartPoint[]>
  uploadSpeedHistory: Ref<ChartPoint[]>
  connectionsHistory: Ref<ChartPoint[]>
  deviceCount: Ref<number>
  onlineCount: Ref<number>
  connectionCount: Ref<number>
  connected: Ref<boolean>
}

export function useDashboardTrafficStream(): DashboardTrafficStreamReturn {
  const downloadSpeed = ref(0)
  const uploadSpeed = ref(0)
  const totalRx = ref(0)
  const totalTx = ref(0)

  const downloadSpeedHistory = ref(makeInitHistory())
  const uploadSpeedHistory = ref(makeInitHistory())
  const connectionsHistory = ref(makeInitHistory())

  const deviceCount = ref(0)
  const onlineCount = ref(0)
  const connectionCount = ref(0)
  const connected = ref(false)

  const dlSpeedParts = ref(splitBytes(0))
  const ulSpeedParts = ref(splitBytes(0))
  const totalRxStr = ref('')
  const totalTxStr = ref('')

  let stream: ReturnType<typeof useEventStream<OverviewEvent>> | null = null

  function handleTrafficEvent(data: TrafficEvent) {
    const ts = Date.now()
    downloadSpeed.value = Math.max(0, data.rx_bps)
    uploadSpeed.value = Math.max(0, data.tx_bps)
    totalRx.value = data.total_rx_bytes
    totalTx.value = data.total_tx_bytes

    downloadSpeedHistory.value = pushHistory(downloadSpeedHistory.value, ts, downloadSpeed.value)
    uploadSpeedHistory.value = pushHistory(uploadSpeedHistory.value, ts, uploadSpeed.value)
  }

  function handleOverviewEvent(data: OverviewEvent) {
    deviceCount.value = data.device_count
    onlineCount.value = data.online_count
    const ts = Date.now()
    connectionCount.value = data.connection_count
    connectionsHistory.value = pushHistory(connectionsHistory.value, ts, data.connection_count)
  }

  onMounted(() => {
    stream = useEventStream<OverviewEvent>({
      path: '/dashboard/overview/stream',
      eventName: 'overview',
      parse: (payload: string) => JSON.parse(payload) as OverviewEvent,
      onEvent: (data: OverviewEvent) => handleOverviewEvent(data),
      onRawEvent: (eventName: string, payload: string) => {
        if (eventName !== 'traffic') return
        try {
          handleTrafficEvent(JSON.parse(payload) as TrafficEvent)
        } catch { /* ignore malformed */ }
      },
      onConnected: () => { connected.value = true },
      reconnectDelayMs: 3000,
    })
    void stream.connect()
  })

  onBeforeUnmount(() => {
    stream?.disconnect()
    connected.value = false
  })

  watchEffect(() => {
    dlSpeedParts.value = splitBytes(downloadSpeed.value)
    ulSpeedParts.value = splitBytes(uploadSpeed.value)
    totalRxStr.value = formatBytes(totalRx.value, { binary: true })
    totalTxStr.value = formatBytes(totalTx.value, { binary: true })
  })

  return {
    downloadSpeed,
    uploadSpeed,
    dlSpeedParts,
    ulSpeedParts,
    totalRx,
    totalTx,
    totalRxStr,
    totalTxStr,
    downloadSpeedHistory,
    uploadSpeedHistory,
    connectionsHistory,
    deviceCount,
    onlineCount,
    connectionCount,
    connected,
  }
}
