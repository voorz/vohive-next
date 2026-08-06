<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Earth24Regular,
  Router24Regular
} from '@vicons/fluent'
import ProxyConfigUpstreamPanel from '../components/ProxyConfigUpstreamPanel.vue'
import ProxyConfigOutboundPanel from '../components/ProxyConfigOutboundPanel.vue'
import ProxyConfigEditDialog from '../components/ProxyConfigEditDialog.vue'
import ProxyConfigOutboundEditDialog from '../components/ProxyConfigOutboundEditDialog.vue'
import ProxyConfigBatchImportDialog from '../components/ProxyConfigBatchImportDialog.vue'
import { usePollingScheduler } from '../composables/usePollingScheduler'
import { useUpstreamProxyStore } from '../stores/upstream-proxy'
import { useProxyStore } from '../stores/proxy'
import { toAppError } from '../services/http'
import { parseProxyUrl, type ParsedProxy } from '../utils/proxyUrlParser'
import { upstreamProxyAddressWarning } from '../utils/upstreamProxyAddress'
import type { UpstreamProxyWithMeta, OutboundInstanceWithStatus, UpstreamProxyFormData } from '../types/proxy-config'
import type { UpstreamProxy, ProxyInstance, UpstreamProxyLookupResult } from '../types/api'

// ── Tab 控制 ──
const activeTab = ref('upstream')

// ══════════════════════════════════════════════════════
// Store
// ══════════════════════════════════════════════════════
const upstreamStore = useUpstreamProxyStore()
const proxyStore = useProxyStore()
const { statusMap } = storeToRefs(proxyStore)

// ── 前置代理状态 ──
const upstreamLoading = ref(true)
const upstreamError = ref<{ message: string; status?: number } | null>(null)
const lookupLoading = ref<string[]>([])
const lookupResults = ref<Map<string, UpstreamProxyLookupResult & { country_code?: string }>>(new Map())

// ── 出站代理状态 ──
const outboundLoading = ref(true)
const outboundError = ref<{ message: string; status?: number } | null>(null)

// ── 对话框状态 ──
const editDialogVisible = ref(false)
const editingProxy = ref<UpstreamProxyWithMeta | null>(null)
const outboundDialogVisible = ref(false)
const editingOutbound = ref<OutboundInstanceWithStatus | null>(null)
const batchDialogVisible = ref(false)

// ══════════════════════════════════════════════════════
// 前置代理：计算属性
// ══════════════════════════════════════════════════════

const upstreamProxies = computed<UpstreamProxyWithMeta[]>(() => {
  return upstreamStore.proxies.map(p => {
    // 优先使用内存中的最新查询结果，其次使用 DB 持久化的 lookup 数据
    const fresh = lookupResults.value.get(p.id)
    let lookup = fresh || null
    if (!lookup && p.lookup_at) {
      // 从持久化字段构建 lookup 对象
      const rule = upstreamStore.countryRules.find(r =>
        r.country_name === p.lookup_country || r.country_code === p.lookup_country
      )
      lookup = {
        status: 'ok',
        ip: p.lookup_ip || '',
        country: p.lookup_country || '',
        region: p.lookup_region || '',
        city: p.lookup_city || '',
        asn: p.lookup_asn || '',
        organization: p.lookup_organization || '',
        latency_ms: p.lookup_latency_ms || 0,
        error: p.lookup_error || '',
        country_code: rule?.country_code
      }
    }
    return {
      id: p.id,
      name: p.name,
      addr: p.addr,
      username: p.username,
      password: p.password || '',
      enabled: p.enabled,
      ruleCount: upstreamStore.getRulesForProxy(p.id).length,
      lookup,
      _testing: lookupLoading.value.includes(p.id)
    }
  })
})

// ══════════════════════════════════════════════════════
// 出站代理：计算属性
// ══════════════════════════════════════════════════════

const outboundInstances = computed<OutboundInstanceWithStatus[]>(() => {
  return proxyStore.instances.map(inst => {
    const status = statusMap.value[inst.id] || { id: inst.id, running: false }
    return {
      id: inst.id,
      name: inst.name,
      device_id: inst.device_id,
      enabled: inst.enabled,
      mode: inst.mode || 'socks5',
      listen_addr: inst.listen_addr,
      listen_port: inst.listen_port,
      auth_enabled: inst.auth_enabled,
      username: inst.username,
      password: inst.password || '',
      running: status.running,
      last_error: status.last_error || ''
    }
  })
})

// ══════════════════════════════════════════════════════
// 前置代理：数据加载
// ══════════════════════════════════════════════════════

async function fetchUpstream(opts: { silent?: boolean; initial?: boolean } = {}) {
  const isInitial = opts.initial === true
  const silent = opts.silent === true
  if (isInitial) {
    upstreamLoading.value = true
  }
  upstreamError.value = null

  try {
    const result = await upstreamStore.fetchAll()
    if (!result.ok) throw new Error(result.error.message)
  } catch (e: unknown) {
    const err = toAppError(e)
    upstreamError.value = {
      message: err.message || '加载前置代理失败',
      status: err.status
    }
  } finally {
    if (isInitial) {
      upstreamLoading.value = false
    }
  }
}

// ══════════════════════════════════════════════════════
// 出站代理：数据加载
// ══════════════════════════════════════════════════════

async function fetchOutbound(opts: { silent?: boolean; initial?: boolean } = {}) {
  const isInitial = opts.initial === true
  if (isInitial) {
    outboundLoading.value = true
  }
  outboundError.value = null

  try {
    const result = await proxyStore.fetchOverview()
    if (!result.ok) throw new Error(result.error.message)
  } catch (e: unknown) {
    const err = toAppError(e)
    outboundError.value = {
      message: err.message || '加载代理配置失败',
      status: err.status
    }
  } finally {
    if (isInitial) {
      outboundLoading.value = false
    }
  }
}

// ══════════════════════════════════════════════════════
// 前置代理：操作
// ══════════════════════════════════════════════════════

function handleAddUpstream() {
  editingProxy.value = null
  editDialogVisible.value = true
}

function handleEditUpstream(proxy: UpstreamProxyWithMeta) {
  editingProxy.value = proxy
  editDialogVisible.value = true
}

async function handleDeleteUpstream(proxy: UpstreamProxyWithMeta) {
  const confirmed = await ElMessageBox.confirm(
    `确定删除前置代理「${proxy.name || proxy.id}」？\n绑定到该代理的国家规则将自动删除，相关国家会恢复直连。`,
    '确认删除',
    { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
  ).then(() => true).catch(() => false)

  if (!confirmed) return

  try {
    const result = await upstreamStore.deleteProxy(proxy.id)
    if (!result.ok) throw new Error(result.error.message || '删除失败')
    ElMessage.success('前置代理已删除')
    await fetchUpstream({ silent: true })
  } catch (e: unknown) {
    const err = toAppError(e)
    ElMessage.error(err.message || '删除失败')
  }
}

async function handleToggleUpstream(proxy: UpstreamProxyWithMeta) {
  try {
    const result = await upstreamStore.updateProxy(proxy.id, { enabled: !proxy.enabled })
    if (!result.ok) throw new Error(result.error.message || '更新失败')
    await fetchUpstream({ silent: true })
  } catch (e: unknown) {
    const err = toAppError(e)
    ElMessage.error(err.message || '更新失败')
  }
}

async function doProxyLookup(id: string, silent = false): Promise<void> {
  lookupLoading.value = [...lookupLoading.value, id]
  try {
    const result = await upstreamStore.lookupProxy(id)
    if (result.ok) {
      const lookupData = result.data
      const rule = upstreamStore.countryRules.find(r =>
        r.country_name === lookupData.country || r.country_code === lookupData.country
      )
      lookupResults.value.set(id, {
        ...lookupData,
        country_code: rule?.country_code
      })
    } else if (!silent) {
      ElMessage.error(result.error?.message || '查询失败')
    }
  } catch (e: unknown) {
    if (!silent) {
      const err = toAppError(e)
      ElMessage.error(err.message || '查询失败')
    }
  } finally {
    lookupLoading.value = lookupLoading.value.filter(x => x !== id)
  }
}

async function handleTestLatency(proxy: UpstreamProxyWithMeta) {
  await doProxyLookup(proxy.id)
}

async function handleSaveUpstream(formData: UpstreamProxyFormData, pendingCountryCodes: string[] = []) {
  const form: UpstreamProxy = {
    id: editingProxy.value ? editingProxy.value.id : `proxy-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
    name: formData.name.trim() || `前置代理_${String(upstreamStore.proxies.length + 1).padStart(2, '0')}`,
    addr: formData.addr.trim(),
    username: formData.username.trim(),
    password: formData.password,
    enabled: formData.enabled
  }

  if (!form.addr) {
    ElMessage.warning('地址不能为空')
    return
  }

  // 如果输入了链接串，解析出 host:port
  const parsed = parseProxyUrl(form.addr)
  if (parsed.valid) {
    form.addr = parsed.host.includes(':') ? `[${parsed.host}]:${parsed.port}` : `${parsed.host}:${parsed.port}`
    if (parsed.username) form.username = parsed.username
    if (parsed.password) form.password = parsed.password
  }

  const addrWarning = upstreamProxyAddressWarning(form.addr)
  if (addrWarning) {
    ElMessage.warning(addrWarning)
    return
  }

  try {
    if (editingProxy.value) {
      // 更新：密码留空则不传
      if (!form.password) delete form.password
      const result = await upstreamStore.updateProxy(editingProxy.value.id, form)
      if (!result.ok) throw new Error(result.error.message || '更新失败')
      ElMessage.success('前置代理已更新')
    } else {
      const result = await upstreamStore.createProxy(form)
      if (!result.ok) throw new Error(result.error.message || '创建失败')
      // 新建成功后批量创建待提交的国家规则
      if (pendingCountryCodes.length > 0) {
        const ruleResults = await Promise.all(
          pendingCountryCodes.map(code =>
            upstreamStore.upsertCountryRule(code, {
              upstream_proxy_id: form.id,
              enabled: true
            })
          )
        )
        const failed = ruleResults.filter(r => !r.ok).length
        if (failed > 0) {
          ElMessage.warning(`代理已创建，${failed} 条国家规则创建失败`)
        } else {
          ElMessage.success(`前置代理已创建，并绑定 ${pendingCountryCodes.length} 个国家规则`)
        }
      } else {
        ElMessage.success('前置代理已创建')
      }
    }
    editDialogVisible.value = false
    await fetchUpstream({ silent: true })
    // 新建代理后自动查询节点信息
    if (!editingProxy.value) {
      doProxyLookup(form.id, true)
    }
  } catch (e: unknown) {
    const err = toAppError(e)
    ElMessage.error(err.message || '保存失败')
  }
}

async function handleAddRule(countryCode: string) {
  if (!editingProxy.value) return
  try {
    const result = await upstreamStore.upsertCountryRule(countryCode, {
      upstream_proxy_id: editingProxy.value.id,
      enabled: true
    })
    if (!result.ok) throw new Error(result.error.message || '保存规则失败')
    ElMessage.success('国家规则已保存')
    await fetchUpstream({ silent: true })
  } catch (e: unknown) {
    const err = toAppError(e)
    ElMessage.error(err.message || '保存规则失败')
  }
}

async function handleDeleteRule(countryCode: string) {
  try {
    const result = await upstreamStore.deleteCountryRule(countryCode)
    if (!result.ok) throw new Error(result.error.message || '删除规则失败')
    ElMessage.success('国家规则已删除，该国家将默认直连')
    await fetchUpstream({ silent: true })
  } catch (e: unknown) {
    const err = toAppError(e)
    ElMessage.error(err.message || '删除规则失败')
  }
}

// ══════════════════════════════════════════════════════
// 批量导入
// ══════════════════════════════════════════════════════

async function handleBatchImport(proxies: ParsedProxy[]) {
  let success = 0
  let failed = 0
  let nameIdx = upstreamStore.proxies.length + 1
  const createdIds: string[] = []
  for (const p of proxies) {
    try {
      const id = `proxy-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`
      const addr = p.host.includes(':') ? `[${p.host}]:${p.port}` : `${p.host}:${p.port}`
      const result = await upstreamStore.createProxy({
        id,
        name: `前置代理_${String(nameIdx).padStart(2, '0')}`,
        addr,
        username: p.username || '',
        password: p.password || '',
        enabled: true
      })
      if (result.ok) {
        success++
        nameIdx++
        createdIds.push(id)
      } else {
        failed++
      }
    } catch {
      failed++
    }
  }
  if (success > 0) {
    ElMessage.success(`成功导入 ${success} 条${failed > 0 ? `，失败 ${failed} 条` : ''}`)
    await fetchUpstream({ silent: true })
    // 自动查询每个新导入节点的信息（静默，不报错弹窗）
    for (const id of createdIds) {
      doProxyLookup(id, true)
    }
  } else {
    ElMessage.error('导入失败')
  }
}

// ══════════════════════════════════════════════════════
// 出站代理：操作
// ══════════════════════════════════════════════════════

function handleAddOutbound() {
  editingOutbound.value = null
  outboundDialogVisible.value = true
}

function handleEditOutbound(inst: OutboundInstanceWithStatus) {
  editingOutbound.value = inst
  outboundDialogVisible.value = true
}

async function handleDeleteOutbound(id: string) {
  const confirmed = await ElMessageBox.confirm(
    `确定删除实例 ${id}？`,
    '确认删除',
    { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
  ).then(() => true).catch(() => false)

  if (!confirmed) return

  const next = proxyStore.instances.filter(i => i.id !== id)
  try {
    const result = await proxyStore.saveConfig(next)
    if (!result.ok) throw new Error(result.error.message || '删除失败')
    ElMessage.success('实例已删除')
    await fetchOutbound({ silent: true })
  } catch (e: unknown) {
    const err = toAppError(e)
    ElMessage.error(err.message || '删除失败')
  }
}

async function handleStart(id: string) {
  try {
    const result = await proxyStore.startInstance(id)
    if (!result.ok) throw new Error(result.error.message || '启动失败')
    ElMessage.success('已启动')
    await fetchOutbound({ silent: true })
  } catch (e: unknown) {
    const err = toAppError(e)
    ElMessage.error(err.message || '启动失败')
  }
}

async function handleStop(id: string) {
  try {
    const result = await proxyStore.stopInstance(id)
    if (!result.ok) throw new Error(result.error.message || '停止失败')
    ElMessage.success('已停止')
    await fetchOutbound({ silent: true })
  } catch (e: unknown) {
    const err = toAppError(e)
    ElMessage.error(err.message || '停止失败')
  }
}

async function handleRestart(id: string) {
  try {
    const result = await proxyStore.restartInstance(id)
    if (!result.ok) throw new Error(result.error.message || '重启失败')
    ElMessage.success('已重启')
    await fetchOutbound({ silent: true })
  } catch (e: unknown) {
    const err = toAppError(e)
    ElMessage.error(err.message || '重启失败')
  }
}

async function handleSaveOutbound(form: OutboundInstanceWithStatus) {
  const inst: ProxyInstance = {
    id: form.id,
    name: form.name,
    device_id: form.device_id,
    enabled: form.enabled,
    mode: form.mode,
    listen_addr: form.listen_addr || '0.0.0.0',
    listen_port: form.listen_port,
    auth_enabled: form.auth_enabled,
    username: form.auth_enabled ? form.username : '',
    password: form.auth_enabled ? form.password : ''
  }

  if (editingOutbound.value) {
    const idx = proxyStore.instances.findIndex(i => i.id === editingOutbound.value!.id)
    if (idx >= 0) {
      proxyStore.instances[idx] = inst
    }
  } else {
    if (proxyStore.instances.some(i => i.id === inst.id)) {
      ElMessage.warning('实例 ID 已存在')
      return
    }
    proxyStore.instances.push(inst)
  }

  try {
    const result = await proxyStore.saveConfig(proxyStore.instances)
    if (!result.ok) throw new Error(result.error.message || '保存失败')
    ElMessage.success(editingOutbound.value ? '实例已保存' : '实例已创建')
    outboundDialogVisible.value = false
    await fetchOutbound({ silent: true })
  } catch (e: unknown) {
    const err = toAppError(e)
    ElMessage.error(err.message || '保存失败')
  }
}

// ══════════════════════════════════════════════════════
// 轮询
// ══════════════════════════════════════════════════════

const upPollEnabled = computed(() => !upstreamLoading.value && activeTab.value === 'upstream')
usePollingScheduler(() => fetchUpstream({ silent: true }), 10000, {
  enabled: upPollEnabled,
  maxIntervalMs: 60000,
  backgroundIntervalMs: 30000
})

const outPollEnabled = computed(() => !outboundLoading.value && proxyStore.instances.length > 0)
usePollingScheduler(() => fetchOutbound({ silent: true }), 5000, {
  enabled: outPollEnabled,
  maxIntervalMs: 60000,
  backgroundIntervalMs: 15000
})

// ══════════════════════════════════════════════════════
// 初始化
// ══════════════════════════════════════════════════════

onMounted(() => {
  fetchUpstream({ initial: true })
  fetchOutbound({ initial: true })
})
</script>

<template>
  <div class="proxy-page h-[calc(100svh-56px-48px)] flex flex-col">
    <div class="proxy-card flex-1 min-h-0 flex flex-col">

      <!-- Tab 切换 -->
      <el-tabs v-model="activeTab" class="proxy-tabs">
        <el-tab-pane name="upstream">
          <template #label>
            <div class="flex items-center gap-1.5">
              <el-icon size="16"><Earth24Regular /></el-icon>
              <span class="font-medium">漫游前置代理</span>
              <span
                v-if="upstreamStore.proxies.length > 0"
                class="inline-flex items-center justify-center px-1.5 py-0.5 text-[10px] font-bold leading-none text-white rounded-full shadow-sm ml-0.5"
                style="background: var(--brand);"
              >
                {{ upstreamStore.proxies.length }}
              </span>
            </div>
          </template>
        </el-tab-pane>
        <el-tab-pane name="outbound">
          <template #label>
            <div class="flex items-center gap-1.5">
              <el-icon size="16"><Router24Regular /></el-icon>
              <span class="font-medium">本地出站代理</span>
              <span
                v-if="outboundInstances.length > 0"
                class="inline-flex items-center justify-center px-1.5 py-0.5 text-[10px] font-bold leading-none text-white rounded-full shadow-sm ml-0.5"
                style="background: var(--brand);"
              >
                {{ outboundInstances.length }}
              </span>
            </div>
          </template>
        </el-tab-pane>
      </el-tabs>

      <!-- ═══ 前置代理 Tab ═══ -->
      <div v-show="activeTab === 'upstream'" class="flex-1 min-h-0 overflow-auto">
        <ProxyConfigUpstreamPanel
          :proxies="upstreamProxies"
          :loading="upstreamLoading"
          :error="upstreamError"
          @batch-import="batchDialogVisible = true"
          @edit="handleEditUpstream"
          @delete="handleDeleteUpstream"
          @toggle="handleToggleUpstream"
          @test-latency="handleTestLatency"
        />
      </div>

      <!-- ═══ 出站代理 Tab ═══ -->
      <div v-show="activeTab === 'outbound'" class="flex-1 min-h-0 overflow-auto">
        <ProxyConfigOutboundPanel
          :instances="outboundInstances"
          :devices="proxyStore.devices"
          :loading="outboundLoading"
          :error="outboundError"
          @add="handleAddOutbound"
          @edit="handleEditOutbound"
          @delete="handleDeleteOutbound"
          @start="handleStart"
          @stop="handleStop"
          @restart="handleRestart"
        />
      </div>

    </div>

    <!-- ═══ 对话框 ═══ -->
    <ProxyConfigEditDialog
      v-model:visible="editDialogVisible"
      :editing="editingProxy"
      :countries="upstreamStore.countries"
      :existing-rules="upstreamStore.countryRules"
      @save="handleSaveUpstream"
      @add-rule="handleAddRule"
      @delete-rule="handleDeleteRule"
    />

    <ProxyConfigOutboundEditDialog
      v-model:visible="outboundDialogVisible"
      :editing="editingOutbound"
      :devices="proxyStore.devices"
      @save="handleSaveOutbound"
    />

    <ProxyConfigBatchImportDialog
      v-model:visible="batchDialogVisible"
      @import="handleBatchImport"
    />
  </div>
</template>

<style scoped>
.proxy-tabs {
  flex-shrink: 0;
}
.proxy-tabs :deep(.el-tabs__header) {
  margin-bottom: 0;
  height: 60px;
  padding: 0 16px;
}
.proxy-tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
}
.proxy-tabs :deep(.el-tabs__content) {
  display: none;
}
.proxy-card {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--card);
  box-shadow: var(--console-shadow-sm);
  overflow: hidden;
}
</style>
