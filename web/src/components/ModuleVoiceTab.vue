<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { storeToRefs } from 'pinia'
import ModuleVoiceDialer from './ModuleVoiceDialer.vue'
import ModuleVoiceHistory from './ModuleVoiceHistory.vue'
import ModuleVoiceStatus from './ModuleVoiceStatus.vue'
import ModuleVoiceContacts from './ModuleVoiceContacts.vue'
import ModuleVoiceContactAdd from './ModuleVoiceContactAdd.vue'
import ModuleVoiceSettings from './ModuleVoiceSettings.vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Call24Regular,
  Dialpad24Regular,
  History24Regular,
  BookContacts24Regular,
  Settings24Regular,
} from '@vicons/fluent'
import { voiceService } from '../services/voice'
import { api } from '../stores/auth'
import { useDevicesStore } from '../stores/devices'

const props = defineProps<{
  deviceId: string
}>()

// 视图切换
const activeView = ref<'dialer' | 'history' | 'contacts' | 'settings'>('dialer')

const viewTitle = computed(() => {
  switch (activeView.value) {
    case 'dialer': return '拨号盘'
    case 'history': return '通话记录'
    case 'contacts': return '通讯录'
    case 'settings': return '设置'
    default: return ''
  }
})

// --- 当前通话方式（只读，后端自动路由） ---
const devicesStore = useDevicesStore()
const { detail: deviceDetail } = storeToRefs(devicesStore)

type CallMethod = 'vowifi' | 'volte' | 'cs' | 'unavailable'

const currentCallMethod = computed<CallMethod>(() => {
  const d = deviceDetail.value
  if (!d) return 'unavailable'
  const rt = d.vowifi_runtime
  // VoWiFi 就绪：隧道 + IMS + 通话能力
  if (rt?.tunnel_ready && rt?.ims_ready && rt?.call_ready) return 'vowifi'
  // VoLTE 就绪：IMS 注册但无 VoWiFi 隧道
  if (rt?.ims_ready && rt?.call_ready) return 'volte'
  // CS 域：模组已注册到网络（reg_status 1/5/6/7 表示已注册）
  const regStatus = d.modem?.reg_status
  if (regStatus === 1 || regStatus === 5 || regStatus === 6 || regStatus === 7) return 'cs'
  // 有音频设备也算 CS
  if (d.audio_device) return 'cs'
  return 'unavailable'
})

const callMethodLabel = computed(() => {
  switch (currentCallMethod.value) {
    case 'vowifi': return 'VoWiFi'
    case 'volte': return 'VoLTE'
    case 'cs': return 'CS'
    default: return '不可用'
  }
})

const callMethodTag = computed(() => {
  switch (currentCallMethod.value) {
    case 'vowifi': return 'success'
    case 'volte': return 'success'
    case 'cs': return 'warning'
    default: return 'info'
  }
})

// --- SIP.js 连接 ---
const sipConnected = ref(false)

async function loadSipConfigFromBackend(): Promise<{ wsUrl: string; username: string; password: string; realm: string } | null> {
  try {
    const res = await api.get('/settings/voice-gateway')
    const d = res.data
    if (!d) return null
    // 优先 WSS，其次 WS，默认用当前页面 hostname + 5060
    const wssListen = d.sip?.wss_listen
    const wsListen = d.sip?.ws_listen
    const listen = wssListen || wsListen
    const scheme = wssListen ? 'wss' : 'ws'
    const defaultPort = wssListen ? '5062' : '5061'
    let wsUrl: string
    if (listen) {
      const [host, port] = listen.split(':')
      const realHost = host === '0.0.0.0' || host === '::' ? window.location.hostname : host
      wsUrl = `${scheme}://${realHost}:${port || defaultPort}`
    } else {
      wsUrl = `${scheme}://${window.location.hostname}:${defaultPort}`
    }
    // SIP 用户：从配置中获取（单用户模式）
    const user = d.user
    if (user && user.username && user.password) {
      return { wsUrl, username: user.username, password: user.password, realm: d.sip?.realm || 'vohive.local' }
    }
    return null
  } catch {
    return null
  }
}

async function ensureSipConnected() {
  if (!voiceService.isRegistered()) {
    const cfg = await loadSipConfigFromBackend()
    if (!cfg) {
      ElMessage.warning('请先在设置中配置 SIP 服务器和用户')
      return false
    }
    try {
      await voiceService.connect(cfg)
      sipConnected.value = true
    } catch (err) {
      ElMessage.error('SIP 连接失败: ' + (err instanceof Error ? err.message : String(err)))
      return false
    }
  }
  return true
}

// --- 通话状态 ---
type CallState = 'idle' | 'dialing' | 'ringing' | 'connected' | 'hanging'
const callState = ref<CallState>('idle')
const callNumber = ref('')
const callTimer = ref(0)
let timerInterval: ReturnType<typeof setInterval> | null = null

const inCall = computed(() => callState.value !== 'idle')

function startTimer() {
  callTimer.value = 0
  timerInterval = setInterval(() => {
    callTimer.value++
  }, 1000)
}

function stopTimer() {
  if (timerInterval) {
    clearInterval(timerInterval)
    timerInterval = null
  }
}

// 拨号
async function onDial(number: string) {
  if (callState.value !== 'idle') return
  if (!(await ensureSipConnected())) return

  callNumber.value = number
  callState.value = 'dialing'
  try {
    await voiceService.call(number)
  } catch (err) {
    ElMessage.error('拨号失败: ' + (err instanceof Error ? err.message : String(err)))
    callState.value = 'idle'
    callNumber.value = ''
  }
}

// 挂断
function onHangup() {
  showDtmf.value = false
  if (callState.value === 'idle') return
  callState.value = 'hanging'
  stopTimer()
  voiceService.hangup()
  // 状态由 SIP.js 回调更新
}

// 接答
async function onAnswer() {
  if (callState.value !== 'ringing') return
  try {
    await voiceService.answer()
    callState.value = 'connected'
    startTimer()
  } catch (err) {
    ElMessage.error('接听失败: ' + (err instanceof Error ? err.message : String(err)))
  }
}

// SIP.js 状态回调
const unsubVoice = voiceService.onCallEvent((event) => {
  callState.value = event.state
  if (event.number) callNumber.value = event.number
  if (event.state === 'connected') startTimer()
  if (event.state === 'idle' || event.state === 'hanging') stopTimer()
  if (event.state === 'idle') {
    callNumber.value = ''
    callTimer.value = 0
    dialerRef.value?.clear()
  }
})

// DTMF
function onDtmfKey(key: string) {
  voiceService.sendDTMF(key)
}

// DTMF 键盘展开
const showDtmf = ref(false)

// 从通话记录回拨
function onCallback(number: string) {
  if (callState.value !== 'idle') {
    ElMessage.warning('当前正在通话中')
    return
  }
  activeView.value = 'dialer'
  dialerRef.value?.setNumber(number)
}

// 切换设备时重置
watch(() => props.deviceId, () => {
  callState.value = 'idle'
  callNumber.value = ''
  callTimer.value = 0
  stopTimer()
  activeView.value = 'dialer'
  dialerRef.value?.clear()
  loadContacts()
})

const dialerRef = ref<InstanceType<typeof ModuleVoiceDialer> | null>(null)
const settingsRef = ref<InstanceType<typeof ModuleVoiceSettings> | null>(null)

// --- 设置编辑模式 ---
const settingsEditMode = ref(false)

async function onSettingsToggleEdit() {
  if (settingsEditMode.value) {
    // 当前是编辑模式 → 保存
    const ok = await settingsRef.value?.saveConfig()
    if (ok) {
      settingsEditMode.value = false
      settingsRef.value?.setReadonly(true)
    }
  } else {
    // 进入编辑模式
    settingsEditMode.value = true
    settingsRef.value?.setReadonly(false)
  }
}

// --- 通话记录编辑模式 ---
const historyEditMode = ref(false)
const selectedHistoryIds = ref<Set<number>>(new Set())

function toggleHistorySelect(id: number) {
  if (selectedHistoryIds.value.has(id)) selectedHistoryIds.value.delete(id)
  else selectedHistoryIds.value.add(id)
  selectedHistoryIds.value = new Set(selectedHistoryIds.value)
}

function deleteSelectedHistory() {
  ElMessageBox.confirm(
    `确定要删除选中的 ${selectedHistoryIds.value.size} 条通话记录吗？此操作不可恢复。`,
    '删除确认',
    { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
  ).then(async () => {
    const ids = Array.from(selectedHistoryIds.value)
    await Promise.all(
      ids.map(id => api.delete(`/devices/${props.deviceId}/voice/history/${id}`).catch(() => {}))
    )
    selectedHistoryIds.value.clear()
    selectedHistoryIds.value = new Set(selectedHistoryIds.value)
    historyEditMode.value = false
  }).catch(() => {})
}

// --- 联系人 ---
interface Contact {
  id: string
  name: string
  number: string
  createdAt: number
}

const contacts = ref<Contact[]>([])
const contactsSubView = ref<'list' | 'add'>('list')
const CONTACTS_KEY_PREFIX = 'voice_contacts_'

function contactsKey() {
  return `${CONTACTS_KEY_PREFIX}${props.deviceId}`
}

function loadContacts() {
  try {
    const raw = localStorage.getItem(contactsKey())
    if (raw) contacts.value = JSON.parse(raw) as Contact[]
    else contacts.value = []
  } catch {
    contacts.value = []
  }
}

function saveContacts(): boolean {
  try {
    localStorage.setItem(contactsKey(), JSON.stringify(contacts.value))
    return true
  } catch {
    return false
  }
}

function onAddContact(contact: { name: string; number: string }) {
  const newContact: Contact = {
    id: Date.now().toString(36) + Math.random().toString(36).slice(2, 6),
    name: contact.name,
    number: contact.number,
    createdAt: Date.now(),
  }
  contacts.value = [newContact, ...contacts.value]
  if (saveContacts()) {
    ElMessage.success('联系人已保存')
    contactsSubView.value = 'list'
  } else {
    ElMessage.error('保存失败，请重试')
  }
}

function onCancelAddContact() {
  contactsSubView.value = 'list'
}

function onDeleteContact(id: string) {
  ElMessageBox.confirm('确定要删除此联系人吗？', '删除确认', {
    confirmButtonText: '删除',
    cancelButtonText: '取消',
    type: 'warning',
  }).then(() => {
    contacts.value = contacts.value.filter(c => c.id !== id)
    if (saveContacts()) {
      ElMessage.success('已删除')
    } else {
      ElMessage.error('删除失败，请重试')
    }
  }).catch(() => {})
}

function onContactCall(number: string) {
  if (callState.value !== 'idle') {
    ElMessage.warning('当前正在通话中')
    return
  }
  activeView.value = 'dialer'
  dialerRef.value?.setNumber(number)
}

onMounted(() => {
  loadContacts()
})

onUnmounted(() => {
  unsubVoice()
  stopTimer()
  voiceService.disconnect()
})
</script>

<template>
  <div class="voice-tab">
    <!-- 头部 -->
    <div class="terminal-card-header">
      <div class="terminal-icon-box">
        <el-icon size="14"><Call24Regular /></el-icon>
      </div>
      <div class="terminal-header-title">
        <span>通话</span>
        <span class="title-separator">›</span>
        <span class="title-sub">{{ viewTitle }}</span>
      </div>
      <!-- 当前通话方式（只读，后端自动路由） -->
      <el-tag
        v-if="activeView === 'dialer'"
        :type="callMethodTag"
        size="small"
        effect="plain"
        class="call-method-tag"
      >
        {{ callMethodLabel }}
      </el-tag>
      <!-- 编辑按钮：设置视图 -->
      <button
        v-if="activeView === 'settings'"
        class="header-action-btn"
        :class="{ danger: settingsEditMode }"
        @click="onSettingsToggleEdit"
      >
        {{ settingsEditMode ? '保存' : '编辑' }}
      </button>
      <!-- 编辑按钮：仅通话记录视图 -->
      <button
        v-if="activeView === 'history'"
        class="header-action-btn"
        @click="historyEditMode = !historyEditMode"
      >
        {{ historyEditMode ? '取消' : '编辑' }}
      </button>
      <!-- 删除按钮：编辑模式且有选中时显示 -->
      <button
        v-if="activeView === 'history' && historyEditMode && selectedHistoryIds.size > 0"
        class="header-action-btn danger"
        @click="deleteSelectedHistory"
      >
        删除 ({{ selectedHistoryIds.size }})
      </button>
      <!-- 添加按钮：仅通讯录列表视图 -->
      <button
        v-if="activeView === 'contacts' && contactsSubView === 'list'"
        class="header-action-btn"
        @click="contactsSubView = 'add'"
      >
        添加
      </button>
    </div>

    <!-- 内容区 -->
    <div class="content-area">
      <!-- 通话状态覆盖层 -->
      <ModuleVoiceStatus
        v-if="inCall"
        :call-state="callState"
        :call-number="callNumber"
        :call-timer="callTimer"
        :show-dtmf="showDtmf"
        @answer="onAnswer"
        @hangup="onHangup"
        @toggle-dtmf="showDtmf = !showDtmf"
        @dtmf-key="onDtmfKey"
      />

      <!-- 拨号盘视图 -->
      <div v-show="activeView === 'dialer'" class="view dialer-view">
        <ModuleVoiceDialer
          ref="dialerRef"
          :disabled="inCall"
          @dial="onDial"
        />
      </div>

      <!-- 通话记录视图 -->
      <div v-show="activeView === 'history'" class="view">
        <ModuleVoiceHistory
          :device-id="props.deviceId"
          :edit-mode="historyEditMode"
          :selected-ids="selectedHistoryIds"
          @callback="onCallback"
          @toggle-select="toggleHistorySelect"
        />
      </div>

      <!-- 通讯录视图 -->
      <div v-show="activeView === 'contacts'" class="view">
        <!-- 列表子视图 -->
        <ModuleVoiceContacts
          v-if="contactsSubView === 'list'"
          :contacts="contacts"
          @call="onContactCall"
          @delete="onDeleteContact"
          @add="contactsSubView = 'add'"
        />
        <!-- 添加子视图 -->
        <ModuleVoiceContactAdd
          v-else
          :existing-numbers="contacts.map(c => c.number)"
          @save="onAddContact"
          @cancel="onCancelAddContact"
        />
      </div>

      <!-- 设置视图 -->
      <div v-show="activeView === 'settings'" class="view">
        <ModuleVoiceSettings ref="settingsRef" />
      </div>

      <!-- 悬浮导航（自定义 radio） -->
      <div class="nav-floating">
        <div class="di-radio-wrap">
          <input type="radio" name="voice-nav" id="nav-dialer" class="di-radio-input" :checked="activeView === 'dialer'" @change="activeView = 'dialer'" />
          <input type="radio" name="voice-nav" id="nav-history" class="di-radio-input" :checked="activeView === 'history'" @change="activeView = 'history'" />
          <input type="radio" name="voice-nav" id="nav-contacts" class="di-radio-input" :checked="activeView === 'contacts'" @change="activeView = 'contacts'" />
          <input type="radio" name="voice-nav" id="nav-settings" class="di-radio-input" :checked="activeView === 'settings'" @change="activeView = 'settings'" />
          <div class="di-radio-island">
            <label for="nav-dialer" class="di-radio-btn">
              <el-icon size="28" class="di-radio-icon"><Dialpad24Regular /></el-icon>
              <span>拨号盘</span>
            </label>
            <label for="nav-history" class="di-radio-btn">
              <el-icon size="28" class="di-radio-icon"><History24Regular /></el-icon>
              <span>通话记录</span>
            </label>
            <label for="nav-contacts" class="di-radio-btn">
              <el-icon size="28" class="di-radio-icon"><BookContacts24Regular /></el-icon>
              <span>通讯录</span>
            </label>
            <label for="nav-settings" class="di-radio-btn">
              <el-icon size="28" class="di-radio-icon"><Settings24Regular /></el-icon>
              <span>设置</span>
            </label>
            <div class="di-radio-indicator" :class="`pos-${activeView}`"></div>
          </div>
        </div>
      </div>
    </div>

  </div>
</template>

<style scoped>
@import '../assets/radio-buttons/kind-lizard-11.css';

.voice-tab {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--background);
  overflow: hidden;
}

/* 头部 */
.terminal-card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.terminal-icon-box {
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

.terminal-header-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
  flex: 1;
  display: flex;
  align-items: center;
  gap: 4px;
}

.title-separator {
  color: var(--muted-foreground);
  font-weight: 400;
}

.title-sub {
  color: var(--muted-foreground);
  font-weight: 600;
}

/* 内容区 */
.content-area {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  position: relative;
}

/* 视图 */
.view {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
  padding-bottom: 56px;
}

.dialer-view {
  align-items: stretch;
  justify-content: flex-start;
}

/* 通话方式标签（顶栏右侧） */
.terminal-card-header .call-method-tag {
  flex-shrink: 0;
}

/* 顶栏操作按钮 */
.header-action-btn {
  padding: 4px 12px;
  font-size: 12px;
  font-weight: 600;
  color: var(--foreground);
  background: var(--muted);
  border: 1px solid var(--border);
  border-radius: 4px;
  cursor: pointer;
  transition: background 0.15s;
  flex-shrink: 0;
}

.header-action-btn:hover {
  background: var(--accent);
}

.header-action-btn.danger {
  color: #ef4444;
  border-color: #ef4444;
}

.header-action-btn.danger:hover {
  background: #ef4444;
  color: #fff;
}

/* 悬浮导航 */
.nav-floating {
  position: absolute;
  bottom: 12px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 15;
}

/* 适配 kind-lizard-11 radio 组件 */
.di-radio-wrap {
  width: auto;
  margin: 0;
}

.di-radio-island {
  background: color-mix(in oklab, var(--card) 92%, transparent);
  border: 1px solid var(--border);
  box-shadow: var(--console-shadow-md);
  backdrop-filter: blur(8px);
}

.di-radio-btn {
  color: var(--muted-foreground);
  font-size: 11px;
  font-weight: 600;
  width: 68px;
  height: 48px;
  padding: 6px;
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  white-space: nowrap;
  line-height: 1;
  text-align: center;
  box-sizing: border-box;
}

.di-radio-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.di-radio-indicator {
  background: var(--accent);
  width: 68px;
  height: calc(100% - 12px);
}

/* 选中项文字高亮 */
.di-radio-input:checked ~ .di-radio-island .di-radio-btn {
  color: var(--foreground);
}

/* 指示器位置 */
.di-radio-indicator.pos-dialer { transform: translateX(0); }
.di-radio-indicator.pos-history { transform: translateX(72px); }
.di-radio-indicator.pos-contacts { transform: translateX(144px); }
.di-radio-indicator.pos-settings { transform: translateX(216px); }

/* 原始 CSS 中的选中高亮适配 */
#nav-dialer:checked ~ .di-radio-island label[for="nav-dialer"],
#nav-history:checked ~ .di-radio-island label[for="nav-history"],
#nav-contacts:checked ~ .di-radio-island label[for="nav-contacts"],
#nav-settings:checked ~ .di-radio-island label[for="nav-settings"] {
  color: var(--foreground);
  transform: translateY(-1px);
}
</style>
