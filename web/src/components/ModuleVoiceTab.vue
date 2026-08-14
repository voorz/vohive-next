<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import ModuleVoiceDialer from './ModuleVoiceDialer.vue'
import ModuleVoiceHistory from './ModuleVoiceHistory.vue'
import ModuleVoiceStatus from './ModuleVoiceStatus.vue'
import ModuleVoiceContacts from './ModuleVoiceContacts.vue'
import ModuleVoiceContactAddDialog from './ModuleVoiceContactAddDialog.vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Call24Regular,
  Dialpad24Regular,
  History24Regular,
  BookContacts24Regular,
} from '@vicons/fluent'

const props = defineProps<{
  deviceId: string
}>()

// 视图切换
const activeView = ref<'dialer' | 'history' | 'contacts'>('dialer')

// --- 呼叫方式 ---
type CallMode = 'vowifi' | 'cs'
const callMode = ref<CallMode>('vowifi')
const callModes = [
  { value: 'vowifi' as const, label: 'VoWiFi 优先（推荐）' },
  { value: 'cs' as const, label: 'CS 域蜂窝' },
]

const MODE_KEY_PREFIX = 'voice_call_mode_'
function modeKey() {
  return `${MODE_KEY_PREFIX}${props.deviceId}`
}

function loadMode() {
  try {
    const saved = localStorage.getItem(modeKey())
    if (saved) callMode.value = saved as CallMode
  } catch { /* ignore */ }
}

function saveMode() {
  try {
    localStorage.setItem(modeKey(), callMode.value)
  } catch { /* ignore */ }
}

watch(() => props.deviceId, () => {
  loadMode()
}, { immediate: true })

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
function onDial(number: string) {
  if (callState.value !== 'idle') return
  callNumber.value = number
  callState.value = 'dialing'
  // TODO: 接入后端 API 发起呼叫
  setTimeout(() => {
    if (callState.value === 'dialing') {
      callState.value = 'connected'
      startTimer()
    }
  }, 2000)
}

// 挂断
function onHangup() {
  showDtmf.value = false
  if (callState.value === 'idle') return
  callState.value = 'hanging'
  stopTimer()
  // TODO: 接入后端 API 挂断
  setTimeout(() => {
    callState.value = 'idle'
    callNumber.value = ''
    callTimer.value = 0
    dialerRef.value?.clear()
  }, 800)
}

// 接答
function onAnswer() {
  if (callState.value !== 'ringing') return
  callState.value = 'connected'
  startTimer()
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
  ).then(() => {
    // TODO: 接入后端 API 删除记录
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
const showAddContact = ref(false)
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
  } else {
    ElMessage.error('保存失败，请重试')
  }
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

const existingNumbers = computed(() => contacts.value.map(c => c.number))

onMounted(() => {
  loadContacts()
})
</script>

<template>
  <div class="voice-tab">
    <!-- 头部 -->
    <div class="terminal-card-header">
      <div class="terminal-icon-box">
        <el-icon size="14"><Call24Regular /></el-icon>
      </div>
      <div class="terminal-header-title">通话</div>
      <!-- 呼叫方式：仅在拨号盘视图显示 -->
      <el-select
        v-if="activeView === 'dialer'"
        v-model="callMode"
        size="small"
        class="call-mode-select"
        @change="saveMode"
      >
        <el-option
          v-for="m in callModes"
          :key="m.value"
          :label="m.label"
          :value="m.value"
        />
      </el-select>
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
      <!-- 添加按钮：仅通讯录视图 -->
      <button
        v-if="activeView === 'contacts'"
        class="header-action-btn"
        @click="showAddContact = true"
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
          :edit-mode="historyEditMode"
          :selected-ids="selectedHistoryIds"
          @callback="onCallback"
          @toggle-select="toggleHistorySelect"
        />
      </div>

      <!-- 通讯录视图 -->
      <div v-show="activeView === 'contacts'" class="view">
        <ModuleVoiceContacts
          :contacts="contacts"
          @call="onContactCall"
          @delete="onDeleteContact"
          @add="showAddContact = true"
        />
      </div>

      <!-- 悬浮导航（自定义 radio） -->
      <div class="nav-floating">
        <div class="di-radio-wrap">
          <input type="radio" name="voice-nav" id="nav-dialer" class="di-radio-input" :checked="activeView === 'dialer'" @change="activeView = 'dialer'" />
          <input type="radio" name="voice-nav" id="nav-history" class="di-radio-input" :checked="activeView === 'history'" @change="activeView = 'history'" />
          <input type="radio" name="voice-nav" id="nav-contacts" class="di-radio-input" :checked="activeView === 'contacts'" @change="activeView = 'contacts'" />
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
            <div class="di-radio-indicator" :class="`pos-${activeView}`"></div>
          </div>
        </div>
      </div>
    </div>

    <!-- 添加联系人对话框 -->
    <ModuleVoiceContactAddDialog
      v-model="showAddContact"
      :existing-numbers="existingNumbers"
      @save="onAddContact"
    />
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

/* 呼叫方式下拉框（顶栏右侧） */
.terminal-card-header .call-mode-select {
  width: 180px;
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

/* 原始 CSS 中的选中高亮适配 */
#nav-dialer:checked ~ .di-radio-island label[for="nav-dialer"],
#nav-history:checked ~ .di-radio-island label[for="nav-history"],
#nav-contacts:checked ~ .di-radio-island label[for="nav-contacts"] {
  color: var(--foreground);
  transform: translateY(-1px);
}
</style>
