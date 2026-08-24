<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import { Info24Regular } from '@vicons/fluent'
import { devicesService } from '../services/devices'
import { errorMessage } from '../services/http'

const props = defineProps<{
  deviceId: string
}>()

type Settings = {
  device_id?: string
  auto_send_install: boolean
  auto_remove_install: boolean
  auto_send_enable: boolean
  auto_remove_enable: boolean
  delete_without_sending_enable: boolean
  auto_send_disable: boolean
  auto_remove_disable: boolean
  delete_without_sending_disable: boolean
  auto_send_delete: boolean
  auto_remove_delete: boolean
  process_initial_load: boolean
  process_after_switch: boolean
  process_after_delete: boolean
  process_before_download: boolean
  process_after_install: boolean
}

const defaultSettings: Settings = {
  auto_send_install: true,
  auto_remove_install: true,
  auto_send_enable: true,
  auto_remove_enable: true,
  delete_without_sending_enable: false,
  auto_send_disable: true,
  auto_remove_disable: true,
  delete_without_sending_disable: false,
  auto_send_delete: true,
  auto_remove_delete: false,
  process_initial_load: true,
  process_after_switch: true,
  process_after_delete: true,
  process_before_download: true,
  process_after_install: true
}

const settings = ref<Settings>({ ...defaultSettings })
const loading = ref(false)
const saving = ref(false)

onMounted(() => {
  loadSettings()
})

async function loadSettings() {
  loading.value = true
  try {
    const result = await devicesService.getEsimNotificationSettings(props.deviceId)
    if (result.ok) {
      settings.value = { ...defaultSettings, ...result.data }
    } else {
      throw result.error
    }
  } catch (e: unknown) {
    ElMessage.error(errorMessage(e, '获取通知设置失败'))
  } finally {
    loading.value = false
  }
}

// 防抖保存：switch 切换后 500ms 自动提交
let saveTimer: ReturnType<typeof setTimeout> | null = null

function onSwitchChange() {
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = setTimeout(() => {
    void saveSettings()
  }, 500)
}

async function saveSettings() {
  if (saving.value) return
  saving.value = true
  try {
    const result = await devicesService.updateEsimNotificationSettings(props.deviceId, settings.value as Record<string, unknown>)
    if (!result.ok) throw result.error
    ElMessage.success({ message: '设置已保存', duration: 1500 })
  } catch (e: unknown) {
    ElMessage.error(errorMessage(e, '保存通知设置失败'))
  } finally {
    saving.value = false
  }
}

onBeforeUnmount(() => {
  if (saveTimer) clearTimeout(saveTimer)
})

const eventCards = [
  { key: 'install', title: '安装 (Install)', fields: { send: 'auto_send_install', remove: 'auto_remove_install' }, hasDeleteWithoutSending: false },
  { key: 'enable', title: '启用 (Enable)', fields: { send: 'auto_send_enable', remove: 'auto_remove_enable', deleteWithoutSending: 'delete_without_sending_enable' }, hasDeleteWithoutSending: true },
  { key: 'disable', title: '禁用 (Disable)', fields: { send: 'auto_send_disable', remove: 'auto_remove_disable', deleteWithoutSending: 'delete_without_sending_disable' }, hasDeleteWithoutSending: true },
  { key: 'delete', title: '删除 (Delete)', fields: { send: 'auto_send_delete', remove: 'auto_remove_delete' }, hasDeleteWithoutSending: false }
] as const
</script>

<template>
  <div class="notif-settings-view">
    <div v-if="loading" class="settings-loading">
      <el-icon class="settings-spinner" :size="24"><Loading /></el-icon>
      <span>正在加载设置...</span>
    </div>
    <template v-else>
      <!-- 提示卡片 -->
      <div class="settings-info-card">
        <div class="settings-info-icon">
          <el-icon size="16"><Info24Regular /></el-icon>
        </div>
        <div class="settings-info-text">
          处理通知有助于您的 eUICC 与 SM-DP+ 服务器（运营商）之间的同步。删除已发送的通知可以保持卡存储清洁。
        </div>
      </div>

      <!-- 四个事件类型卡片 -->>
      <div v-for="card in eventCards" :key="card.key" class="settings-section">
        <div class="settings-section-title">{{ card.title }}</div>
        <div class="settings-section-body">
          <div class="form-switch-row">
            <div>
              <div class="switch-title">自动发送</div>
              <div class="switch-desc">发送通知到 RSP 服务器</div>
            </div>
            <el-switch v-model="(settings as any)[card.fields.send]" @change="onSwitchChange" />
          </div>
          <div v-if="(settings as any)[card.fields.send]" class="form-switch-row">
            <div>
              <div class="switch-title">发送后移除</div>
              <div class="switch-desc">发送成功后从卡上删除</div>
            </div>
            <el-switch v-model="(settings as any)[card.fields.remove]" @change="onSwitchChange" />
          </div>
          <div v-if="card.hasDeleteWithoutSending" class="form-switch-row is-danger">
            <div>
              <div class="switch-title">不发送直接移除</div>
              <div class="switch-desc">跳过发送，直接从卡上删除</div>
            </div>
            <el-switch v-model="(settings as any)[(card.fields as any).deleteWithoutSending]" @change="onSwitchChange" />
          </div>
        </div>
      </div>

      <!-- 处理时机卡片 -->
      <div class="settings-section">
        <div class="settings-section-title">处理时机</div>
        <div class="settings-section-body">
          <div class="form-switch-row">
            <div>
              <div class="switch-title">打开页面时自动处理</div>
              <div class="switch-desc">配置文件完成加载后处理通知</div>
            </div>
            <el-switch v-model="settings.process_initial_load" @change="onSwitchChange" />
          </div>
          <div class="form-switch-row">
            <div>
              <div class="switch-title">切换 Profile 后自动处理</div>
              <div class="switch-desc">启用或禁用配置文件后处理通知</div>
            </div>
            <el-switch v-model="settings.process_after_switch" @change="onSwitchChange" />
          </div>
          <div class="form-switch-row">
            <div>
              <div class="switch-title">删除配置后</div>
              <div class="switch-desc">删除配置文件后处理通知</div>
            </div>
            <el-switch v-model="settings.process_after_delete" @change="onSwitchChange" />
          </div>
          <div class="form-switch-row">
            <div>
              <div class="switch-title">下载前</div>
              <div class="switch-desc">开始下载配置文件前处理通知</div>
            </div>
            <el-switch v-model="settings.process_before_download" @change="onSwitchChange" />
          </div>
          <div class="form-switch-row">
            <div>
              <div class="switch-title">安装后</div>
              <div class="switch-desc">配置文件安装后处理通知</div>
            </div>
            <el-switch v-model="settings.process_after_install" @change="onSwitchChange" />
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
/* 设置面板 — 参照 ModuleEsimDownload .download-container */
.notif-settings-view {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.settings-loading {
  padding: 40px 0;
  text-align: center;
  color: var(--muted-foreground);
  font-size: 13px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}

.settings-spinner {
  animation: settings-spin 0.8s linear infinite;
}

@keyframes settings-spin {
  to { transform: rotate(360deg); }
}

/* 提示卡片 — 参照 chip-card 样式 */
.settings-info-card {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--muted);
}

.settings-info-icon {
  width: 28px;
  height: 28px;
  border-radius: 5px;
  background: var(--background);
  border: 1px solid var(--border);
  color: var(--brand);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.settings-info-text {
  font-size: 12px;
  line-height: 1.6;
  color: var(--muted-foreground);
  padding-top: 4px;
}

/* 设置卡片 — 参照 ModuleEsimDownload .download-section */
.settings-section {
  border: 1px solid var(--border);
  border-radius: 6px;
  overflow: hidden;
}

/* 卡片标题 — 参照 .download-section-header */
.settings-section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px;
  font-size: 12px;
  font-weight: 700;
  background: var(--muted);
  border-bottom: 1px solid var(--border);
  color: var(--foreground);
}

/* 卡片内容 — 参照 .download-section-body */
.settings-section-body {
  padding: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

/* 标准开关行 — 参照 ModuleCardPolicy / CarrierConfigForm .form-switch-row */
.form-switch-row {
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  background: var(--muted);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 10px 12px;
}

.switch-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
}

.switch-desc {
  font-size: 11px;
  color: var(--muted-foreground);
  margin-top: 1px;
}

.form-switch-row.is-danger .switch-title {
  color: var(--destructive, #ef4444);
}

.form-switch-row.is-danger .switch-desc {
  color: var(--destructive, #ef4444);
  opacity: 0.7;
}

.form-switch-row :deep(.el-switch) {
  --el-switch-on-color: var(--brand);
  --el-switch-off-color: var(--muted-foreground);
}
</style>
