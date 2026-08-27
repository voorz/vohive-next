<script setup lang="ts">
import { ref, watch, onBeforeUnmount, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { devicesService } from '../services/devices'
import { errorMessage } from '../services/http'
import { useSensitiveVisibility } from '../composables/useSensitiveVisibility'
import { applyOptimisticActiveState } from './deviceEsimOptimistic'
import { onNotificationCountChange } from '../composables/useEsimNotifications'
import type { EsimChipInfo, EsimEUICCProfiles, EsimProfileItem } from '../types/api'
import ModuleEsimChipCard from './ModuleEsimChipCard.vue'
import ModuleEsimProfileItem from './ModuleEsimProfileItem.vue'
import ModuleEsimDownloadView from './ModuleEsimDownloadView.vue'
import ModuleEsimSettingsDialog from './ModuleEsimSettingsDialog.vue'
import ModuleEsimNotificationsView from './ModuleEsimNotificationsView.vue'
import { Sim24Regular, ArrowDownload24Regular, Alert24Regular } from '@vicons/fluent'

const props = defineProps<{
  deviceId?: string
  deviceImei?: string
  deviceOnline?: boolean
  isPCSC?: boolean
}>()

// Data
const loading = ref(false)
const profilesRefreshing = ref(false)
const chipInfo = ref<EsimChipInfo | null>(null)
const profiles = ref<EsimEUICCProfiles[]>([])
const notificationCount = ref(0)

// Switching state
const switching = ref<string | null>(null)

// Settings dialog
const settingsTarget = ref<{ profile: EsimProfileItem; aidHex: string } | null>(null)
const settingsOpen = ref(false)

// Notifications view
const notificationsViewOpen = ref(false)

// Download view
const downloadViewOpen = ref(false)

// F4: 监听通知数量变化，实时更新红点
let unsubCountChange: (() => void) | null = null
onMounted(() => {
  unsubCountChange = onNotificationCountChange((devId, count) => {
    if (devId === props.deviceId) {
      notificationCount.value = count
    }
  })
})
onBeforeUnmount(() => {
  if (unsubCountChange) unsubCountChange()
})

// Sensitive visibility
const showSensitive = useSensitiveVisibility()

// Data fetching
let fetchAbortController: AbortController | null = null
let fetchRequestId = 0

function reloadEsim() {
  loading.value = true
  fetchOverview(true)
}

async function fetchOverview(refresh = false) {
  if (!props.deviceId) return
  fetchRequestId += 1
  const requestId = fetchRequestId

  if (fetchAbortController) {
    fetchAbortController.abort()
  }
  const controller = new AbortController()
  fetchAbortController = controller

  if (refresh) {
    profilesRefreshing.value = true
  } else {
    loading.value = true
  }

  const result = await devicesService.getEsimOverview(props.deviceId, {
    refresh,
    signal: controller.signal
  })
  let shouldResetLoading = true
  try {
    if (requestId !== fetchRequestId) {
      shouldResetLoading = false
      return
    }
    if (!result.ok) throw result.error
    chipInfo.value = result.data.chipInfo
    profiles.value = result.data.profiles || []
    notificationCount.value = result.data.notificationCount ?? 0
    // 首次加载通知数为 0 时，延迟静默刷新以拿取异步统计结果
    maybeScheduleNotifCountRefresh(notificationCount.value, !refresh)
  } catch (e: unknown) {
    if (result.ok === false && result.error.code === 'ERR_CANCELED') {
      return
    }
    chipInfo.value = null
    profiles.value = []
    notificationCount.value = 0
    ElMessage.error(errorMessage(e, '获取 eSIM 信息失败'))
  } finally {
    if (shouldResetLoading) {
      if (refresh) {
        profilesRefreshing.value = false
      } else {
        loading.value = false
      }
    }
  }
}

// 首次加载后通知数为 0 时，延迟静默刷新一次以拿取异步统计的通知数
// 后端 refreshNotificationCountAsync 在 overview 返回后异步执行，约 1-2s 完成
let notifCountRefreshTimer: ReturnType<typeof setTimeout> | null = null
function maybeScheduleNotifCountRefresh(count: number, wasFirstLoad: boolean) {
  if (notifCountRefreshTimer) {
    clearTimeout(notifCountRefreshTimer)
    notifCountRefreshTimer = null
  }
  if (wasFirstLoad && count === 0) {
    notifCountRefreshTimer = setTimeout(() => {
      notifCountRefreshTimer = null
      void fetchOverviewSilent(true)
    }, 2500)
  }
}

// 切卡后模组恢复需要时间（SIM power cycle + 网络注册），渐进式重试 fetchOverview(true)
// 3s → 6s → 10s → 15s → 22s，最多 5 次。期间静默不弹错误。
let postSwitchRefreshTimer: ReturnType<typeof setTimeout> | null = null
function schedulePostSwitchOverviewRefresh() {
  if (postSwitchRefreshTimer) {
    clearTimeout(postSwitchRefreshTimer)
    postSwitchRefreshTimer = null
  }
  const delays = [3000, 6000, 10000, 15000, 22000]
  let attempt = 0
  const device = props.deviceId
  async function tryRefresh() {
    if (props.deviceId !== device) return // 设备已切换，停止重试
    attempt++
    const ok = await fetchOverviewSilent(true)
    if (ok) return // 成功拿到数据，停止重试
    if (attempt >= delays.length) return // 超时，停止重试
    postSwitchRefreshTimer = setTimeout(() => void tryRefresh(), delays[attempt] - delays[attempt - 1])
  }
  postSwitchRefreshTimer = setTimeout(() => void tryRefresh(), delays[0])
}

// 静默 fetchOverview：成功返回 true，失败返回 false（不弹错误，不清空数据）
async function fetchOverviewSilent(refresh: boolean): Promise<boolean> {
  if (!props.deviceId) return false
  fetchRequestId += 1
  const requestId = fetchRequestId
  if (fetchAbortController) {
    fetchAbortController.abort()
  }
  const controller = new AbortController()
  fetchAbortController = controller
  if (refresh) {
    profilesRefreshing.value = true
  }
  const result = await devicesService.getEsimOverview(props.deviceId, {
    refresh,
    signal: controller.signal
  })
  let shouldResetLoading = true
  try {
    if (requestId !== fetchRequestId) {
      shouldResetLoading = false
      return false
    }
    if (!result.ok) throw result.error
    chipInfo.value = result.data.chipInfo
    profiles.value = result.data.profiles || []
    notificationCount.value = result.data.notificationCount ?? 0
    return true
  } catch {
    return false
  } finally {
    if (shouldResetLoading) {
      if (refresh) {
        profilesRefreshing.value = false
      } else {
        loading.value = false
      }
    }
  }
}

async function switchProfile(iccid: string, state: number, aidHex: string) {
  const action = state === 1 ? '禁用' : '启用'
  const confirmed = await ElMessageBox.confirm(
    `确定要${action}此 Profile (${iccid}) 吗？切换后设备会短暂断网。`,
    `${action} Profile`,
    { confirmButtonText: action, cancelButtonText: '取消', type: 'warning' }
  ).then(() => true).catch(() => false)
  if (!confirmed) return

  switching.value = iccid
  try {
    const result = await devicesService.switchEsimProfile(props.deviceId!, {
      iccid,
      aid_hex: aidHex,
      state
    })
    if (!result.ok) throw new Error(result.error.message || `${action}失败`)
    ElMessage.success(`Profile ${action}成功`)
    profiles.value = applyOptimisticActiveState(profiles.value, iccid, aidHex)
    // 切卡后模组恢复需要时间，渐进式重试 fetchOverview(true) 直到成功或超时
    // 期间静默不弹错误（模组恢复中 APDU 失败是预期行为）
    schedulePostSwitchOverviewRefresh()
  } catch (e: unknown) {
    ElMessage.error(errorMessage(e, `${action}失败`))
  } finally {
    switching.value = null
  }
}

function openSettings(profile: EsimProfileItem, aidHex: string) {
  settingsTarget.value = { profile, aidHex }
  settingsOpen.value = true
}

function onSettingsChanged() {
  void fetchOverview(true)
}

function onDownloaded() {
  void fetchOverview(true)
}

// Watch deviceId
watch(() => props.deviceId, (newId) => {
  if (fetchAbortController) {
    fetchAbortController.abort()
  }
  notificationsViewOpen.value = false
  downloadViewOpen.value = false
  if (!newId) {
    chipInfo.value = null
    profiles.value = []
    notificationCount.value = 0
    return
  }
  fetchOverview()
}, { immediate: true })

onBeforeUnmount(() => {
  if (fetchAbortController) {
    fetchAbortController.abort()
  }
  if (postSwitchRefreshTimer) {
    clearTimeout(postSwitchRefreshTimer)
    postSwitchRefreshTimer = null
  }
  if (notifCountRefreshTimer) {
    clearTimeout(notifCountRefreshTimer)
    notifCountRefreshTimer = null
  }
})
</script>

<template>
  <div class="preview-panel">
    <!-- 通知视图模式 -->
    <ModuleEsimNotificationsView
      v-if="notificationsViewOpen"
      :device-id="deviceId || ''"
      :chip-info="chipInfo"
      :show-sensitive="showSensitive"
      @back="notificationsViewOpen = false"
      @count-change="(n: number) => notificationCount = n"
    />

    <!-- 下载视图模式 -->
    <ModuleEsimDownloadView
      v-else-if="downloadViewOpen"
      :device-id="deviceId || ''"
      :chip-info="chipInfo"
      :device-imei="deviceImei"
      @back="downloadViewOpen = false"
      @downloaded="onDownloaded"
    />

    <!-- 主页面 (eSIM 列表) -->
    <template v-else>
      <!-- 头部 (60px) -->
      <div class="preview-header">
        <div class="preview-header-left">
          <div class="preview-header-icon">
            <el-icon size="20"><Sim24Regular /></el-icon>
          </div>
          <div class="preview-title">eSIM 管理</div>
        </div>
        <div class="preview-header-actions">
          <button
            class="preview-tab-btn"
            title="下载 eSIM"
            @click="downloadViewOpen = true"
          >
            <el-icon size="20"><ArrowDownload24Regular /></el-icon>
          </button>
          <button
            class="preview-tab-btn"
            title="通知管理"
            @click="notificationsViewOpen = true"
          >
            <el-icon size="20"><Alert24Regular /></el-icon>
            <span v-if="notificationCount > 0" class="preview-tab-badge">{{ notificationCount > 99 ? '99+' : notificationCount }}</span>
          </button>
        </div>
      </div>

      <!-- 无设备 -->
      <div v-if="!deviceId" class="preview-empty">
        <el-empty description="选择设备查看 eSIM" :image-size="60" />
      </div>

      <!-- 加载中 -->
      <div v-else-if="loading" class="preview-loading">
        <div class="preview-loading-spinner" />
        <span>正在加载 eSIM 信息...</span>
      </div>

      <!-- eSIM 列表 -->
      <template v-else>
        <div v-if="chipInfo" class="preview-chip-area">
          <ModuleEsimChipCard
            :chip-info="chipInfo"
            :show-sensitive="showSensitive"
            :refreshing="profilesRefreshing"
            @refresh="fetchOverview(true)"
            @toggle-sensitive="showSensitive = !showSensitive"
          />
        </div>

        <div class="preview-profile-wrapper">
          <div class="preview-profile-scroll">
            <template v-if="profiles.length > 0">
              <template v-for="group in profiles" :key="group.aid_hex || group.eid">
                <ModuleEsimProfileItem
                  v-for="p in group.profiles"
                  :key="p.iccid"
                  :profile="p"
                  :aid-hex="group.aid_hex"
                  :show-sensitive="showSensitive"
                  :switching="switching === p.iccid"
                  @switch="switchProfile"
                  @open-settings="openSettings"
                />
              </template>
            </template>
            <div v-else-if="!chipInfo" class="preview-empty">
              <span class="preview-empty-text">未检测到 eUICC 或 eUICC 出现问题，请检查读卡器或查看日志</span>
              <el-button size="small" @click="reloadEsim()" class="preview-empty-btn">刷新</el-button>
            </div>
            <div v-else class="preview-empty">
              <el-empty description="暂无 Profile" :image-size="60" />
            </div>
          </div>
        </div>
      </template>

      <!-- 底部预留栏 -->
      <div class="preview-footer" />

      <!-- 设置弹窗 -->
      <ModuleEsimSettingsDialog
        v-model:visible="settingsOpen"
        :profile="settingsTarget?.profile || null"
        :aid-hex="settingsTarget?.aidHex || ''"
        :device-id="deviceId || ''"
        :device-online="deviceOnline"
        :is-p-c-s-c="isPCSC"
        @renamed="onSettingsChanged"
        @deleted="onSettingsChanged"
        @policy-changed="onSettingsChanged"
      />
    </template>
  </div>
</template>

<style scoped>
.preview-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--card);
  overflow: hidden;
}

/* 头部 — 60px */
.preview-header {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 0 12px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.preview-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.preview-header-icon {
  width: 38px;
  height: 38px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  background: var(--background);
  border: 1px solid var(--border);
  color: var(--foreground);
}

.preview-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--foreground);
}

/* 芯片卡区域 — 固定，有 padding */
.preview-chip-area {
  flex-shrink: 0;
  padding: 12px;
}

/* Profile 列表 — 外层 wrapper */
.preview-profile-wrapper {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  position: relative;
  border-top: 1px solid var(--border);
}

/* Profile 列表 — 内层滚动 */
.preview-profile-scroll {
  height: 100%;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
}

/* 头部右侧图标按钮 — 参照 notif-tab-btn (32x32, 圆角6px) */
.preview-header-actions {
  display: flex;
  align-items: center;
  gap: 2px;
}

.preview-tab-btn {
  position: relative;
  width: 32px;
  height: 32px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--muted-foreground);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.12s;
}
.preview-tab-btn:hover {
  background: var(--accent);
  color: var(--foreground);
}

.preview-tab-badge {
  position: absolute;
  top: -2px;
  right: -2px;
  min-width: 14px;
  height: 14px;
  padding: 0 3px;
  border-radius: 7px;
  background: #ef4444;
  color: #fff;
  font-size: 9px;
  font-weight: 700;
  line-height: 14px;
  text-align: center;
  pointer-events: none;
  box-shadow: 0 0 0 1.5px var(--card);
}

/* 底部预留栏 */
.preview-footer {
  flex-shrink: 0;
  border-top: 1px solid var(--border);
  padding: 8px 12px;
  min-height: 40px;
}

.preview-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  flex: 1;
  min-height: 200px;
  color: var(--muted-foreground);
  font-size: 13px;
}

.preview-empty-text {
  text-align: center;
  max-width: 280px;
  line-height: 1.5;
}

.preview-empty-btn {
  border: 1px solid var(--border);
  border-radius: 6px;
}

.preview-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  flex: 1;
  min-height: 200px;
  color: var(--muted-foreground);
  font-size: 12px;
}

.preview-loading-spinner {
  width: 24px;
  height: 24px;
  border: 2px solid var(--border);
  border-top-color: var(--brand);
  border-radius: 999px;
  animation: preview-spin 0.8s linear infinite;
}

@keyframes preview-spin {
  to { transform: rotate(360deg); }
}
</style>
