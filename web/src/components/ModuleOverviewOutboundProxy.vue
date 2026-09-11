<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import type { DeviceOverviewItem } from '../types/api'
import { devicesService } from '../services/devices'
import { copyToClipboard } from '../utils/clipboard'
import { loadPlmnInfo } from '../composables/plmn-info'
import { ElMessage } from 'element-plus'
import { Globe24Regular, Link24Regular } from '@vicons/fluent'
import CountryFlag from './CountryFlag.vue'
import { useUpstreamProxyStore } from '../stores/upstream-proxy'
import type { UpstreamProxyCountryRule } from '../types/api'

type OutboundProxyStatus = {
  enabled: boolean
  op_ready: boolean
  iccid?: string
  exposed_as_upstream?: boolean
  ip_country_code?: string  // IP 查询返回的实际出口 ISO
  public_ip?: string  // 公网 IP
  latency_ms?: number  // 延迟（毫秒）
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

// ---- 前置代理 store（国家规则复用）----
const upstreamStore = useUpstreamProxyStore()

// ---- 状态 ----
const status = ref<OutboundProxyStatus | null>(null)
const toggling = ref(false)
const exposing = ref(false)
const selectedCountry = ref('')
const ruleLoading = ref(false)
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

// 实际出口 IP 归属地 ISO（仅用 IP 查询结果）
const actualIso = computed(() => {
  return status.value?.ip_country_code || ''
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
      const result = await devicesService.exposeOutboundProxyAsUpstream(id)
      if (!result.ok) throw new Error(result.error?.message || '暴露失败')
      ElMessage.success('已暴露为前置代理')
    } else {
      const result = await devicesService.unexposeOutboundProxyAsUpstream(id)
      if (!result.ok) throw new Error(result.error?.message || '取消暴露失败')
      ElMessage.success('已取消暴露前置代理')
    }
    await fetchStatus()
    // 刷新前置代理 store（国家规则列表依赖此数据）
    await upstreamStore.fetchAll()
    // 暴露成功后延迟 3 秒再拉取一次，等待后端异步延迟测试写入 DB
    if (targetExposed) {
      setTimeout(() => {
        fetchStatus()
        upstreamStore.fetchAll()
      }, 3000)
    }
    return true
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : '操作失败')
    return false
  } finally {
    exposing.value = false
  }
}

// ---- 国家路由规则 ----

// 当前出站代理暴露的前置代理的国家规则
const currentRules = computed<UpstreamProxyCountryRule[]>(() => {
  if (!status.value?.exposed_as_upstream || !status.value?.iccid) return []
  // 通过 ICCID 查找对应的 Auto 来源前置代理，再获取其国家规则
  const proxy = upstreamStore.proxies.find(p => p.source === 'Auto' && p.identity_id === status.value?.iccid)
  if (!proxy) return []
  return upstreamStore.getRulesForProxy(proxy.id)
})

// 国家规则数量
const ruleCount = computed(() => currentRules.value.length)

// 可选国家（排除已配置到任何代理的，包括当前代理——已在列表中）
const availableCountries = computed(() => {
  return upstreamStore.countries.filter(country => {
    const rule = upstreamStore.countryRules.find(r => r.country_code === country.country_code)
    // 没有规则 → 可选
    if (!rule) return true
    // 有规则 → 不可选（已配置到某个代理）
    return false
  })
})

function formatCountryLabel(country: { country_code: string; country_name: string; mccs: string[] }) {
  const mccs = country.mccs?.length ? ` · MCC ${country.mccs.join('/')}` : ''
  return `${country.country_code} · ${country.country_name || country.country_code}${mccs}`
}

async function handleAddRule() {
  if (!selectedCountry.value || !status.value?.iccid) return
  const proxy = upstreamStore.proxies.find(p => p.source === 'Auto' && p.identity_id === status.value?.iccid)
  if (!proxy) {
    ElMessage.warning('前置代理未找到')
    return
  }
  ruleLoading.value = true
  try {
    const result = await upstreamStore.upsertCountryRule(selectedCountry.value, {
      upstream_proxy_id: proxy.id,
      enabled: true,
    })
    if (!result.ok) throw new Error(result.error?.message || '添加规则失败')
    ElMessage.success('国家规则已添加')
    selectedCountry.value = ''
    await upstreamStore.fetchAll()
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : '操作失败')
  } finally {
    ruleLoading.value = false
  }
}

async function handleDeleteRule(countryCode: string) {
  ruleLoading.value = true
  try {
    const result = await upstreamStore.deleteCountryRule(countryCode)
    if (!result.ok) throw new Error(result.error?.message || '删除规则失败')
    ElMessage.success('国家规则已删除')
    await upstreamStore.fetchAll()
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : '操作失败')
  } finally {
    ruleLoading.value = false
  }
}

// ---- 生命周期 ----

onMounted(() => {
  loadPlmnInfo()
  // 加载前置代理数据（国家列表 + 规则）
  upstreamStore.fetchAll()
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

// 公网 变化时（如切换 IP 后）重新获取状态
watch(
  () => props.device?.public_ip,
  (newIP, oldIP) => {
    if (newIP !== oldIP && newIP && props.device?.id) {
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
      <!-- Part 1: 使用流量创建出站代理开关 -->
      <div class="field col-span-2 form-switch-row">
        <div>
          <div class="switch-title">使用流量创建出站代理</div>
          <div class="switch-desc">通过当前流量在模组建立物理网络通道给代理节点使用</div>
        </div>
        <el-switch
          :model-value="proxyEnabled"
          :loading="toggling"
          :before-change="onProxyBeforeChange"
        />
      </div>

      <!-- Part 2: 实例表格 -->
        <div v-if="instances.length > 0" class="op-conn-table">
          <el-table :data="instances" :border="false">
<el-table-column prop="id" label="实例 ID" align="center" show-overflow-tooltip>
<template #default="{ row }">
<span class="op-cell-mono">{{ row.id }}</span>
</template>
</el-table-column>
<el-table-column prop="running" label="运行状态" align="center">
<template #default="{ row }">
<span :class="row.running ? 'op-running' : 'op-stopped'">
{{ row.running ? '运行中' : '已停止' }}
</span>
</template>
</el-table-column>
            <el-table-column prop="active_conns" label="活跃连接数" align="center">
              <template #default="{ row }">
                <el-tag size="small" :type="row.active_conns > 0 ? 'success' : 'info'" effect="light" round>
                  {{ row.active_conns }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <!-- Part 3: 国家路由规则 -->
        <div v-if="exposedAsUpstream" class="op-rules-section">
          <div class="op-section-label">国家路由规则 <span class="op-section-hint">(VoWiFi 根据PLMN自动命中)</span></div>
          <!-- 已配置规则列表 -->
          <div v-if="currentRules.length > 0" class="op-rules-list">
            <div
              v-for="rule in currentRules"
              :key="rule.country_code"
              class="op-rule-item"
            >
              <div class="op-rule-info">
                <CountryFlag :iso="rule.country_code" :size="18" />
                <span class="op-rule-code">{{ rule.country_code }}</span>
                <span class="op-rule-name">· {{ rule.country_name || rule.country_code }}</span>
                <span class="op-rule-mcc">MCC {{ rule.mccs?.join('/') || '-' }}</span>
              </div>
              <el-button size="small" type="danger" text :loading="ruleLoading" @click="handleDeleteRule(rule.country_code)">
                删除
              </el-button>
            </div>
          </div>
          <div v-else class="op-rules-empty">暂无国家规则，未配置的国家默认直连</div>
          <!-- 添加规则 -->
          <div class="op-rule-add">
            <el-select
              v-model="selectedCountry"
              placeholder="选择国家"
              class="flex-1"
              filterable
              :disabled="ruleLoading"
            >
              <el-option
                v-for="country in availableCountries"
                :key="country.country_code"
                :label="formatCountryLabel(country)"
                :value="country.country_code"
              >
                <div class="op-country-option">
                  <CountryFlag :iso="country.country_code" :size="16" />
                  <span>{{ country.country_code }} · {{ country.country_name || country.country_code }}</span>
                  <span class="op-country-mcc">MCC {{ country.mccs.join('/') }}</span>
                </div>
              </el-option>
            </el-select>
            <el-button
              type="primary"
              :disabled="!selectedCountry || ruleLoading"
              :loading="ruleLoading"
              @click="handleAddRule"
            >
              <el-icon class="mr-1"><Link24Regular /></el-icon>
              <span>添加</span>
            </el-button>
          </div>
        </div>

        <!-- Part 4: 信息统计 -->
        <div v-if="exposedAsUpstream && instances.length > 0" class="op-stats-row">
          <div class="op-stat-item">
            <span class="op-stat-label">节点信息</span>
            <span class="op-stat-value">Socks5</span>
          </div>
          <div class="op-stat-item">
            <span class="op-stat-label">IP 归属地</span>
            <span class="op-stat-value">
              <CountryFlag v-if="actualIso" :iso="actualIso" :size="14" />
              {{ actualIso || '--' }}
            </span>
          </div>
          <div class="op-stat-item">
            <span class="op-stat-label">公网</span>
            <span class="op-stat-value font-mono">{{ status?.public_ip || '--' }}</span>
          </div>
          <div class="op-stat-item">
            <span class="op-stat-label">国家规则</span>
            <span class="op-stat-value">{{ ruleCount }}个</span>
          </div>
        </div>

        <!-- Part 5: 前置代理节点卡片 -->
        <div v-if="exposedAsUpstream && instances.length > 0" class="op-node-card">
          <div
            v-for="inst in instances"
            :key="inst.id"
            class="op-node-card-row"
          >
            <div class="op-node-card-item">
              <span class="op-node-card-label">监听地址</span>
              <span class="op-node-card-value font-mono">0.0.0.0:{{ inst.listen_port }}</span>
            </div>
            <div class="op-node-card-item">
              <span class="op-node-card-label">延迟</span>
              <span class="op-node-card-value" :class="{ 'op-latency-ok': status?.latency_ms && status.latency_ms > 0 }">
                {{ status?.latency_ms && status.latency_ms > 0 ? status.latency_ms + 'ms' : '--' }}
              </span>
            </div>
            <div class="op-node-card-action">
              <el-button size="small" @click="copyVal(`socks5://vohive:vohive@0.0.0.0:${inst.listen_port}`)">
                复制节点
              </el-button>
            </div>
          </div>
        </div>

        <!-- Part 6: 激活代理节点 switch -->
        <div v-if="proxyEnabled" class="field col-span-2 form-switch-row" :class="{ 'is-unsupported': !opReady }">
          <div>
            <div class="switch-title">激活代理节点</div>
            <div class="switch-desc">在通道中建立socks5前置代理节点并暴露到局域网</div>
          </div>
          <el-switch
            :model-value="exposedAsUpstream"
            :loading="exposing"
            :disabled="!opReady"
            :before-change="onExposeBeforeChange"
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

/* 开关行（参照 ModuleCardPolicy 风格） */
.field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
  margin-bottom: 12px;
}
.field.col-span-2 {
  grid-column: 1 / -1;
}
.form-switch-row {
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
.form-switch-row.is-unsupported .switch-title,
.form-switch-row.is-unsupported .switch-desc {
  opacity: 0.4;
}

/* Part 4: 信息统计 */
.op-stats-row {
  display: flex;
  gap: 20px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--muted);
  margin-bottom: 12px;
}
.op-stat-item {
display: flex;
flex-direction: column;
align-items: center;
gap: 2px;
flex: 1;
min-width: 0;
}
.op-stat-label {
  font-size: 11px;
  color: var(--muted-foreground);
}
.op-stat-value {
  font-size: 13px;
  color: var(--foreground);
  font-family: var(--oomol-font-mono);
  display: flex;
  align-items: center;
  gap: 4px;
}

/* 连接数表格 */
.op-conn-table {
  margin-bottom: 12px;
}
.op-conn-table :deep(.el-table) {
  font-size: 13px;
}
.op-conn-table :deep(.el-table th .cell) {
  font-size: 11px;
  color: var(--muted-foreground);
  font-weight: 400;
}
.op-conn-table :deep(.el-table .cell) {
  padding: 0 8px;
}
.op-cell-mono {
  font-family: var(--oomol-font-mono);
}
.op-running {
  color: var(--brand);
}
.op-stopped {
  color: var(--muted-foreground);
}

/* Part 3: 国家路由规则 */
.op-rules-section {
  margin-bottom: 12px;
}
.op-section-label {
font-size: 11px;
color: var(--muted-foreground);
margin-bottom: 8px;
}
.op-section-hint {
font-size: 10px;
opacity: 0.7;
}
.op-rules-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 8px;
}
.op-rule-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: var(--muted);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 8px 12px;
}
.op-rule-info {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.op-rule-code {
  font-size: 13px;
  color: var(--foreground);
  font-family: var(--oomol-font-mono);
}
.op-rule-name {
  font-size: 11px;
  color: var(--muted-foreground);
}
.op-rule-mcc {
  font-size: 11px;
  font-family: var(--oomol-font-mono);
  color: var(--muted-foreground);
  margin-left: 4px;
}
.op-rules-empty {
  text-align: center;
  padding: 10px;
  color: var(--muted-foreground);
  font-size: 11px;
  border: 1px dashed var(--border);
  border-radius: 6px;
  margin-bottom: 8px;
}
.op-rule-add {
  display: flex;
  align-items: center;
  gap: 8px;
}
.op-country-option {
  display: flex;
  align-items: center;
  gap: 6px;
}
.op-country-mcc {
  font-size: 11px;
  font-family: var(--oomol-font-mono);
  color: var(--muted-foreground);
  margin-left: auto;
}

/* Part 5: 前置代理节点卡片 */
.op-node-card {
  margin-bottom: 12px;
}
.op-node-card-row {
  display: flex;
  align-items: center;
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 8px 12px;
}
.op-node-card-item {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 6px;
}
.op-node-card-item:nth-child(2) {
  justify-content: center;
}
.op-node-card-action {
  flex: 1;
  display: flex;
  justify-content: flex-end;
}
.op-node-card-label {
  font-size: 11px;
  color: var(--muted-foreground);
}
.op-node-card-value {
  font-size: 13px;
  color: var(--foreground);
  font-family: var(--oomol-font-mono);
}
.op-latency-ok {
  color: var(--brand);
}

</style>
