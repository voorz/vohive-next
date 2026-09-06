<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { ElMessage } from 'element-plus'
import { useSettingsStore } from '../stores/settings'
import FieldRow from '../components/FieldRow.vue'
import { 
  Key24Regular, 
  Save24Regular,
  Server24Regular,
  Alert24Regular,
  Add20Regular,
  Delete20Regular,
  Globe24Regular,
  ChevronDown20Regular,
  WindowConsole20Regular
} from '@vicons/fluent'

const settingsStore = useSettingsStore()
const { systemInfo, loadingNotifications, savingNotifications, testingWebhook, testingBark, testingEmail, testingTelegram, testingFeishu, testingPushplus, telegramForm, feishuForm, qqForm, webhookSettings, barkSettings, emailForm, pushplusForm } = storeToRefs(settingsStore)
const activeNotifyTab = ref('telegram')

const activeTab = ref('notify')

// ── 短信发送限制（全局）──
const smsLimitLoading = ref(false)
const smsLimitSaving = ref(false)
const smsHourlyLimit = ref(3)
const smsDailyLimit = ref(10)
const smsLimitExpanded = ref(true)

async function loadSMSRateLimit() {
  smsLimitLoading.value = true
  try {
    const res = await systemService.getSMSRateLimit()
    if (res.ok) {
      smsHourlyLimit.value = res.data.hourly_limit
      smsDailyLimit.value = res.data.daily_limit
    }
  } catch {
    // keep defaults
  } finally {
    smsLimitLoading.value = false
  }
}

async function saveSMSRateLimit() {
  smsLimitSaving.value = true
  try {
    const res = await systemService.saveSMSRateLimit(smsHourlyLimit.value, smsDailyLimit.value)
    if (!res.ok) throw new Error(res.error.message || '保存失败')
    ElMessage.success('短信限制已更新')
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    smsLimitSaving.value = false
  }
}



const hasValidWebhookURLs = computed(() => {
  if (!Array.isArray(webhookSettings.value.urls)) {
    return false
  }
  return webhookSettings.value.urls.some((u) => String(u || '').trim().length > 0)
})

const hasValidBarkURLs = computed(() => {
  if (!Array.isArray(barkSettings.value.urls)) {
    return false
  }
  return barkSettings.value.urls.some((u) => String(u || '').trim().length > 0)
})

const hasValidEmailConfig = computed(() => {
  return !!(
    emailForm.value.smtp_host &&
    emailForm.value.smtp_port &&
    emailForm.value.username &&
    emailForm.value.password &&
    emailForm.value.from_address &&
    emailForm.value.to_addresses
  )
})



async function loadSystemInfo() {
  const result = await settingsStore.fetchSystemInfo()
  if (!result.ok) {
    console.error('系统信息读取失败', result.error)
  }
}


async function loadNotifications() {
  try {
    const result = await settingsStore.fetchNotifications()
    if (!result.ok) throw new Error(result.error.message || '通知配置加载失败')
    syncWebhookHeaderRowsFromSettings()
  } catch {
    ElMessage.error('通知配置加载失败')
  }
}

function openAPIDocs() {
  const docsURL = String(systemInfo.value.docs?.docs_ui || '').trim()
  if (!docsURL) {
    ElMessage.warning('API 文档入口暂不可用')
    return
  }
  window.open(docsURL, '_blank', 'noopener,noreferrer')
}

async function saveNotifications() {
  try {
    const result = await settingsStore.saveNotificationsFromForms()
    if (!result.ok) throw new Error(result.error.message || '通知配置保存失败')
    const applied = result.data.applied
    const warning = result.data.warning
    if (applied === false && warning) {
      ElMessage.warning(warning)
    } else {
      ElMessage.success('通知配置已保存（已写入 config.yaml）')
    }
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : '通知配置保存失败')
  }
}

async function testWebhookNotification() {
  try {
    const result = await settingsStore.testWebhookFromForm()
    if (!result.ok) {
      throw new Error(result.error.message || 'Webhook 测试失败')
    }
    const data = result.data
    if (data.ok) {
      ElMessage.success(data.message || '测试通知已发送')
      return
    }
    if (Array.isArray(data.failed_urls) && data.failed_urls.length > 0) {
      ElMessage.error(`${data.message}\n失败 URL: ${data.failed_urls.join(', ')}`)
      return
    }
    ElMessage.error(data.message || 'Webhook 测试失败')
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : 'Webhook 测试失败')
  }
}

function addWebhookUrl() {
  if (!webhookSettings.value.urls) {
     webhookSettings.value.urls = []
  }
  webhookSettings.value.urls.push('')
}

function removeWebhookUrl(index: number) {
  webhookSettings.value.urls.splice(index, 1)
}

// 自定义请求头以「行」形式编辑（rows 为唯一编辑源），保存时单向回写为 map。
// 受保护的系统头由后端强制覆盖。
const PROTECTED_WEBHOOK_HEADERS = new Set(['content-type', 'x-vohive-signature'])
// 常用请求头预设，下拉可选；filterable + allow-create 也允许自行输入其它名称
const COMMON_WEBHOOK_HEADERS = [
  'Authorization',
  'X-Api-Key',
  'X-Auth-Token',
  'X-Webhook-Token',
  'X-Signature',
  'X-Request-Id',
  'Accept',
  'User-Agent'
]
// 每行带稳定 id，避免用数组下标作 v-for key 时，删除中间行后 el-select 复用实例残留选项
let webhookHeaderUid = 0
const webhookHeaderRows = ref<{ id: number; key: string; value: string }[]>([])

// 加载完成后调用，把已保存的 headers map 转换为可编辑的行
function syncWebhookHeaderRowsFromSettings() {
  const headers = webhookSettings.value.headers || {}
  webhookHeaderRows.value = Object.entries(headers).map(([key, value]) => ({
    id: webhookHeaderUid++,
    key,
    value: String(value ?? '')
  }))
}

// 行变化时单向回写为 map（丢弃空 key 与受保护头）。无反向 watch，故不会回环。
watch(
  webhookHeaderRows,
  (rows) => {
    const map: Record<string, string> = {}
    for (const row of rows) {
      const key = String(row.key || '').trim()
      if (!key || PROTECTED_WEBHOOK_HEADERS.has(key.toLowerCase())) continue
      map[key] = String(row.value ?? '')
    }
    webhookSettings.value.headers = map
  },
  { deep: true }
)

function addWebhookHeader() {
  webhookHeaderRows.value.push({ id: webhookHeaderUid++, key: '', value: '' })
}

function removeWebhookHeader(index: number) {
  webhookHeaderRows.value.splice(index, 1)
}

async function testBarkNotification() {
  try {
    const result = await settingsStore.testBarkFromForm()
    if (!result.ok) {
      throw new Error(result.error.message || 'Bark 测试失败')
    }
    const data = result.data
    if (data.ok) {
      ElMessage.success(data.message || '测试通知已发送')
      return
    }
    if (Array.isArray(data.failed_urls) && data.failed_urls.length > 0) {
      ElMessage.error(`${data.message}\n失败 URL: ${data.failed_urls.join(', ')}`)
      return
    }
    ElMessage.error(data.message || 'Bark 测试失败')
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : 'Bark 测试失败')
  }
}

async function testTelegramNotification() {
try {
const result = await settingsStore.testTelegramFromForm()
if (!result.ok) {
throw new Error(result.error.message || 'Telegram 测试失败')
}
if (result.data.ok) {
ElMessage.success(result.data.message || '测试通知已发送')
} else {
ElMessage.error(result.data.message || 'Telegram 测试失败')
}
} catch (e: unknown) {
ElMessage.error(e instanceof Error ? e.message : 'Telegram 测试失败')
}
}

async function testFeishuNotification() {
try {
const result = await settingsStore.testFeishuFromForm()
if (!result.ok) {
throw new Error(result.error.message || '飞书测试失败')
}
if (result.data.ok) {
ElMessage.success(result.data.message || '测试通知已发送')
} else {
ElMessage.error(result.data.message || '飞书测试失败')
}
} catch (e: unknown) {
ElMessage.error(e instanceof Error ? e.message : '飞书测试失败')
}
}

async function testPushplusNotification() {
try {
const result = await settingsStore.testPushplusFromForm()
if (!result.ok) {
throw new Error(result.error.message || 'Pushplus 测试失败')
}
if (result.data.ok) {
ElMessage.success(result.data.message || '测试通知已发送')
} else {
ElMessage.error(result.data.message || 'Pushplus 测试失败')
}
} catch (e: unknown) {
ElMessage.error(e instanceof Error ? e.message : 'Pushplus 测试失败')
}
}

async function testEmailNotification() {
  try {
    const result = await settingsStore.testEmailFromForm()
    if (!result.ok) {
      throw new Error(result.error.message || 'Email 测试失败')
    }
    const data = result.data
    if (data.ok) {
      ElMessage.success(data.message || '测试邮件已发送')
      return
    }
    ElMessage.error(data.message || 'Email 测试失败')
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : 'Email 测试失败')
  }
}

function addBarkUrl() {
  if (!barkSettings.value.urls) {
     barkSettings.value.urls = []
  }
  barkSettings.value.urls.push('')
}

function removeBarkUrl(index: number) {
  barkSettings.value.urls.splice(index, 1)
}



watch(() => emailForm.value.smtp_port, (newPort) => {
  if (Number(newPort) === 465) {
    emailForm.value.use_ssl = true
  }
})



import CoreManagement from '../components/CoreManagement.vue'
import SettingsSecurityCredentials from '../components/SettingsSecurityCredentials.vue'
import SettingsSecurityServerPort from '../components/SettingsSecurityServerPort.vue'
import SettingsSecurityRateLimit from '../components/SettingsSecurityRateLimit.vue'
import SettingsMcpTokenManager from '../components/SettingsMcpTokenManager.vue'
import SettingsGlobalDebugMode from '../components/SettingsGlobalDebugMode.vue'
import SettingsGlobalDisplaySettings from '../components/SettingsGlobalDisplaySettings.vue'
import SettingsGlobalSiteInfo from '../components/SettingsGlobalSiteInfo.vue'
import SettingsGlobalVoWiFiBehavior from '../components/SettingsGlobalVoWiFiBehavior.vue'
import SettingsGlobalPersonalization from '../components/SettingsGlobalPersonalization.vue'
import SettingsMcpConfig from '../components/SettingsMcpConfig.vue'
import { systemService, type UpdateInfo } from '../services/system'

const updateInfo = ref<UpdateInfo | null>(null)
const checkingUpdate = ref(false)
let hasAutoChecked = false

async function autoCheckUpdate() {
  if (hasAutoChecked || !repoAvailable.value) return
  hasAutoChecked = true
  checkingUpdate.value = true
  try {
    const res = await systemService.checkUpdate()
    if (res.ok) {
      updateInfo.value = res.data
    }
  } catch {
    // 静默失败，不打扰用户
  } finally {
    checkingUpdate.value = false
  }
}

watch(activeTab, (tab) => {
  if (tab === 'about') {
    autoCheckUpdate()
  }
})

const updateRepoDialogOpen = ref(false)
const updateRepoForm = ref({ url: '' })
const updateRepoLoading = ref(false)

onMounted(() => {
  loadNotifications()
  loadSystemInfo()
  loadSMSRateLimit()
  loadUpdateRepo()
})

async function openUpdateRepoDialog() {
  await loadUpdateRepo()
  updateRepoDialogOpen.value = true
}

async function loadUpdateRepo() {
  try {
    const res = await systemService.getUpdateRepo()
    if (res.ok) {
      if (res.data.owner && res.data.name) {
        updateRepoForm.value.url = `https://github.com/${res.data.owner}/${res.data.name}`
      } else {
        updateRepoForm.value.url = ''
      }
    }
  } catch {
    // keep defaults
  }
}

const repoAvailable = computed(() => {
  return !!updateRepoForm.value.url
})

function parseGitHubUrl(url: string): { owner: string; name: string } | null {
  const match = url.match(/github\.com\/([^/]+)\/([^/]+)/)
  if (match) {
    return { owner: match[1], name: match[2].replace(/\.git$/, '') }
  }
  return null
}

async function saveUpdateRepo() {
  const url = updateRepoForm.value.url.trim()
  if (!url) {
    ElMessage.error('仓库地址不能为空')
    return
  }
  const parsed = parseGitHubUrl(url)
  if (!parsed) {
    ElMessage.error('无效的 GitHub 仓库地址，应为 https://github.com/owner/name')
    return
  }
  updateRepoLoading.value = true
  try {
    const res = await systemService.saveUpdateRepo(parsed.owner, parsed.name)
    if (!res.ok) throw new Error(res.error.message || '保存失败')
    ElMessage.success('Release 源已更新')
    updateRepoDialogOpen.value = false
    await loadUpdateRepo()
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    updateRepoLoading.value = false
  }
}

async function deleteUpdateRepo() {
  try {
    await systemService.deleteUpdateRepo()
    ElMessage.success('配置已删除')
    updateRepoForm.value.url = ''
    await loadUpdateRepo()
  } catch {
    ElMessage.error('删除失败')
  }
}

onBeforeUnmount(() => {
})
</script>

<template>
  <div class="settings-page h-[calc(100svh-56px-48px)] flex flex-col">
    <div class="settings-card flex-1 min-h-0 flex flex-col overflow-hidden">

      <el-tabs v-model="activeTab" class="settings-tabs">
        <el-tab-pane name="notify">
          <template #label>
            <div class="flex items-center gap-1.5">
              <el-icon size="16"><Alert24Regular /></el-icon>
              <span>通知</span>
            </div>
          </template>
        </el-tab-pane>
        <el-tab-pane name="global">
          <template #label>
            <div class="flex items-center gap-1.5">
              <el-icon size="16"><Globe24Regular /></el-icon>
              <span>全局</span>
            </div>
          </template>
        </el-tab-pane>
        <el-tab-pane name="mcp">
          <template #label>
            <div class="flex items-center gap-1.5">
              <el-icon size="16"><WindowConsole20Regular /></el-icon>
              <span>MCP</span>
            </div>
          </template>
        </el-tab-pane>
        <el-tab-pane name="security">
          <template #label>
            <div class="flex items-center gap-1.5">
              <el-icon size="16"><Key24Regular /></el-icon>
              <span>安全</span>
            </div>
          </template>
        </el-tab-pane>
        <el-tab-pane name="about">
          <template #label>
            <div class="flex items-center gap-1.5">
              <el-icon size="16"><Server24Regular /></el-icon>
              <span>关于</span>
            </div>
          </template>
        </el-tab-pane>
      </el-tabs>

      <!-- ═══════════ 通知 Tab ═══════════ -->
      <div v-show="activeTab === 'notify'" class="flex-1 min-h-0 overflow-auto">
        <div class="p-6">
          <div class="flex items-center justify-between mb-6">
            <div class="flex items-center gap-3">
              <div class="settings-icon-box">
                <el-icon size="20"><Alert24Regular /></el-icon>
              </div>
              <div>
                <h3 class="settings-card-title">通知配置</h3>
                <p class="settings-card-desc">Telegram / 飞书 / QQ / Bark / Email / Pushplus / Webhook</p>
              </div>
            </div>
            <el-button type="primary" :loading="savingNotifications" :disabled="loadingNotifications" @click="saveNotifications" class="!border-0">
              <el-icon><Save24Regular /></el-icon>
              保存通知配置
            </el-button>
          </div>

          <div v-if="loadingNotifications" class="text-sm" style="color: var(--muted-foreground);">正在加载通知配置…</div>

          <div v-else class="settings-inner-card">
            <el-tabs v-model="activeNotifyTab">
              <!-- Telegram -->
              <el-tab-pane label="Telegram Bot" name="telegram" class="pt-2">
                <div class="flex items-center justify-between mb-4">
                  <div class="flex items-center gap-2">
                    <div class="settings-toggle-title">启用 Telegram 机器人</div>
                  </div>
                  <div class="flex items-center gap-2">
                    <el-button
                      size="small"
                      type="primary"
                      plain
                      :loading="testingTelegram"
                      :disabled="!telegramForm.enabled"
                      @click="testTelegramNotification"
                    >
                      测试通知
                    </el-button>
                    <el-switch v-model="telegramForm.enabled" />
                  </div>
                </div>

                <div class="space-y-4">
                  <div class="space-y-1">
                    <label class="settings-form-label">Bot Token</label>
                    <el-input v-model="telegramForm.bot_token" :disabled="!telegramForm.enabled" placeholder="xxxx:yyyy" />
                  </div>
                  <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
                    <div class="space-y-1">
                      <label class="settings-form-label">Chat ID</label>
                      <el-input v-model="telegramForm.chat_id" :disabled="!telegramForm.enabled" type="number" inputmode="numeric" placeholder="例如 123456" />
                    </div>
                    <div class="space-y-1">
                      <label class="settings-form-label">Admin ID</label>
                      <el-input v-model="telegramForm.admin_id" :disabled="!telegramForm.enabled" type="number" inputmode="numeric" placeholder="例如 123456" />
                    </div>
                  </div>
                  <div class="space-y-1">
                    <label class="settings-form-label">TG API 反代（可选）</label>
                    <el-input v-model="telegramForm.base_url" :disabled="!telegramForm.enabled" placeholder="留空直连 api.telegram.org；需要反代时填写" />
                    <div class="settings-form-hint">反向代理地址 (例如 https://api.telegram.org/bot%s/%s)</div>
                  </div>
                  <div class="space-y-1">
                    <label class="settings-form-label">HTTP 代理（可选）</label>
                    <el-input v-model="telegramForm.proxy" :disabled="!telegramForm.enabled" placeholder="例如 http://127.0.0.1:7890" />
                    <div class="settings-form-hint">用于连接 API 服务器的 HTTP 代理</div>
                  </div>
                </div>
              </el-tab-pane>

              <!-- 飞书 -->
              <el-tab-pane label="飞书 Bot" name="feishu" class="pt-2">
                <div class="flex items-center justify-between mb-4">
                  <div class="flex items-center gap-2">
                    <div class="settings-toggle-title">启用飞书机器人</div>
                  </div>
                  <div class="flex items-center gap-2">
                    <el-button
                      size="small"
                      type="primary"
                      plain
                      :loading="testingFeishu"
                      :disabled="!feishuForm.enabled"
                      @click="testFeishuNotification"
                    >
                      测试通知
                    </el-button>
                    <el-switch v-model="feishuForm.enabled" />
                  </div>
                </div>

                <div class="space-y-4">
                  <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
                    <div class="space-y-1">
                      <label class="settings-form-label">App ID</label>
                      <el-input v-model="feishuForm.app_id" :disabled="!feishuForm.enabled" placeholder="cli_xxxx" />
                    </div>
                    <div class="space-y-1">
                      <label class="settings-form-label">App Secret</label>
                      <el-input v-model="feishuForm.app_secret" :disabled="!feishuForm.enabled" type="password" show-password placeholder="••••••••" />
                    </div>
                  </div>
                  <div class="space-y-1">
                    <label class="settings-form-label">Chat IDs</label>
                    <el-input v-model="feishuForm.chat_ids" :disabled="!feishuForm.enabled" placeholder="多个群组用英文逗号分隔" />
                    <div class="settings-form-hint">飞书群聊的 Chat ID (oc_xxxx)，可通过飞书开放平台 API 获取，支持逗号分隔多个群组。</div>
                  </div>
                  <div class="settings-info-hint">
                    <strong>配置说明：</strong>
                    <ol class="list-decimal ml-4 mt-1 space-y-1">
                      <li>在<a href="https://open.feishu.cn" target="_blank" class="underline" style="color: var(--brand);">飞书开放平台</a>创建自建应用，启用「机器人」能力</li>
                      <li>在「事件与回调 → 事件配置」中选择「使用长连接接收事件」</li>
                      <li>添加 <code>im:message</code> 和 <code>im:message:send_as_bot</code> 权限</li>
                    </ol>
                  </div>
                </div>
              </el-tab-pane>

              <!-- QQ -->
              <el-tab-pane label="QQ Bot" name="qq" class="pt-2">
                <div class="flex items-center justify-between mb-4">
                  <div class="flex items-center gap-2">
                    <div class="settings-toggle-title">启用 QQ 机器人</div>
                  </div>
                  <el-switch v-model="qqForm.enabled" />
                </div>

                <div class="space-y-4">
                  <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
                    <div class="space-y-1">
                      <label class="settings-form-label">App ID</label>
                      <el-input v-model="qqForm.app_id" :disabled="!qqForm.enabled" placeholder="QQ Bot App ID" />
                    </div>
                    <div class="space-y-1">
                      <label class="settings-form-label">App Secret</label>
                      <el-input v-model="qqForm.app_secret" :disabled="!qqForm.enabled" type="password" show-password placeholder="••••••••" />
                    </div>
                  </div>
                  <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
                    <div class="space-y-1">
                      <label class="settings-form-label">Group IDs (群聊)</label>
                      <el-input v-model="qqForm.group_ids" :disabled="!qqForm.enabled" placeholder="群聊 OpenID，多个使用逗号分隔" />
                    </div>
                    <div class="space-y-1">
                      <label class="settings-form-label">User IDs (私聊)</label>
                      <el-input v-model="qqForm.direct_ids" :disabled="!qqForm.enabled" placeholder="用户 OpenID，多个使用逗号分隔" />
                    </div>
                  </div>
                  <div class="settings-info-hint" style="border-color: color-mix(in oklab, var(--warning) 20%, var(--border));">
                    <ol class="list-decimal ml-4 mt-1 space-y-1">
                      <li>QQbot申请地址：<a href="https://q.qq.com/qqbot/openclaw/index.html" target="_blank" class="underline" style="color: var(--warning);">官方控制台</a></li>
                      <li>向机器人发送消息后，去系统日志查看 OpenID，填入后 Bot 只对匹配的会话进行回复和推送。</li>
                    </ol>
                  </div>
                </div>
              </el-tab-pane>

              <!-- Bark -->
              <el-tab-pane label="Bark" name="bark" class="pt-2">
                <div class="flex items-center justify-between mb-4">
                  <div class="flex items-center gap-2">
                    <div class="settings-toggle-title">启用 Bark 推送</div>
                  </div>
                  <div class="flex items-center gap-2">
                    <el-button
                      size="small"
                      type="primary"
                      plain
                      :loading="testingBark"
                      :disabled="!barkSettings.enabled || !hasValidBarkURLs"
                      @click="testBarkNotification"
                    >
                      测试通知
                    </el-button>
                    <el-switch v-model="barkSettings.enabled" />
                  </div>
                </div>

                <div class="space-y-4">
                  <div class="space-y-2">
                    <div class="flex items-center justify-between">
                      <label class="settings-form-label">目标 URLs</label>
                      <el-button size="small" type="primary" plain @click="addBarkUrl" :disabled="!barkSettings.enabled">
                         <el-icon><Add20Regular /></el-icon>
                         <span class="ml-1">添加 URL</span>
                      </el-button>
                    </div>
                    
                    <div v-if="barkSettings.urls && barkSettings.urls.length === 0" class="settings-empty-hint">
                      尚未配置任何 Bark URL，点击右侧添加按钮。
                    </div>

                    <div v-for="(url, index) in barkSettings.urls" :key="index" class="flex items-center gap-2">
                       <el-input v-model="barkSettings.urls[index]" :disabled="!barkSettings.enabled" placeholder="https://api.day.app/YOUR_KEY/" class="flex-1" />
                       <el-button type="danger" plain @click="removeBarkUrl(index)" :disabled="!barkSettings.enabled">
                          <el-icon><Delete20Regular /></el-icon>
                       </el-button>
                    </div>
                  </div>

                  <div class="space-y-1">
                    <label class="settings-form-label">分组 (Group)</label>
                    <el-input v-model="barkSettings.group" :disabled="!barkSettings.enabled" placeholder="例如 vohive" />
                    <div class="settings-form-hint">iOS 设备上的通知分组。</div>
                  </div>

                  <div class="space-y-1">
                    <label class="settings-form-label">通知级别 (Level)</label>
                    <el-select v-model="barkSettings.level" :disabled="!barkSettings.enabled" placeholder="选择通知级别" class="w-full">
                      <el-option label="时效性 (timeSensitive)" value="timeSensitive" />
                      <el-option label="积极 (active)" value="active" />
                      <el-option label="被动 (passive)" value="passive" />
                    </el-select>
                    <div class="settings-form-hint">iOS 的专注模式/打扰规则会根据此级别决定是否亮屏。</div>
                  </div>

                  <div class="space-y-1">
                    <label class="settings-form-label">图标 (Icon)</label>
                    <el-input v-model="barkSettings.icon" :disabled="!barkSettings.enabled" placeholder="图标 URL，可选" />
                  </div>
                </div>
              </el-tab-pane>

              <!-- Email -->
              <el-tab-pane label="Email" name="email" class="pt-2">
                <div class="flex items-center justify-between mb-4">
                  <div class="flex items-center gap-2">
                    <div class="settings-toggle-title">启用 Email 推送</div>
                  </div>
                  <div class="flex items-center gap-2">
                    <el-button
                      size="small"
                      type="primary"
                      plain
                      :loading="testingEmail"
                      :disabled="!emailForm.enabled || !hasValidEmailConfig"
                      @click="testEmailNotification"
                    >
                      测试通知
                    </el-button>
                    <el-switch v-model="emailForm.enabled" />
                  </div>
                </div>

                <div class="space-y-4">
                  <div class="grid grid-cols-1 sm:grid-cols-10 gap-4">
                    <div class="space-y-1 sm:col-span-5">
                      <label class="settings-form-label">SMTP 主机</label>
                      <el-input v-model="emailForm.smtp_host" :disabled="!emailForm.enabled" placeholder="smtp.example.com" />
                    </div>
                    <div class="space-y-1 sm:col-span-3">
                      <label class="settings-form-label">SMTP 端口</label>
                      <el-input v-model="emailForm.smtp_port" :disabled="!emailForm.enabled" type="number" inputmode="numeric" placeholder="465 / 587" />
                    </div>
                    <div class="space-y-1 sm:col-span-2">
                      <label class="settings-form-label">使用 SSL/TLS </label>
                      <div class="h-10 flex items-center">
                        <el-switch v-model="emailForm.use_ssl" :disabled="!emailForm.enabled" />
                      </div>
                    </div>
                  </div>
                  <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
                    <div class="space-y-1">
                      <label class="settings-form-label">用户名 (Username)</label>
                      <el-input v-model="emailForm.username" :disabled="!emailForm.enabled" placeholder="邮箱账号" />
                    </div>
                    <div class="space-y-1">
                      <label class="settings-form-label">密码 (Password)</label>
                      <el-input v-model="emailForm.password" :disabled="!emailForm.enabled" type="password" show-password placeholder="邮箱密码或授权码" />
                    </div>
                  </div>
                  <div class="space-y-1">
                    <label class="settings-form-label">发件人地址 (From)</label>
                    <el-input v-model="emailForm.from_address" :disabled="!emailForm.enabled" placeholder="例如 noreply@example.com" />
                  </div>
                  <div class="space-y-1">
                    <label class="settings-form-label">收件人地址 (To)</label>
                    <el-input v-model="emailForm.to_addresses" :disabled="!emailForm.enabled" placeholder="多个收件人请用英文逗号分隔" />
                  </div>
                </div>
              </el-tab-pane>

              <!-- Pushplus -->
              <el-tab-pane label="Pushplus" name="pushplus" class="pt-2">
                <div class="flex items-center justify-between mb-4">
                  <div class="flex items-center gap-2">
                    <div class="settings-toggle-title">启用 Pushplus 推送</div>
                  </div>
                  <div class="flex items-center gap-2">
                    <el-button
                      size="small"
                      type="primary"
                      plain
                      :loading="testingPushplus"
                      :disabled="!pushplusForm.enabled"
                      @click="testPushplusNotification"
                    >
                      测试通知
                    </el-button>
                    <el-switch v-model="pushplusForm.enabled" />
                  </div>
                </div>

                <div class="space-y-4">
                  <div class="space-y-1">
                    <label class="settings-form-label">Token</label>
                    <el-input v-model="pushplusForm.token" :disabled="!pushplusForm.enabled" placeholder="Pushplus 用户的 Token" />
                  </div>
                  <div class="space-y-1">
                    <label class="settings-form-label">群组编码 (Topic)</label>
                    <el-input v-model="pushplusForm.topic" :disabled="!pushplusForm.enabled" placeholder="群组编码，不填则发给个人" />
                  </div>
                  <div class="space-y-1">
                    <label class="settings-form-label">渠道 (Channel)</label>
                    <el-select v-model="pushplusForm.channel" :disabled="!pushplusForm.enabled" placeholder="选择渠道" class="w-full">
                      <el-option label="微信 (wechat)" value="wechat" />
                      <el-option label="Webhook (webhook)" value="webhook" />
                      <el-option label="企业微信 (cp)" value="cp" />
                      <el-option label="邮件 (mail)" value="mail" />
                    </el-select>
                  </div>
                </div>
              </el-tab-pane>

              <!-- Webhook -->
              <el-tab-pane label="Webhook" name="webhook" class="pt-2">
                <div class="flex items-center justify-between mb-4">
                  <div class="flex items-center gap-2">
                    <div class="settings-toggle-title">启用 Webhook 推送</div>
                  </div>
                  <div class="flex items-center gap-2">
                    <el-button
                      size="small"
                      type="primary"
                      plain
                      :loading="testingWebhook"
                      :disabled="!webhookSettings.enabled || !hasValidWebhookURLs"
                      @click="testWebhookNotification"
                    >
                      测试通知
                    </el-button>
                    <el-switch v-model="webhookSettings.enabled" />
                  </div>
                </div>

                <div class="space-y-4">
                  <div class="space-y-2">
                    <div class="flex items-center justify-between">
                      <label class="settings-form-label">目标 URLs</label>
                      <el-button size="small" type="primary" plain @click="addWebhookUrl" :disabled="!webhookSettings.enabled">
                         <el-icon><Add20Regular /></el-icon>
                         <span class="ml-1">添加 URL</span>
                      </el-button>
                    </div>
                    
                    <div v-if="webhookSettings.urls && webhookSettings.urls.length === 0" class="settings-empty-hint">
                      尚未配置任何 Webhook URL，点击右侧添加按钮。
                    </div>

                    <div v-for="(url, index) in webhookSettings.urls" :key="index" class="flex items-center gap-2">
                       <el-input v-model="webhookSettings.urls[index]" :disabled="!webhookSettings.enabled" placeholder="https://..." class="flex-1" />
                       <el-button type="danger" plain @click="removeWebhookUrl(index)" :disabled="!webhookSettings.enabled">
                          <el-icon><Delete20Regular /></el-icon>
                       </el-button>
                    </div>
                  </div>

                  <div class="space-y-1">
                    <label class="settings-form-label">数字签名密钥 (Secret)</label>
                    <el-input v-model="webhookSettings.secret" :disabled="!webhookSettings.enabled" placeholder="用于 HMAC-SHA256 签名，选填" />
                    <div class="settings-form-hint">若配置，将通过请求头 X-Vohive-Signature 提供 payload 验证。</div>
                  </div>

                  <div class="space-y-2">
                    <div class="flex items-center justify-between">
                      <label class="settings-form-label">自定义请求头 (Headers)</label>
                      <el-button size="small" type="primary" plain @click="addWebhookHeader" :disabled="!webhookSettings.enabled">
                        <el-icon><Add20Regular /></el-icon>
                        <span class="ml-1">添加 Header</span>
                      </el-button>
                    </div>

                    <div v-if="webhookHeaderRows.length === 0" class="settings-empty-hint">
                      尚未配置自定义请求头，例如 Authorization、X-Api-Key 等。
                    </div>

                    <div v-for="(row, index) in webhookHeaderRows" :key="row.id" class="flex items-center gap-2">
                      <el-select
                        v-model="row.key"
                        :disabled="!webhookSettings.enabled"
                        filterable
                        allow-create
                        default-first-option
                        placeholder="选择或输入 Header 名"
                        class="flex-1"
                      >
                        <el-option v-for="name in COMMON_WEBHOOK_HEADERS" :key="name" :label="name" :value="name" />
                      </el-select>
                      <el-input v-model="row.value" :disabled="!webhookSettings.enabled" placeholder="值，如 Bearer xxx" class="flex-1" />
                      <el-button type="danger" plain @click="removeWebhookHeader(index)" :disabled="!webhookSettings.enabled">
                        <el-icon><Delete20Regular /></el-icon>
                      </el-button>
                    </div>
                    <div class="settings-form-hint">
                      Content-Type 与 X-Vohive-Signature 为系统保留头，自定义同名头会被忽略。
                    </div>
                  </div>

                  <div class="space-y-1">
                    <label class="settings-form-label">文本模板 (Text Template)</label>
                    <el-input
                      v-model="webhookSettings.text_template"
                      :disabled="!webhookSettings.enabled"
                      type="textarea"
                      :rows="2"
                      placeholder="{{device_label}} {{text}}"
                    />
                    <div class="settings-form-hint">
                      支持占位符：<code v-pre>{{text}}</code>、<code v-pre>{{event}}</code>、<code v-pre>{{timestamp}}</code>、<code v-pre>{{device_id}}</code>、<code v-pre>{{device_name}}</code>、<code v-pre>{{device_label}}</code>。留空则直接发送原始 text。
                    </div>
                  </div>
                  
                  <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
                    <div class="space-y-1">
                      <label class="settings-form-label">请求超时 (ms)</label>
                      <el-input-number v-model="webhookSettings.timeout_ms" :min="1000" :max="60000" :disabled="!webhookSettings.enabled" class="w-full !w-full" controls-position="right" />
                    </div>
                    <div class="space-y-1">
                      <label class="settings-form-label">最大重试次数</label>
                      <el-input-number v-model="webhookSettings.retry_max" :min="0" :max="10" :disabled="!webhookSettings.enabled" class="w-full !w-full" controls-position="right" />
                    </div>
                  </div>
                </div>
              </el-tab-pane>
            </el-tabs>
          </div>
        </div>
      </div>

      <!-- ═══════════ 全局 Tab ═══════════ -->
      <div v-show="activeTab === 'global'" class="flex-1 min-h-0 overflow-auto">
        <div class="p-6">
          <div class="flex items-center gap-3 mb-6">
            <div class="settings-icon-box">
              <el-icon size="20"><Globe24Regular /></el-icon>
            </div>
            <div>
              <h3 class="settings-card-title">全局配置</h3>
              <p class="settings-card-desc">所有设备共享的全局设置</p>
            </div>
          </div>

          <!-- FAQ 折叠卡片：短信发送限制 -->
          <div class="faq-card" v-loading="smsLimitLoading">
            <div class="faq-header" @click="smsLimitExpanded = !smsLimitExpanded">
              <span class="faq-title">短信发送限制</span>
              <el-icon class="faq-arrow" :class="{ expanded: smsLimitExpanded }" size="16">
                <ChevronDown20Regular />
              </el-icon>
            </div>
            <div v-show="smsLimitExpanded" class="faq-body">
              <div class="flex items-center justify-between mb-4">
                <div>
                  <div class="faq-item-title">短信发送限制</div>
                  <div class="faq-item-desc">所有设备、所有卡共享同一个限速器</div>
                </div>
                <el-button size="small" type="primary" :loading="smsLimitSaving" @click="saveSMSRateLimit" class="!border-0">
                  保存限制
                </el-button>
              </div>
              <div class="flex flex-wrap items-center gap-6">
                <div class="flex items-center gap-2">
                  <span class="faq-field-label">每小时最多</span>
                  <el-input-number v-model="smsHourlyLimit" :min="1" :max="100" size="small" controls-position="right" />
                  <span class="faq-field-unit">条</span>
                </div>
                <div class="flex items-center gap-2">
                  <span class="faq-field-label">每天最多</span>
                  <el-input-number v-model="smsDailyLimit" :min="1" :max="500" size="small" controls-position="right" />
                  <span class="faq-field-unit">条</span>
                </div>
              </div>
            </div>
          </div>

          <!-- FAQ 折叠卡片：调试模式 -->
          <SettingsGlobalDebugMode />

          <!-- FAQ 折叠卡片：显示设置 -->
          <SettingsGlobalDisplaySettings />

          <!-- FAQ 折叠卡片：站点信息 -->
          <SettingsGlobalSiteInfo />

          <!-- FAQ 折叠卡片：个性化配置 -->
          <SettingsGlobalPersonalization />

          <!-- FAQ 折叠卡片：VoWiFi 行为控制 -->
          <SettingsGlobalVoWiFiBehavior />
        </div>
      </div>

      <!-- ═══════════ MCP Tab ═══════════ -->
      <div v-show="activeTab === 'mcp'" class="flex-1 min-h-0 overflow-auto">
        <div class="p-6">
          <div class="flex items-center gap-3 mb-6">
            <div class="settings-icon-box">
              <el-icon size="20"><WindowConsole20Regular /></el-icon>
            </div>
            <div>
              <h3 class="settings-card-title">MCP</h3>
              <p class="settings-card-desc">Model Context Protocol 配置与资源</p>
            </div>
          </div>
          <SettingsMcpConfig />

          <!-- Token 管理 -->
          <SettingsMcpTokenManager />
        </div>
      </div>

      <!-- ═══════════ 安全 Tab ═══════════ -->
      <div v-show="activeTab === 'security'" class="flex-1 min-h-0 overflow-auto">
        <div class="p-6">
          <div class="flex items-center gap-3 mb-6">
            <div class="settings-icon-box">
              <el-icon size="20"><Key24Regular /></el-icon>
            </div>
            <div>
              <h3 class="settings-card-title">安全配置</h3>
              <p class="settings-card-desc">登录凭据、端口与限速</p>
            </div>
          </div>

          <!-- 登录凭据 -->
          <SettingsSecurityCredentials :username="systemInfo.username" />

          <!-- Web 管理端口 -->
          <SettingsSecurityServerPort />

          <!-- 登录限速 -->
          <SettingsSecurityRateLimit />

        </div>
      </div>

      <!-- ═══════════ 关于 Tab ═══════════ -->
      <div v-show="activeTab === 'about'" class="flex-1 min-h-0 overflow-auto">
        <div class="p-6">
          <div class="flex items-center gap-3 mb-6">
            <div class="settings-icon-box">
              <el-icon size="20"><Server24Regular /></el-icon>
            </div>
            <div>
              <h3 class="settings-card-title">系统信息</h3>
              <p class="settings-card-desc">运行环境</p>
            </div>
          </div>

          <div class="settings-inner-card p-4">
            <div class="space-y-4 text-sm">
            <!-- 核心版本 -->
            <div class="settings-info-row">
              <FieldRow label="核心版本">
                <div class="flex items-center justify-end gap-2">
                  <span class="font-mono">{{ systemInfo.version || '--' }}</span>
                  <el-tag v-if="checkingUpdate" type="info" size="small">检查中...</el-tag>
                  <el-tag v-else-if="updateInfo?.has_update" type="warning" size="small">可更新至 {{ updateInfo.latest_version }}</el-tag>
                  <el-tag v-else-if="updateInfo && !updateInfo.has_update" type="success" size="small">已是最新</el-tag>
                </div>
              </FieldRow>
            </div>

            <!-- 构建时间 -->
            <div class="settings-info-row">
              <FieldRow label="构建时间" :value="systemInfo.build_time" monospace />
            </div>

            <!-- Release 仓库 -->
            <div class="settings-info-row">
              <FieldRow label="Release 仓库">
                <div class="flex items-center justify-end gap-2">
                  <span class="flex items-center gap-1.5">
                    <span class="settings-status-dot" :class="repoAvailable ? 'ok' : 'error'"></span>
                    <span v-if="repoAvailable" class="font-mono text-xs">{{ updateRepoForm.url }}</span>
                    <span v-else style="color: var(--muted-foreground);">未配置</span>
                  </span>
                  <el-button size="small" @click="openUpdateRepoDialog">配置 Release</el-button>
                </div>
              </FieldRow>
            </div>

            <!-- 配置路径 -->
            <div class="settings-info-row">
              <FieldRow label="配置路径" :value="systemInfo.config" monospace copyable />
            </div>
        </div>
        </div>

        <!-- 核心管理 -->
        <div class="mt-4">
          <CoreManagement :repo-available="repoAvailable" />
        </div>

        <!-- API 文档（核心管理下方） -->
          <div class="settings-api-card mt-4">
            <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
              <div class="min-w-0">
                <div>
                  <div class="settings-api-title">API 文档</div>
                  <div class="settings-card-desc">Scalar API Reference 交互式文档</div>
                </div>
              </div>
              <el-button
                type="primary"
                class="self-start sm:self-center shrink-0 !border-0"
                :disabled="!systemInfo.docs?.docs_ui"
                @click="openAPIDocs"
              >
                打开 API 文档
              </el-button>
            </div>
          </div>
        </div>
      </div>

    </div>

    <!-- ═══════════ Release 源配置对话框 ═══════════ -->
    <el-dialog v-model="updateRepoDialogOpen" title="配置 Release 源" width="440px" :close-on-click-modal="false">
      <div v-loading="updateRepoLoading" class="space-y-4">
        <div class="space-y-1">
          <label class="settings-form-label">GitHub 仓库地址</label>
          <el-input v-model="updateRepoForm.url" placeholder="https://github.com/{OWNER}/{REPO}" />
          <div class="settings-form-hint">粘贴完整仓库地址，系统将自动解析 owner 和 name</div>
        </div>
        <div class="flex justify-between gap-2">
          <el-button @click="updateRepoDialogOpen = false">取消</el-button>
          <div class="flex gap-2">
            <el-button v-if="repoAvailable" type="danger" :loading="updateRepoLoading" @click="deleteUpdateRepo">删除配置</el-button>
            <el-button type="primary" :loading="updateRepoLoading" @click="saveUpdateRepo" class="!border-0">保存</el-button>
          </div>
        </div>
      </div>
    </el-dialog>

  </div>
</template>

<style scoped>
.settings-page {
  container-type: inline-size;
}

.settings-card {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--card);
  box-shadow: var(--console-shadow-sm);
}

.settings-tabs {
  flex-shrink: 0;
}

.settings-tabs :deep(.el-tabs__header) {
  margin-bottom: 0;
  height: 60px;
  padding: 0 16px;
}

.settings-tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
}

.settings-tabs :deep(.el-tabs__content) {
  display: none;
}

.settings-tabs :deep(.el-input-number) {
  width: 100%;
}

.settings-icon-box {
  width: 38px;
  height: 38px;
  border-radius: 6px;
  background: var(--muted);
  border: 1px solid var(--border);
  color: var(--foreground);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.settings-card-title {
  font-size: 18px;
  font-weight: 700;
  color: var(--foreground);
}

.settings-card-desc {
  font-size: 12px;
  color: var(--muted-foreground);
}

.settings-form-label {
  font-size: 12px;
  font-weight: 700;
  color: var(--foreground);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.settings-form-hint {
  font-size: 12px;
  color: var(--muted-foreground);
  margin-top: 4px;
}

.settings-toggle-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
}

.settings-info-hint {
  font-size: 12px;
  color: var(--muted-foreground);
  background: var(--muted);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 12px;
  margin-top: 8px;
}

.settings-empty-hint {
  font-size: 12px;
  color: var(--muted-foreground);
  padding: 8px 0;
}

.settings-info-row {
  border-bottom: 1px solid var(--border);
  padding-bottom: 12px;
}

.settings-api-card {
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 16px;
  background: var(--muted);
}

.settings-api-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
}

.settings-status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
}

.settings-status-dot.ok {
  background: var(--success);
}

.settings-status-dot.error {
  background: var(--destructive);
}

.settings-inner-card {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--background);
  overflow: hidden;
}

.settings-inner-card :deep(.el-tabs__header) {
  margin-bottom: 0;
  padding: 0 16px;
}

.settings-inner-card :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
}

.settings-inner-card :deep(.el-tabs__content) {
  padding: 0 16px 16px;
}

.faq-card {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--background);
  overflow: hidden;
}

.faq-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 16px;
  cursor: pointer;
  user-select: none;
  transition: background 0.15s;
}

.faq-header:hover {
  background: var(--accent);
}

.faq-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
}

.faq-arrow {
  transition: transform 0.2s;
  color: var(--muted-foreground);
}

.faq-arrow.expanded {
  transform: rotate(180deg);
}

.faq-body {
  padding: 16px;
  border-top: 1px solid var(--border);
}

.faq-item-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
}

.faq-item-desc {
  font-size: 12px;
  color: var(--muted-foreground);
  margin-top: 2px;
}

.faq-field-label {
  font-size: 13px;
  color: var(--muted-foreground);
}

.faq-field-unit {
  font-size: 13px;
  color: var(--muted-foreground);
}

/* ── plain 测试按钮 hover 修复（暗色主题下白底白字问题） ── */
.settings-inner-card :deep(.el-button.is-plain.el-button--primary) {
  --el-button-hover-bg-color: var(--brand);
  --el-button-hover-text-color: var(--brand-foreground);
  --el-button-hover-border-color: var(--brand);
  --el-button-active-bg-color: var(--brand);
  --el-button-active-text-color: var(--brand-foreground);
  --el-button-active-border-color: var(--brand);
}

.settings-inner-card :deep(.el-button.is-plain.el-button--primary:hover) {
  background-color: var(--brand);
  color: var(--brand-foreground);
  border-color: var(--brand);
}

.settings-inner-card :deep(.el-button.is-plain.el-button--danger:hover) {
  background-color: var(--destructive);
  color: white;
  border-color: var(--destructive);
}
</style>
