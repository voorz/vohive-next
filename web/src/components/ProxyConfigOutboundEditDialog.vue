<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { ElMessage } from 'element-plus'
import type { OutboundInstanceWithStatus } from '../types/proxy-config'
import type { ProxyDevice, ProxyMode } from '../types/api'

const props = defineProps<{
  visible: boolean
  editing: OutboundInstanceWithStatus | null
  devices: ProxyDevice[]
}>()

const emit = defineEmits<{
  'update:visible': [val: boolean]
  'save': [form: OutboundInstanceWithStatus]
}>()

const modeOptions: Array<{ label: string; value: ProxyMode }> = [
  { label: 'SOCKS5', value: 'socks5' },
  { label: 'HTTP', value: 'http' }
]

const form = ref<OutboundInstanceWithStatus>({
  id: '',
  name: '',
  device_id: '',
  enabled: true,
  mode: 'socks5',
  listen_addr: '0.0.0.0',
  listen_port: 10800,
  auth_enabled: false,
  username: '',
  password: '',
  running: false,
  last_error: ''
})

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val)
})

const isEditing = computed(() => !!props.editing)
const title = computed(() => isEditing.value ? '编辑代理实例' : '新增代理实例')

watch(() => props.visible, (val) => {
  if (val) {
    if (props.editing) {
      form.value = { ...props.editing }
    } else {
      form.value = {
        id: `proxy-${Date.now()}`,
        name: '',
        device_id: props.devices[0]?.id || '',
        enabled: true,
        mode: 'socks5',
        listen_addr: '0.0.0.0',
        listen_port: 10800,
        auth_enabled: false,
        username: '',
        password: '',
        running: false,
        last_error: ''
      }
    }
  }
})

function handleSave() {
  if (!form.value.id.trim()) {
    ElMessage.warning('实例 ID 不能为空')
    return
  }
  if (!form.value.device_id) {
    ElMessage.warning('必须绑定设备')
    return
  }
  if (form.value.listen_port <= 0 || form.value.listen_port > 65535) {
    ElMessage.warning('监听端口无效')
    return
  }
  if (form.value.auth_enabled) {
    if (!form.value.username.trim() || !form.value.password.trim()) {
      ElMessage.warning('启用认证时必须填写用户名和密码')
      return
    }
  } else {
    form.value.username = ''
    form.value.password = ''
  }
  emit('save', { ...form.value })
}
</script>

<template>
  <el-dialog
    v-model="dialogVisible"
    :title="title"
    width="560px"
    :close-on-click-modal="false"
    align-center
  >
    <div class="space-y-6 pb-2">
      <!-- Section: 基础设置 -->
      <div class="space-y-4">
        <div class="pc-dlg-section-header">
          <div class="pc-dlg-section-bar" style="background: var(--brand);" />
          <h3 class="pc-dlg-section-title">基础设置</h3>
        </div>

        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-1">
            <label class="pc-form-label">实例 ID</label>
            <el-input
              v-model="form.id"
              :disabled="isEditing"
              placeholder="唯一标识"
            />
          </div>
          <div class="space-y-1">
            <label class="pc-form-label">名称</label>
            <el-input v-model="form.name" placeholder="显示名称" />
          </div>
        </div>

        <div class="space-y-1">
          <label class="pc-form-label">绑定设备</label>
          <el-select v-model="form.device_id" placeholder="选择设备" class="w-full">
            <el-option
              v-for="d in devices"
              :key="d.id"
              :label="`${d.name} (${d.interface})`"
              :value="d.id"
            />
          </el-select>
        </div>

        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-1">
            <label class="pc-form-label">代理模式</label>
            <el-select v-model="form.mode" class="w-full">
              <el-option
                v-for="opt in modeOptions"
                :key="opt.value"
                :label="opt.label"
                :value="opt.value"
              />
            </el-select>
          </div>
          <div class="space-y-1">
            <label class="pc-form-label">监听端口</label>
            <el-input-number v-model="form.listen_port" :min="1" :max="65535" class="!w-full" />
          </div>
        </div>

        <div class="pc-switch-row">
          <div>
            <div class="pc-switch-title">启用实例</div>
            <div class="pc-switch-desc">禁用后实例不会自动启动</div>
          </div>
          <el-switch v-model="form.enabled" />
        </div>
      </div>

      <!-- Section: 认证设置 -->
      <div class="space-y-4">
        <div class="pc-dlg-section-header">
          <div class="pc-dlg-section-bar" style="background: var(--warning);" />
          <h3 class="pc-dlg-section-title">认证设置</h3>
        </div>

        <div class="pc-switch-row">
          <div>
            <div class="pc-switch-title">启用账号认证</div>
            <div class="pc-switch-desc">关闭后将允许免认证连接</div>
          </div>
          <el-switch v-model="form.auth_enabled" />
        </div>

        <div v-if="form.auth_enabled" class="grid grid-cols-2 gap-4">
          <div class="space-y-1">
            <label class="pc-form-label">用户名</label>
            <el-input v-model="form.username" placeholder="例如 user01" />
          </div>
          <div class="space-y-1">
            <label class="pc-form-label">密码</label>
            <el-input v-model="form.password" type="password" show-password placeholder="请输入密码" />
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex items-center justify-end gap-2">
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSave">保存</el-button>
      </div>
    </template>
  </el-dialog>
</template>

<style scoped>
.pc-dlg-section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border);
}
.pc-dlg-section-bar {
  width: 4px;
  height: 16px;
  border-radius: 999px;
}
.pc-dlg-section-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
}
.pc-form-label {
  font-size: 12px;
  font-weight: 700;
  color: var(--muted-foreground);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
.pc-switch-row {
  background: var(--muted);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.pc-switch-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
}
.pc-switch-desc {
  font-size: 12px;
  color: var(--muted-foreground);
  margin-top: 2px;
}
</style>
