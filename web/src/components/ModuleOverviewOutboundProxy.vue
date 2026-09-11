<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import type { DeviceOverviewItem } from '../types/api'
import { devicesService } from '../services/devices'
import { copyToClipboard } from '../utils/clipboard'
import { loadPlmnInfo, mccToIso } from '../composables/plmn-info'
import { ElMessage } from 'element-plus'
import { Globe24Regular, Server24Regular, Copy24Regular } from '@vicons/fluent'

type OutboundProxyStatus = {
  enabled: boolean
  op_ready: boolean
  iccid?: string
  exposed_as_upstream?: boolean
  instances?: {
    id: string
    running: boolean
    listen_port: number
    active_conns: number
  }[]
}

const props = defineProps<{
  device: DeviceOverviewItem | null
}>()

const emit = defineEmits<{
  changed: []
}>()

// ---- 状态 ----
const status = ref<OutboundProxyStatus | null>(null)
const toggling = ref(false)
const exposing = ref(false)
// 防止 watch 误触发 API 的标记
const skipWatch = ref(false)

// ---- 计算属性 ----

const proxyEnabled = computed(() => {
  if (status.value) return status.value.enabled
  return !!props.device?.outbound_proxy_enabled
})

const opReady = computed(() => {
  if (status.value) return status.value.op_ready
  return !!props.device?.op_ready
})

const exposedAsUpstream = computed(() => !!status.value?.exposed_as_upstream)

const instances = computed(() => status.value?.instances || [])

const countryIso = computed(() => {
  const mcc = props.device?.modem?.native_mcc
  const mnc = props.device?.modem?.native_mnc
  if (!mcc) return ''
  return mccToIso(mcc, mnc)
})

// ---- 方法 ----

function copyVal(val: string | undefined) {
  if (!val || val === '--') return
  void copyToClipboard(val)
  ElMessage.success('已复制')
}

async function fetchStatus() {
  const id = props.device?.id
  if (!id) return
  skipWatch.value = true
  try {
    const result = await devicesService.getOutboundProxyStatus(id)
    if (result.ok) {
      status.value = result.data
    }
  } catch {
    // 静默失败
  } finally {
    skipWatch.value = false
  }
}

// 出站代理 Switch：使用 before-change 钩子确保只在用户点击时触发
async function onProxyBeforeChange(): Promise<boolean> {
  const id = props.device?.id
  if (!id) return false
  if (toggling.value) return false

  const targetEnabled = !proxyEnabled.value
  toggling.value = true
  try {
    if (targetEnabled) {
      const result = await devicesService.enableOutboundProxy(id)
      if (!result.ok) throw new Error(result.error?.message || '开启失败')
      ElMessage.success('出站代理已开启')
    } else {
      const result = await devicesService.disableOutboundProxy(id)
      if (!result.ok) throw new Error(result.error?.message || '关闭失败')
      ElMessage.success('出站代理已关闭')
      status.value = null
    }
    emit('changed')
    await fetchStatus()
    return true
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : '操作失败')
    return false
  } finally {
    toggling.value = false
  }
}

// 前置代理 Switch：使用 before-change 钩子确保只在用户点击时触发
async function onExposeBeforeChange(): Promise<boolean> {
  const id = props.device?.id
  if (!id) return false
  if (exposing.value) return false

  const targetExposed = !exposedAsUpstream.value
  exposing.value = true
  try {
    if (targetExposed) {
      const result = await devicesService.exposeOutboundProxyAsUpstream(id, countryIso.value)
      if (!result.ok) throw new Error(result.error?.message || '暴露失败')
      ElMessage.success('已暴露为前置代理')
    } else {
      const result = await devicesService.unexposeOutboundProxyAsUpstream(id)
      if (!result.ok) throw new Error(result.error?.message || '取消暴露失败')
      ElMessage.success('已取消暴露前置代理')
    }
    await fetchStatus()
    return true
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : '操作失败')
    return false
  } finally {
    exposing.value = false
  }
}

// ---- 生命周期 ----

onMounted(() => {
  loadPlmnInfo()
  if (props.device?.id) {
    fetchStatus()
  }
})

// 设备切换时重新获取状态
watch(
  () => props.device?.id,
  (newId, oldId) => {
    if (newId !== oldId && newId) {
      status.value = null
      fetchStatus()
    }
  }
)
</script>

<template>
  <div class="ov-card">
    <!-- 卡片头部：仅标题和图标 -->
    <div class="ov-card-head">
      <div class="ov-icon-box">
        <el-icon size="14"><Globe24Regular /></el-icon>
      </div>
      <span class="ov-card-title">出站代理</span>
    </div>
    <div class="ov-card-body">
      <!-- 出站代理开关（卡片体内） -->
      <div class="op-toggle-row">
        <span class="op-toggle-label">出站代理</span>
        <el-switch
          :model-value="proxyEnabled"
          :loading="toggling"
          :before-change="onProxyBeforeChange"
          size="small"
        />
      </div>

      <!-- 状态指示 -->
        <div class="op-status-row" :class="{ 'op-disabled': !proxyEnabled }">
          <div class="op-status-item">
            <span class="op-status-label">状态</span>
            <span class="op-status-value" :class="{ 'op-ready': opReady, 'op-pending': proxyEnabled && !opReady, 'op-off': !proxyEnabled }">
              {{ !proxyEnabled ? '未启用' : opReady ? '就绪' : '启动中...' }}
            </span>
          </div>
          <div class="op-status-item">
            <span class="op-status-label">归属地</span>
            <span class="op-status-value">{{ countryIso || '--' }}</span>
          </div>
        </div>

        <!-- 节点信息 -->
        <div v-if="instances.length > 0" class="op-node-info" :class="{ 'op-disabled': !proxyEnabled }">
          <div
            v-for="inst in instances"
            :key="inst.id"
            class="op-node-row"
          >
            <div class="op-node-field">
              <span class="op-node-label">监听地址</span>
              <span class="op-node-value copyable" @click="copyVal(`0.0.0.0:${inst.listen_port}`)">
                0.0.0.0:{{ inst.listen_port }}
              </span>
              <el-icon size="12" class="op-copy-icon"><Copy24Regular /></el-icon>
            </div>
            <div class="op-node-field">
              <span class="op-node-label">端口</span>
              <span class="op-node-value copyable" @click="copyVal(String(inst.listen_port))">
                {{ inst.listen_port }}
              </span>
            </div>
          </div>
        </div>

        <!-- 连接数表格 -->
        <div v-if="instances.length > 0" class="op-conn-table" :class="{ 'op-disabled': !proxyEnabled }">
          <el-table :data="instances" size="small" :border="false">
            <el-table-column prop="id" label="实例 ID" show-overflow-tooltip>
              <template #default="{ row }">
                <span class="op-cell-mono">{{ row.id }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="running" label="运行状态" width="100">
              <template #default="{ row }">
                <span :class="row.running ? 'op-running' : 'op-stopped'">
                  {{ row.running ? '运行中' : '已停止' }}
                </span>
              </template>
            </el-table-column>
            <el-table-column prop="active_conns" label="活跃连接数" width="120" align="right">
              <template #default="{ row }">
                <span class="op-cell-mono">{{ row.active_conns }}</span>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <!-- 暴露为前置代理（卡片内部区域） -->
        <div class="op-expose-row" :class="{ 'op-disabled': !proxyEnabled }">
          <div class="op-expose-label">
            <el-icon size="16"><Server24Regular /></el-icon>
            <span>暴露为前置代理</span>
          </div>
          <el-switch
            :model-value="exposedAsUpstream"
            :loading="exposing"
            :disabled="!proxyEnabled || !opReady"
            :before-change="onExposeBeforeChange"
            size="small"
          />
        </div>
    </div>
  </div>
</template>

<style scoped>
.ov-card {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--background);
  overflow: hidden;
}
.ov-card-head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
}
.ov-icon-box {
  width: 28px;
  height: 28px;
  border-radius: 5px;
  background: var(--muted);
  border: 1px solid var(--border);
  color: var(--foreground);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.ov-card-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
}
.ov-card-body {
  padding: 14px;
}

/* 出站代理开关行 */
.op-toggle-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  margin-bottom: 12px;
}
.op-toggle-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--foreground);
}

/* 未开启提示 */
.op-hint {
  font-size: 12px;
  color: var(--muted-foreground);
  text-align: center;
  padding: 8px 0;
}

/* 状态行 */
.op-status-row {
  display: flex;
  gap: 24px;
  margin-bottom: 12px;
}
.op-status-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.op-status-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-foreground);
}
.op-status-value {
  font-size: 13px;
  color: var(--foreground);
  font-weight: 500;
}
.op-status-value.op-ready {
  color: var(--brand);
}
.op-status-value.op-pending {
  color: var(--warning);
}
.op-status-value.op-off {
  color: var(--muted-foreground);
}

/* 节点信息 */
.op-node-info {
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 10px 12px;
  margin-bottom: 12px;
}
.op-node-row {
  display: flex;
  gap: 16px;
}
.op-node-field {
  display: flex;
  align-items: center;
  gap: 6px;
}
.op-node-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-foreground);
}
.op-node-value {
  font-size: 12px;
  color: var(--foreground);
  font-family: var(--oomol-font-mono);
}
.op-node-value.copyable {
  cursor: pointer;
}
.op-node-value.copyable:hover {
  color: var(--brand);
}
.op-copy-icon {
  color: var(--muted-foreground);
  flex-shrink: 0;
}

/* 连接数表格 */
.op-conn-table {
  margin-bottom: 12px;
}
.op-cell-mono {
  font-family: var(--oomol-font-mono);
  font-size: 12px;
}
.op-running {
  color: var(--brand);
  font-weight: 500;
}
.op-stopped {
  color: var(--muted-foreground);
}

/* 暴露为前置代理 */
.op-expose-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
}
.op-expose-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 500;
  color: var(--foreground);
}

/* 禁用状态 */
.op-disabled {
  opacity: 0.5;
  pointer-events: none;
}
</style>
