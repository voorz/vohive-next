/**
 * useNotificationStream — 全局通知 SSE composable
 *
 * 订阅 /notifications/stream，根据 level 分发：
 * - low  → ElNotification 右上角气泡（自动消失）
 * - high → highNotification ref（由调用方渲染自定义浮窗）
 */

import { ref, onBeforeUnmount } from 'vue'
import { ElNotification } from 'element-plus'
import { useEventStream } from './useEventStream'

export type FrontendNotification = {
  level: 'high' | 'low'
  event: string
  title: string
  body: string
  device_id?: string
  device_name?: string
  timestamp: string
}

export function useNotificationStream() {
  const highNotification = ref<FrontendNotification | null>(null)

  const { connect, disconnect } = useEventStream<FrontendNotification>({
    path: '/notifications/stream',
    eventName: 'notification',
    reconnectDelayMs: 5000,
    parse: (payload: string) => JSON.parse(payload) as FrontendNotification,
    onEvent: (n: FrontendNotification) => {
      if (n.level === 'high') {
        highNotification.value = n
      } else {
        ElNotification({
          title: n.title || '通知',
          message: n.body || '',
          type: 'info',
          duration: 5000,
          position: 'top-right',
        })
      }
    },
  })

  function dismissHigh() {
    highNotification.value = null
  }

  onBeforeUnmount(() => {
    disconnect()
  })

  return {
    highNotification,
    dismissHigh,
    connect,
    disconnect,
  }
}
