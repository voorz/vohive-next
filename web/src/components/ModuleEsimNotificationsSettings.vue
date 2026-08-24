<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import { devicesService } from '../services/devices'
import { errorMessage } from '../services/http'

const props = defineProps<{
  visible: boolean
  deviceId: string
}>()

const emit = defineEmits<{
  'update:visible': [val: boolean]
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

watch(() => props.visible, async (open) => {
  if (open) {
    await loadSettings()
  }
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

async function saveSettings() {
  saving.value = true
  try {
    const result = await devicesService.updateEsimNotificationSettings(props.deviceId, settings.value as Record<string, unknown>)
    if (!result.ok) throw result.error
    ElMessage.success('通知设置已保存')
    emit('update:visible', false)
  } catch (e: unknown) {
    ElMessage.error(errorMessage(e, '保存通知设置失败'))
  } finally {
    saving.value = false
  }
}

const eventCards = [
  { key: 'install', title: '安装 (Install)', fields: { send: 'auto_send_install', remove: 'auto_remove_install' }, hasDeleteWithoutSending: false },
  { key: 'enable', title: '启用 (Enable)', fields: { send: 'auto_send_enable', remove: 'auto_remove_enable', deleteWithoutSending: 'delete_without_sending_enable' }, hasDeleteWithoutSending: true },
  { key: 'disable', title: '禁用 (Disable)', fields: { send: 'auto_send_disable', remove: 'auto_remove_disable', deleteWithoutSending: 'delete_without_sending_disable' }, hasDeleteWithoutSending: true },
  { key: 'delete', title: '删除 (Delete)', fields: { send: 'auto_send_delete', remove: 'auto_remove_delete' }, hasDeleteWithoutSending: false }
] as const
</script>

<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="(v: boolean) => emit('update:visible', v)"
    title="通知处理设置"
    width="min(520px, 90vw)"
  >
    <div v-if="loading" class="settings-loading">
      <el-icon class="settings-spinner" :size="24"><Loading /></el-icon>
      <span>正在加载设置...</span>
    </div>
    <div v-else class="settings-list">
      <!-- 四个事件类型卡片 -->
      <div v-for="card in eventCards" :key="card.key" class="settings-section">
        <div class="settings-section-title">{{ card.title }}</div>
        <div class="settings-section-body">
          <div class="form-switch-row">
            <div>
              <div class="switch-title">自动发送</div>
              <div class="switch-desc">发送通知到 RSP 服务器</div>
            </div>
            <el-switch v-model="(settings as any)[card.fields.send]" />
          </div>
          <div v-if="(settings as any)[card.fields.send]" class="form-switch-row">
            <div>
              <div class="switch-title">发送后移除</div>
              <div class="switch-desc">发送成功后从卡上删除</div>
            </div>
            <el-switch v-model="(settings as any)[card.fields.remove]" />
          </div>
          <div v-if="card.hasDeleteWithoutSending" class="form-switch-row is-danger">
            <div>
              <div class="switch-title">不发送直接移除</div>
              <div class="switch-desc">跳过发送，直接从卡上删除</div>
            </div>
            <el-switch v-model="(settings as any)[(card.fields as any).deleteWithoutSending]" />
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
            <el-switch v-model="settings.process_initial_load" />
          </div>
          <div class="form-switch-row">
            <div>
              <div class="switch-title">切换 Profile 后自动处理</div>
              <div class="switch-desc">启用或禁用配置文件后处理通知</div>
            </div>
            <el-switch v-model="settings.process_after_switch" />
          </div>
          <div class="form-switch-row">
            <div>
              <div class="switch-title">删除配置后</div>
              <div class="switch-desc">删除配置文件后处理通知</div>
            </div>
            <el-switch v-model="settings.process_after_delete" />
          </div>
          <div class="form-switch-row">
            <div>
              <div class="switch-title">下载前</div>
              <div class="switch-desc">开始下载配置文件前处理通知</div>
            </div>
            <el-switch v-model="settings.process_before_download" />
          </div>
          <div class="form-switch-row">
            <div>
              <div class="switch-title">安装后</div>
              <div class="switch-desc">配置文件安装后处理通知</div>
            </div>
            <el-switch v-model="settings.process_after_install" />
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <el-button @click="emit('update:visible', false)">取消</el-button>
      <el-button type="primary" :loading="saving" @click="saveSettings">保存</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
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

.settings-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-height: 600px;
  overflow-y: auto;
}

.settings-section {
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: visible;
}

.settings-section-title {
  padding: 8px 12px;
  font-size: 13px;
  font-weight: 600;
  background: var(--muted);
  color: var(--foreground);
}

.settings-section-body {
  padding: 12px;
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
