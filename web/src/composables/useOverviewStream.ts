/**
 * SSE Overview Stream + 实时流量
 *
 * 订阅 /devices/:id/overview/stream，实时更新 device detail，
 * 并处理 traffic 事件维护 60 秒滚动窗口。
 */

import { ref, watch, onMounted, onBeforeUnmount, type Ref } from 'vue'
import type { DeviceOverviewItem, RealtimeTrafficSnapshot } from '../types/api'
import { useEventStream } from './useEventStream'

// ---- 格式化工具 ----

function formatBytes(bytes: unknown): string {
  const v = Number(bytes) || 0
  const units = ['B', 'KB', 'MB', 'GB']
  let val = v
  let i = 0
  while (val >= 1024 && i < units.length - 1) { val /= 1024; i++ }
  return `${val.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

function formatBytesPerSecond(bps: unknown): string {
  const v = Number(bps) || 0
  const units = ['B/s', 'KB/s', 'MB/s', 'GB/s']
  let val = v
  let i = 0
  while (val >= 1024 && i < units.length - 1) { val /= 1024; i++ }
  return `${val.toFixed(i === 0 ? 0 : 1)}${units[i]}`
}

// ---- 滚动窗口 ----

type RollingTrafficSample = { at: number; rxBytes: number; txBytes: number }

const REALTIME_TRAFFIC_WINDOW_MS = 60_000

// ---- composable ----

export type UseOverviewStreamOptions = {
  /** 设备 ID（响应式 getter） */
  deviceId: () => string | undefined
  /** 要更新的 detail ref */
  detail: Ref<DeviceOverviewItem | null>
}

export type OverviewStreamReturn = {
  trafficSpeedRx: Ref<string>
  trafficSpeedTx: Ref<string>
  rollingMinuteRx: Ref<string>
  rollingMinuteTx: Ref<string>
}

export function useOverviewStream(options: UseOverviewStreamOptions): OverviewStreamReturn {
  const { deviceId, detail } = options

  const trafficSpeedRx = ref('')
  const trafficSpeedTx = ref('')
  const rollingMinuteRx = ref('')
  const rollingMinuteTx = ref('')
  const realtimeTrafficActiveUntil = ref(0)

  let rollingTrafficWindow: RollingTrafficSample[] = []

  // ---- 滚动窗口操作 ----

  function resetRollingTrafficWindow() {
    rollingTrafficWindow = []
    rollingMinuteRx.value = ''
    rollingMinuteTx.value = ''
  }

  function setRollingTrafficWindowStatus(value: string) {
    rollingTrafficWindow = []
    rollingMinuteRx.value = value
    rollingMinuteTx.value = value
  }

  function updateRollingTrafficWindow(rxDeltaBytes: unknown, txDeltaBytes: unknown, at = Date.now()) {
    const cutoff = at - REALTIME_TRAFFIC_WINDOW_MS
    rollingTrafficWindow = rollingTrafficWindow.filter(sample => sample.at >= cutoff)
    rollingTrafficWindow.push({
      at,
      rxBytes: Math.max(0, Number(rxDeltaBytes) || 0),
      txBytes: Math.max(0, Number(txDeltaBytes) || 0),
    })
    let rxBytes = 0
    let txBytes = 0
    for (const sample of rollingTrafficWindow) {
      rxBytes += sample.rxBytes
      txBytes += sample.txBytes
    }
    rollingMinuteRx.value = formatBytes(rxBytes)
    rollingMinuteTx.value = formatBytes(txBytes)
  }

  // ---- 流量速度计算 ----

  function updateTrafficSpeedFromDetail() {
    const d = detail.value
    if (!d || !d.network_connected || !d.traffic_raw || d.traffic_meta?.status !== 'ok') {
      trafficSpeedRx.value = ''
      trafficSpeedTx.value = ''
      resetRollingTrafficWindow()
      return
    }
    if (Date.now() < realtimeTrafficActiveUntil.value) return
    const rx = Number(d.traffic_raw?.bytes_received ?? 0)
    const tx = Number(d.traffic_raw?.bytes_sent ?? 0)
    trafficSpeedRx.value = formatBytesPerSecond(Math.max(0, rx) / 60)
    trafficSpeedTx.value = formatBytesPerSecond(Math.max(0, tx) / 60)
    resetRollingTrafficWindow()
  }

  // ---- SSE 事件处理 ----

  function handleRealtimeTrafficEvent(data: RealtimeTrafficSnapshot) {
    const id = deviceId()
    if (!data || data.device_id !== id) return
    realtimeTrafficActiveUntil.value = Date.now() + 2500
    if (data.status === 'ok') {
      trafficSpeedRx.value = formatBytesPerSecond(Math.max(0, Number(data.rx_bps) || 0))
      trafficSpeedTx.value = formatBytesPerSecond(Math.max(0, Number(data.tx_bps) || 0))
      updateRollingTrafficWindow(data.rx_delta_bytes, data.tx_delta_bytes)
      return
    }
    if (data.status === 'waiting_sample') {
      trafficSpeedRx.value = '等待采样'
      trafficSpeedTx.value = '等待采样'
      setRollingTrafficWindowStatus('等待采样')
      return
    }
    if (data.status === 'reset') {
      trafficSpeedRx.value = formatBytesPerSecond(0)
      trafficSpeedTx.value = formatBytesPerSecond(0)
      setRollingTrafficWindowStatus(formatBytes(0))
      return
    }
    trafficSpeedRx.value = '采样中断'
    trafficSpeedTx.value = '采样中断'
    setRollingTrafficWindowStatus('采样中断')
  }

  // ---- SSE 连接管理 ----

  type OverviewSSEPayload = { devices?: DeviceOverviewItem[] }

  let overviewStream: ReturnType<typeof useEventStream<OverviewSSEPayload>> | null = null

  function setupSSE() {
    if (overviewStream) {
      overviewStream.disconnect()
      overviewStream = null
    }
    realtimeTrafficActiveUntil.value = 0
    trafficSpeedRx.value = ''
    trafficSpeedTx.value = ''
    resetRollingTrafficWindow()

    const id = deviceId()
    if (!id) return

    overviewStream = useEventStream<OverviewSSEPayload>({
      path: `/devices/${id}/overview/stream`,
      eventName: 'overview',
      reconnectDelayMs: 3000,
      parse: (payload: string) => JSON.parse(payload) as OverviewSSEPayload,
      onEvent: (data: OverviewSSEPayload) => {
        if (!data?.devices?.length) return
        detail.value = data.devices[0]
        updateTrafficSpeedFromDetail()
      },
      onRawEvent: (eventName: string, payload: string) => {
        if (eventName !== 'traffic') return
        try {
          handleRealtimeTrafficEvent(JSON.parse(payload) as RealtimeTrafficSnapshot)
        } catch { /* ignore malformed frame */ }
      },
    })
    void overviewStream.connect()
  }

  // ---- 生命周期 ----

  watch(() => deviceId(), () => setupSSE())
  onMounted(() => setupSSE())
  onBeforeUnmount(() => {
    overviewStream?.disconnect()
  })

  return {
    trafficSpeedRx,
    trafficSpeedTx,
    rollingMinuteRx,
    rollingMinuteTx,
  }
}
