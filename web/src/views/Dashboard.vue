<script setup lang="ts">
import { h, onMounted, onUnmounted, ref, computed, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import DeviceCard from '../components/DeviceCard.vue'
import EmptyState from '../components/EmptyState.vue'
import ListSkeleton from '../components/ListSkeleton.vue'
import ErrorState from '../components/ErrorState.vue'
import RefreshButton from '../components/RefreshButton.vue'
import TrafficAnalysisPanel from '../components/TrafficAnalysisPanel.vue'
import { usePollingScheduler } from '../composables/usePollingScheduler'
import { useDashboardStore } from '../stores/dashboard'
import { useHeaderActionsStore } from '../stores/headerActions'
import type { TrafficRange } from '../services/traffic'

const dashboard = useDashboardStore()
const router = useRouter()
const headerActions = useHeaderActionsStore()
const {
  devices,
  devicesLoading: loading,
  devicesLastOkAt,
  devicesError,
  analysis,
  analysisLoading,
  analysisLastOkAt,
  analysisError
} = storeToRefs(dashboard)

const lastUpdatedAt = ref<number | null>(null)

const analysisRange = ref<TrafficRange>('day')

const totalCount = computed(() => devices.value.length)
const onlineCount = computed(() => devices.value.filter(d => d?.healthy).length)
const offlineCount = computed(() => Math.max(0, totalCount.value - onlineCount.value))

async function fetchDevices() {
  await dashboard.fetchDevices()
  lastUpdatedAt.value = Date.now()
}

async function fetchTrafficAnalysis() {
  await dashboard.fetchAnalysis(analysisRange.value)
}

function handleAnalysisRangeChange(range: TrafficRange) {
  if (analysisRange.value === range) return
  analysisRange.value = range
  void fetchTrafficAnalysis()
}

function openDeviceOverview(id: string) {
  const deviceID = String(id || '').trim()
  if (!deviceID) return
  void router.push({
    name: 'Devices',
    query: {
      device: deviceID,
      tab: 'overview'
    }
  })
}

usePollingScheduler(fetchDevices, 5000, {
  immediate: true,
  maxIntervalMs: 30000,
  backgroundIntervalMs: 15000
})
usePollingScheduler(fetchTrafficAnalysis, 60000, {
  immediate: false,
  maxIntervalMs: 300000,
  backgroundIntervalMs: 120000
})

onMounted(() => {
  const win = window as Window & {
    requestIdleCallback?: (cb: IdleRequestCallback, opts?: IdleRequestOptions) => number
  }
  if (typeof win.requestIdleCallback === 'function') {
    win.requestIdleCallback(() => fetchTrafficAnalysis(), { timeout: 1500 })
  } else {
    setTimeout(fetchTrafficAnalysis, 800)
  }
})

watch(loading, () => {
  headerActions.setActions(h(RefreshButton, { loading: loading.value, onClick: fetchDevices }))
}, { immediate: true })

onUnmounted(() => {
  headerActions.clear()
})
</script>

<template>
  <div>
    <div class="page-stack">
      <!-- 指标卡片 -->
      <div class="grid grid-cols-2 lg:grid-cols-4 gap-0 border-l border-r" style="border-color: var(--border);">
        <div class="metric">
          <span>设备总数</span>
          <strong>{{ totalCount }}</strong>
        </div>
        <div class="metric">
          <span>在线</span>
          <strong style="color: var(--success);">{{ onlineCount }}</strong>
        </div>
        <div class="metric">
          <span>离线</span>
          <strong style="color: var(--destructive);">{{ offlineCount }}</strong>
        </div>
        <div class="metric">
          <span>最近刷新</span>
          <strong style="font-size: 14px; font-weight: 520;">
            {{ lastUpdatedAt ? new Date(lastUpdatedAt).toLocaleTimeString() : '--:--:--' }}
          </strong>
        </div>
      </div>

      <ErrorState
        v-if="devicesError"
        title="设备列表加载失败"
        :message="devicesError.message"
        :status-code="devicesError.status"
        :request-method="devicesError.method"
        :request-url="devicesError.url"
        :last-success-at="devicesLastOkAt"
        retry-text="重试"
        @retry="fetchDevices"
      />

      <ListSkeleton v-if="loading && devices.length === 0" :rows="10" />

      <EmptyState v-else-if="devices.length === 0" title="暂无设备接入" subtitle="请先在设备管理中添加或接管设备" />

      <!-- 设备卡片网格 -->
      <div v-else class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4 gap-4">
        <DeviceCard
          v-for="dev in devices"
          :key="dev.id"
          :device="dev"
          @open-device="openDeviceOverview"
        />
      </div>

      <TrafficAnalysisPanel
        v-if="devices.length > 0 || !loading"
        :analysis="analysis"
        :loading="analysisLoading"
        :error="analysisError"
        :last-ok-at="analysisLastOkAt"
        :range="analysisRange"
        mode="global"
        @update:range="handleAnalysisRangeChange"
        @refresh="fetchTrafficAnalysis"
      />
    </div>
  </div>
</template>
