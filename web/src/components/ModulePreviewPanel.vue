<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { devicesService } from '../services/devices'
import { errorMessage } from '../services/http'
import { useSensitiveVisibility } from '../composables/useSensitiveVisibility'
import { applyOptimisticActiveState } from './deviceEsimOptimistic'
import type { EsimChipInfo, EsimEUICCProfiles, EsimProfileItem } from '../types/api'
import ModuleEsimChipCard from './ModuleEsimChipCard.vue'
import ModuleEsimProfileItem from './ModuleEsimProfileItem.vue'
import ModuleEsimDownload from './ModuleEsimDownload.vue'
import ModuleEsimSettingsDialog from './ModuleEsimSettingsDialog.vue'
import ModuleEsimNotificationsDialog from './ModuleEsimNotificationsDialog.vue'
import { Sim24Regular } from '@vicons/fluent'

const props = defineProps<{
  deviceId?: string
  deviceImei?: string
  deviceOnline?: boolean
}>()

// Tab
const activeTab = ref<'preview' | 'download'>('preview')

// Data
const loading = ref(false)
const profilesRefreshing = ref(false)
const chipInfo = ref<EsimChipInfo | null>(null)
const profiles = ref<EsimEUICCProfiles[]>([])

// Switching state
const switching = ref<string | null>(null)

// Settings dialog
const settingsTarget = ref<{ profile: EsimProfileItem; aidHex: string } | null>(null)
const settingsOpen = ref(false)

// Notifications dialog
const notificationsOpen = ref(false)

// Sensitive visibility
const showSensitive = useSensitiveVisibility()

// Data fetching
let fetchAbortController: AbortController | null = null
let fetchRequestId = 0

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
    profiles.value = result.data.profiles
  } catch (e: unknown) {
    if (result.ok === false && result.error.code === 'ERR_CANCELED') {
      return
    }
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

// Switch profile
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
      aid_hex: aidHex
    })
    if (!result.ok) throw new Error(result.error.message || `${action}失败`)
    ElMessage.success(`Profile ${action}成功`)
    profiles.value = applyOptimisticActiveState(profiles.value, iccid, aidHex)
  } catch (e: unknown) {
    ElMessage.error(errorMessage(e, `${action}失败`))
  } finally {
    switching.value = null
  }
}

// Settings dialog
function openSettings(profile: EsimProfileItem, aidHex: string) {
  settingsTarget.value = { profile, aidHex }
  settingsOpen.value = true
}

function onSettingsChanged() {
  void fetchOverview(true)
}

// Download completed
function onDownloaded() {
  void fetchOverview(true)
}

// Watch deviceId
watch(() => props.deviceId, (newId) => {
  if (fetchAbortController) {
    fetchAbortController.abort()
  }
  if (!newId) {
    chipInfo.value = null
    profiles.value = []
    return
  }
  fetchOverview()
}, { immediate: true })

onBeforeUnmount(() => {
  if (fetchAbortController) {
    fetchAbortController.abort()
  }
})
</script>

<template>
  <div class="preview-panel">
    <!-- 头部 (60px) -->
    <div class="preview-header">
      <div class="preview-header-left">
        <div class="preview-header-icon">
          <el-icon size="20"><Sim24Regular /></el-icon>
        </div>
        <div class="preview-title">eSIM 管理</div>
      </div>
      <!-- Tab 切换 -->
      <div class="preview-mode-tabs">
        <button
          class="preview-mode-tab"
          :class="{ active: activeTab === 'preview' }"
          @click="activeTab = 'preview'"
        >预览</button>
        <button
          class="preview-mode-tab"
          :class="{ active: activeTab === 'download' }"
          @click="activeTab = 'download'"
        >下载</button>
      </div>
    </div>

    <!-- 无设备 -->
    <div v-if="!deviceId" class="preview-empty">
      选择设备查看 eSIM
    </div>

    <!-- 加载中 -->
    <div v-else-if="loading" class="preview-loading">
      <div class="preview-loading-spinner" />
      <span>正在加载 eSIM 信息...</span>
    </div>

    <!-- 预览模式 -->
    <template v-else-if="activeTab === 'preview'">
      <!-- 芯片信息卡区域（固定，有 padding） -->
      <div v-if="chipInfo" class="preview-chip-area">
        <ModuleEsimChipCard
          :chip-info="chipInfo"
          :show-sensitive="showSensitive"
          :refreshing="profilesRefreshing"
          @refresh="fetchOverview(true)"
          @open-notifications="notificationsOpen = true"
          @toggle-sensitive="showSensitive = !showSensitive"
        />
      </div>

      <!-- 全宽分割线 -->
      <div v-if="chipInfo" class="preview-section-divider" />

      <!-- Profile 列表（外层阴影 wrapper + 内层滚动） -->
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
            未检测到 eUICC
          </div>
          <div v-else class="preview-empty">
            暂无 Profile
          </div>
        </div>
      </div>
    </template>

    <!-- 下载模式 -->
    <template v-else-if="activeTab === 'download'">
      <div class="preview-download-scroll">
        <div v-if="!chipInfo" class="preview-empty">
          未检测到 eUICC，无法下载
        </div>
        <ModuleEsimDownload
          v-else
          :device-id="deviceId"
          :chip-info="chipInfo"
          :device-imei="deviceImei"
          @downloaded="onDownloaded"
        />
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
      @renamed="onSettingsChanged"
      @deleted="onSettingsChanged"
      @policy-changed="onSettingsChanged"
    />

    <!-- 通知弹窗 -->
    <ModuleEsimNotificationsDialog
      v-model:visible="notificationsOpen"
      :device-id="deviceId || ''"
    />
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
  padding: 0 14px;
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

/* Tab 切换 */
.preview-mode-tabs {
  display: flex;
  gap: 2px;
  flex-shrink: 0;
}

.preview-mode-tab {
  padding: 4px 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: transparent;
  color: var(--muted-foreground);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.12s;
}

.preview-mode-tab:hover {
  background: var(--accent);
  color: var(--foreground);
}

.preview-mode-tab.active {
  background: var(--background);
  border-color: var(--border);
  color: var(--foreground);
  box-shadow: var(--console-shadow-sm);
}

/* 芯片卡区域 — 固定，有 padding */
.preview-chip-area {
  flex-shrink: 0;
  padding: 10px;
}

/* 全宽分割线 — 连接外框 */
.preview-section-divider {
  height: 1px;
  background: var(--border);
  flex-shrink: 0;
}

/* Profile 列表 — 外层 wrapper（下沉式内阴影） */
.preview-profile-wrapper {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  position: relative;
  box-shadow: inset 0 4px 6px -3px rgba(0,0,0,0.12), inset 0 -4px 6px -3px rgba(0,0,0,0.12);
}

/* Profile 列表 — 内层滚动 */
.preview-profile-scroll {
  height: 100%;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 10px;
}

/* 下载模式 — 独立滚动 */
.preview-download-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 10px;
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
  align-items: center;
  justify-content: center;
  flex: 1;
  min-height: 200px;
  color: var(--muted-foreground);
  font-size: 13px;
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
