<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { smsService } from '../services/sms'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Chat24Regular, Add24Regular, Send24Regular, ArrowLeft24Regular, EyeOff24Regular, Eye24Regular, CheckmarkCircle24Regular, ErrorCircle24Regular, Clock24Regular, Delete24Regular, Edit24Regular } from '@vicons/fluent'
import type { SMSMessage } from '../types/api'
import type { SmsThreadVM } from '../types/view-model'

const props = defineProps<{
  deviceId: string
}>()

// 视图状态：list | detail | new
const view = ref<'list' | 'detail' | 'new'>('list')

// 脱敏开关（默认开启，持久化）
const MASK_KEY = 'sms_mask_enabled'
const maskEnabled = ref(true)
try {
  const saved = localStorage.getItem(MASK_KEY)
  if (saved !== null) maskEnabled.value = saved === '1'
} catch { /* ignore */ }

function toggleMask() {
  maskEnabled.value = !maskEnabled.value
  try { localStorage.setItem(MASK_KEY, maskEnabled.value ? '1' : '0') } catch { /* ignore */ }
}

// 脱敏改为 CSS blur，不再替换字符

// 编辑模式（批量删除消息）
const editMode = ref(false)
const selectedMsgIds = ref<Set<number>>(new Set())

function toggleEditMode() {
  editMode.value = !editMode.value
  if (!editMode.value) selectedMsgIds.value.clear()
}

function toggleMsgSelect(id: number) {
  if (selectedMsgIds.value.has(id)) selectedMsgIds.value.delete(id)
  else selectedMsgIds.value.add(id)
  selectedMsgIds.value = new Set(selectedMsgIds.value)
}

async function handleDeleteThread(peer: string, e: Event) {
  e.stopPropagation()
  try {
    await ElMessageBox.confirm(
      `确定要删除与 ${peer} 的全部消息记录吗？此操作不可恢复。`,
      '删除会话',
      { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
    )
  } catch { return }
  try {
    const result = await smsService.deleteThread({ device_id: props.deviceId, peer })
    if (!result.ok) throw new Error(result.error.message || '删除失败')
    ElMessage.success('会话已删除')
    await fetchThreads()
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : '删除失败')
  }
}

async function handleBatchDelete() {
  if (selectedMsgIds.value.size === 0) return
  const count = selectedMsgIds.value.size
  try {
    await ElMessageBox.confirm(
      `确定要删除选中的 ${count} 条消息吗？`,
      '批量删除',
      { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
    )
  } catch { return }
  const ids = [...selectedMsgIds.value]
  let ok = 0
  let fail = 0
  for (const id of ids) {
    try {
      const r = await smsService.deleteMessage(id)
      if (r.ok) ok++
      else fail++
    } catch { fail++ }
  }
  if (fail === 0) ElMessage.success(`已删除 ${ok} 条消息`)
  else ElMessage.warning(`删除 ${ok} 条成功，${fail} 条失败`)
  selectedMsgIds.value.clear()
  editMode.value = false
  await fetchMessages()
  await fetchThreads()
}

// 消息列表（带日期分割）
const messagesWithSeparators = computed(() => {
  const result: Array<{ type: 'sep'; date: string; key: string } | { type: 'msg'; msg: SMSMessage; key: string }> = []
  let lastDate = ''
  for (const msg of messages.value) {
    const ts = parseTs(msg.timestamp)
    const d = new Date(ts)
    const dateStr = `${d.getFullYear()}.${String(d.getMonth() + 1).padStart(2, '0')}.${String(d.getDate()).padStart(2, '0')}`
    if (dateStr !== lastDate) {
      result.push({ type: 'sep', date: dateStr, key: `sep-${dateStr}` })
      lastDate = dateStr
    }
    result.push({ type: 'msg', msg, key: `msg-${msg.id}` })
  }
  return result
})

// 发送状态追踪（持久化）
const SEND_STATUS_KEY = 'sms_send_status'
const sendStatusMap = ref<Record<string, 'sending' | 'sent' | 'failed'>>({})
try {
  const raw = localStorage.getItem(SEND_STATUS_KEY)
  if (raw) sendStatusMap.value = JSON.parse(raw)
} catch { /* ignore */ }

function saveSendStatus() {
  try {
    const entries = Object.entries(sendStatusMap.value)
    if (entries.length > 50) {
      sendStatusMap.value = Object.fromEntries(entries.slice(-50))
    }
    localStorage.setItem(SEND_STATUS_KEY, JSON.stringify(sendStatusMap.value))
  } catch { /* ignore */ }
}

function sendStatusKey(peer: string, ts: string): string {
  return `${props.deviceId}:${peer}:${ts}`
}

function getSendStatus(msg: SMSMessage): 'sending' | 'sent' | 'failed' | null {
  if (msg.type !== 2) return null
  if (msg.status === 2) return 'sent'
  if (msg.status === 3) return 'failed'
  const key = sendStatusKey(msg.sender || '', msg.timestamp)
  return sendStatusMap.value[key] || null
}

// 未读追踪（持久化）
const SEEN_KEY = 'sms_last_seen'
const seenMap = ref<Record<string, number>>({})
try {
  const raw = localStorage.getItem(SEEN_KEY)
  if (raw) seenMap.value = JSON.parse(raw)
} catch { /* ignore */ }

function saveSeen() {
  try { localStorage.setItem(SEEN_KEY, JSON.stringify(seenMap.value)) } catch { /* ignore */ }
}

function seenKey(peer: string): string {
  return `${props.deviceId}:${peer}`
}

function getLastSeen(peer: string): number {
  return seenMap.value[seenKey(peer)] || 0
}

function markSeen(peer: string) {
  seenMap.value[seenKey(peer)] = Date.now()
  saveSeen()
}

function isUnread(thread: SmsThreadVM): boolean {
  return thread.lastTs > getLastSeen(thread.peer)
}

function unreadCount(): number {
  return threads.value.filter(isUnread).length
}

// 会话列表
const threads = ref<SmsThreadVM[]>([])
const loadingThreads = ref(false)

// 当前选中
const selectedPeer = ref('')
const newPeer = ref('')

// 消息列表
const messages = ref<SMSMessage[]>([])
const loadingMessages = ref(false)

// 发送
const composer = ref('')
const sending = ref(false)

// 滚动容器
const scrollRef = ref<HTMLElement | null>(null)

async function fetchThreads() {
  if (!props.deviceId) return
  loadingThreads.value = true
  try {
    const result = await smsService.listContacts(props.deviceId)
    if (result.ok) {
      threads.value = result.data
    }
  } catch {
    // ignore
  } finally {
    loadingThreads.value = false
  }
}

async function fetchMessages() {
  if (!props.deviceId || !selectedPeer.value) {
    messages.value = []
    return
  }
  loadingMessages.value = true
  try {
    const result = await smsService.getThread({
      device_id: props.deviceId,
      peer: selectedPeer.value,
      limit: 100
    })
    if (result.ok) {
      messages.value = result.data
      await nextTick()
      scrollToBottom()
    }
  } catch {
    // ignore
  } finally {
    loadingMessages.value = false
  }
}

function scrollToBottom() {
  if (scrollRef.value) {
    scrollRef.value.scrollTop = scrollRef.value.scrollHeight
  }
}

function parseTs(s: string) {
  const ms = new Date(s).getTime()
  return Number.isFinite(ms) ? ms : 0
}

function formatDate(ms: number) {
  if (!ms) return ''
  const d = new Date(ms)
  return `${d.getFullYear()}/${String(d.getMonth() + 1).padStart(2, '0')}/${String(d.getDate()).padStart(2, '0')}`
}

function formatTime(ms: number) {
  if (!ms) return ''
  const d = new Date(ms)
  const now = new Date()
  if (d.toDateString() === now.toDateString()) {
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
  return `${d.getMonth() + 1}/${d.getDate()} ${d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`
}

function enterDetail(peer: string) {
  selectedPeer.value = peer
  view.value = 'detail'
  markSeen(peer)
  void fetchMessages()
}

function enterNew() {
  newPeer.value = ''
  composer.value = ''
  messages.value = []
  view.value = 'new'
}

// 输入时自动格式化联系人号码（去除空格）
function onNewPeerInput(e: Event) {
  const input = e.target as HTMLInputElement
  newPeer.value = normalizePhone(input.value)
}

function backToList() {
  view.value = 'list'
  selectedPeer.value = ''
  newPeer.value = ''
  composer.value = ''
  messages.value = []
  editMode.value = false
  selectedMsgIds.value.clear()
  void fetchThreads()
}

function normalizePhone(phone: string): string {
  // 去除所有空格、连字符、括号
  return phone.replace(/[\s\-()]/g, '')
}

async function handleSend() {
  const text = composer.value.trim()
  if (!text || sending.value) return

  const rawPeer = view.value === 'new' ? newPeer.value.trim() : selectedPeer.value
  if (!rawPeer) {
    ElMessage.warning('请输入联系人号码')
    return
  }

  // 格式化号码：去除空格等无效字符
  const peer = normalizePhone(rawPeer)
  if (!peer) {
    ElMessage.warning('联系人号码格式无效')
    return
  }

  sending.value = true
  const sendTs = new Date().toISOString()
  const statusKey = sendStatusKey(peer, sendTs)
  sendStatusMap.value[statusKey] = 'sending'
  saveSendStatus()
  try {
    const result = await smsService.send({
      device_id: props.deviceId,
      phone: peer,
      message: text
    })
    if (!result.ok) throw new Error(result.error.message || '发送失败')
    sendStatusMap.value[statusKey] = 'sent'
    saveSendStatus()
    ElMessage.success('短信已发送')
    composer.value = ''

    if (view.value === 'new') {
      // 新建模式发送后切换到详情模式
      newPeer.value = peer
      selectedPeer.value = peer
      view.value = 'detail'
    }
    markSeen(peer)
    await fetchMessages()
    await fetchThreads()
  } catch (e: unknown) {
    sendStatusMap.value[statusKey] = 'failed'
    saveSendStatus()
    ElMessage.error(e instanceof Error ? e.message : '发送失败')
  } finally {
    sending.value = false
  }
}

function handleKeydown(e: Event) {
  const ev = e as KeyboardEvent
  if (ev.key === 'Enter' && !ev.shiftKey) {
    ev.preventDefault()
    void handleSend()
  }
}

watch(() => props.deviceId, () => {
  view.value = 'list'
  selectedPeer.value = ''
  newPeer.value = ''
  threads.value = []
  messages.value = []
  composer.value = ''
  void fetchThreads()
})

onMounted(() => {
  void fetchThreads()
})
</script>

<template>
  <div class="module-sms-tab">
    <!-- 头部 -->
    <div class="terminal-card-header">
      <div v-if="view !== 'list'" class="sms-back-btn" @click="backToList">
        <el-icon size="14"><ArrowLeft24Regular /></el-icon>
      </div>
      <div v-else class="terminal-icon-box">
        <el-icon size="14"><Chat24Regular /></el-icon>
      </div>
      <div class="terminal-header-title">信息</div>
      <div v-if="unreadCount() > 0" class="sms-unread-badge">{{ unreadCount() }}</div>
      <div class="sms-header-actions">
        <button class="sms-icon-btn" @click="toggleMask" :title="maskEnabled ? '点击显示原始号码' : '点击脱敏号码'">
          <el-icon size="14">
            <EyeOff24Regular v-if="maskEnabled" />
            <Eye24Regular v-else />
          </el-icon>
        </button>
        <button class="sms-new-btn" @click="enterNew">
          <el-icon size="14"><Add24Regular /></el-icon>
          <span>新建</span>
        </button>
      </div>
    </div>

    <!-- 消息列表模式 -->
    <div v-if="view === 'list'" class="sms-list">
      <div v-if="loadingThreads && threads.length === 0" class="sms-empty">
        加载中...
      </div>
      <div v-else-if="threads.length === 0" class="sms-empty">
        暂无消息
      </div>
      <template v-else>
        <div
          v-for="t in threads"
          :key="t.key"
          class="sms-list-item"
          @click="enterDetail(t.peer)"
        >
          <div class="sms-list-item-top">
            <div class="sms-list-item-left">
              <span class="sms-list-item-peer" :class="{ 'sms-blur-text': maskEnabled }">{{ t.peer }}</span>
              <span v-if="isUnread(t)" class="sms-unread-dot" />
            </div>
            <span class="sms-list-item-date" :class="{ 'sms-unread-date': isUnread(t) }">{{ formatDate(t.lastTs) }}</span>
          </div>
          <div class="sms-list-item-preview" :class="{ 'sms-unread-preview': isUnread(t) }">{{ t.lastMessage }}</div>
          <button class="sms-list-item-delete" @click="handleDeleteThread(t.peer, $event)" title="删除会话">
            <el-icon size="13"><Delete24Regular /></el-icon>
          </button>
        </div>
      </template>
    </div>

    <!-- 消息内页 / 新建页面 -->
    <template v-else>
      <!-- 联系人栏 -->
      <div class="sms-contact-bar">
        <template v-if="view === 'detail'">
          <span class="sms-contact-label">联系人</span>
          <span class="sms-contact-phone" :class="{ 'sms-blur-text': maskEnabled }">{{ selectedPeer }}</span>
          <div class="sms-contact-bar-actions">
            <template v-if="editMode">
              <button class="sms-edit-btn cancel" @click="toggleEditMode">取消</button>
              <button class="sms-edit-btn danger" :disabled="selectedMsgIds.size === 0" @click="handleBatchDelete">
                删除{{ selectedMsgIds.size > 0 ? `(${selectedMsgIds.size})` : '' }}
              </button>
            </template>
            <template v-else>
              <button class="sms-icon-btn" @click="toggleEditMode" title="编辑消息">
                <el-icon size="13"><Edit24Regular /></el-icon>
              </button>
            </template>
          </div>
        </template>
        <template v-else>
          <span class="sms-contact-label">联系人</span>
          <input
            v-model="newPeer"
            class="sms-contact-input"
            placeholder="输入联系人号码（含国家码，如 +8613800138000）"
            @input="onNewPeerInput"
          />
        </template>
      </div>

      <!-- 气泡区 -->
      <div ref="scrollRef" class="sms-bubbles">
        <div v-if="loadingMessages && messages.length === 0" class="sms-empty">
          加载中...
        </div>
        <div v-else-if="messages.length === 0" class="sms-empty">
          暂无消息
        </div>
        <template v-else>
          <template v-for="item in messagesWithSeparators" :key="item.key">
            <div v-if="item.type === 'sep'" class="sms-date-sep">
              <span class="sms-date-sep-line"></span>
              <span class="sms-date-sep-text">{{ item.date }}</span>
              <span class="sms-date-sep-line"></span>
            </div>
            <div
              v-else
              class="sms-bubble-row"
              :class="{ sent: item.msg.type === 2, received: item.msg.type === 1, editing: editMode }"
              @click="editMode && toggleMsgSelect(item.msg.id)"
            >
              <div class="sms-checkbox-wrap" :class="{ visible: editMode }" @click.stop>
                <el-checkbox :model-value="selectedMsgIds.has(item.msg.id)" @change="toggleMsgSelect(item.msg.id)" size="small" />
              </div>
              <div v-if="item.msg.type === 2 && getSendStatus(item.msg)" class="sms-bubble-status">
                <el-icon v-if="getSendStatus(item.msg) === 'sent'" size="16" class="sms-status-sent"><CheckmarkCircle24Regular /></el-icon>
                <el-icon v-else-if="getSendStatus(item.msg) === 'failed'" size="16" class="sms-status-failed"><ErrorCircle24Regular /></el-icon>
                <el-icon v-else-if="getSendStatus(item.msg) === 'sending'" size="16" class="sms-status-sending"><Clock24Regular /></el-icon>
              </div>
              <div class="sms-bubble" :class="{ 'sms-bubble-selected': editMode && selectedMsgIds.has(item.msg.id) }">
                <div class="sms-bubble-text">{{ item.msg.content }}</div>
                <div class="sms-bubble-meta">
                  <span class="sms-bubble-time">{{ formatTime(parseTs(item.msg.timestamp)) }}</span>
                </div>
              </div>
            </div>
          </template>
        </template>
      </div>

      <!-- 输入栏 -->
      <div class="sms-input-bar">
        <input
          v-model="composer"
          class="sms-input-field"
          placeholder="输入消息"
          @keydown="handleKeydown"
          :disabled="sending"
        />
        <button
          class="sms-send-btn"
          :disabled="!composer.trim() || sending || (view === 'new' && !newPeer.trim())"
          @click="handleSend"
        >
          <span>发送</span>
        </button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.module-sms-tab {
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

.sms-back-btn {
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
  cursor: pointer;
  transition: all 0.15s;
}

.sms-back-btn:hover {
  background: var(--accent);
}

.terminal-header-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
  flex: 1;
}

.sms-header-actions {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.sms-icon-btn {
  width: 28px;
  height: 28px;
  border-radius: 5px;
  border: 1px solid var(--border);
  background: var(--background);
  color: var(--foreground);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: all 0.15s;
}

.sms-icon-btn:hover {
  background: var(--accent);
}

.sms-new-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  font-size: 12px;
  font-weight: 600;
  border-radius: 4px;
  border: 1px solid var(--border);
  background: var(--background);
  color: var(--foreground);
  cursor: pointer;
  transition: all 0.15s;
}

.sms-new-btn:hover {
  background: var(--brand);
  border-color: var(--brand);
  color: #fff;
}

/* 消息列表 */
.sms-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.sms-list-item {
  position: relative;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
  cursor: pointer;
  transition: background 0.12s;
}

.sms-list-item:hover {
  background: var(--accent);
}

.sms-list-item-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.sms-list-item-left {
  display: flex;
  align-items: center;
  gap: 6px;
  overflow: hidden;
  min-width: 0;
}

.sms-unread-dot {
  width: 7px;
  height: 7px;
  border-radius: 999px;
  background: var(--brand);
  flex-shrink: 0;
}

.sms-unread-date {
  font-weight: 700;
  color: var(--brand);
}

.sms-unread-preview {
  font-weight: 600;
  color: var(--foreground);
}

.sms-unread-badge {
  min-width: 18px;
  height: 18px;
  border-radius: 999px;
  background: var(--brand);
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 5px;
  flex-shrink: 0;
}

.sms-list-item-peer {
  font-size: 13px;
  font-weight: 700;
  color: var(--foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sms-list-item-date {
  font-size: 11px;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
  flex-shrink: 0;
}

.sms-list-item-preview {
  font-size: 12px;
  color: var(--muted-foreground);
  margin-top: 3px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 列表项删除按钮 */
.sms-list-item-delete {
  position: absolute;
  right: 8px;
  bottom: 6px;
  width: 24px;
  height: 24px;
  border-radius: 5px;
  border: 1px solid var(--border);
  background: var(--background);
  color: var(--muted-foreground);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: all 0.15s;
}

.sms-list-item:hover .sms-list-item-delete {
  opacity: 1;
}

.sms-list-item-delete:hover {
  background: #ff6b6b;
  border-color: #ff6b6b;
  color: #fff;
}

/* 联系人栏 */
.sms-contact-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  border-bottom: 1px solid var(--border);
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
  flex-shrink: 0;
}

.sms-contact-label {
  flex-shrink: 0;
}

.sms-contact-phone {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sms-contact-bar-actions {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.sms-edit-btn {
  padding: 3px 10px;
  font-size: 12px;
  font-weight: 600;
  border-radius: 4px;
  border: 1px solid var(--border);
  background: var(--background);
  color: var(--foreground);
  cursor: pointer;
  white-space: nowrap;
  transition: all 0.15s;
}

.sms-edit-btn.cancel:hover {
  background: var(--accent);
}

.sms-edit-btn.danger {
  color: #ff6b6b;
  border-color: #ff6b6b;
}

.sms-edit-btn.danger:not(:disabled):hover {
  background: #ff6b6b;
  color: #fff;
}

.sms-edit-btn.danger:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

/* CSS blur 脱敏 */
.sms-blur-text {
  filter: blur(4px);
  user-select: none;
  cursor: pointer;
  transition: filter 0.15s;
}

.sms-blur-text:hover {
  filter: blur(0);
}

.sms-contact-input {
  flex: 1;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--background);
  color: var(--foreground);
  padding: 4px 8px;
  font-size: 13px;
  outline: none;
}

.sms-contact-input:focus {
  border-color: var(--brand);
}

/* 气泡区 */
.sms-bubbles {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 12px 14px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.sms-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--muted-foreground);
  font-size: 12px;
}

/* 气泡行 */
.sms-bubble-row {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 6px;
}

.sms-bubble-row.received {
  justify-content: flex-start;
}

.sms-bubble-row.sent {
  justify-content: flex-end;
}

.sms-bubble-row.editing {
  cursor: pointer;
}

/* 状态图标（发送消息） */
.sms-bubble-status {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  order: 1;
}

.sms-checkbox-wrap {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  width: 0;
  opacity: 0;
  overflow: hidden;
  transition: width 0.2s ease, opacity 0.2s ease;
}

.sms-checkbox-wrap.visible {
  width: 20px;
  opacity: 1;
}

/* 接收消息：复选框在左侧 */
.sms-bubble-row.received .sms-checkbox-wrap { order: 1; }
.sms-bubble-row.received .sms-bubble { order: 2; }

/* 发送消息：状态图标在左，气泡居中，复选框在右 */
.sms-bubble-row.sent .sms-bubble { order: 2; }
.sms-bubble-row.sent .sms-checkbox-wrap { order: 3; }

/* 日期分割 */
.sms-date-sep {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 0;
  flex-shrink: 0;
}

.sms-date-sep-line {
  flex: 1;
  height: 1px;
  background: var(--border);
}

.sms-date-sep-text {
  font-size: 11px;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
  white-space: nowrap;
  flex-shrink: 0;
}

.sms-bubble-selected {
  outline: 2px solid var(--brand);
  outline-offset: 1px;
}

/* 气泡 */
.sms-bubble {
  max-width: 78%;
  padding: 6px 10px;
  border-radius: 10px;
  font-size: 13px;
  line-height: 1.4;
  word-break: break-word;
}

.sms-bubble-text {
  color: inherit;
}

.sms-bubble-meta {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 2px;
}

.sms-bubble-time {
  font-size: 10px;
  opacity: 0.6;
  font-family: var(--oomol-font-mono);
}

.sms-status-sent {
  color: var(--muted-foreground);
}

.sms-status-failed {
  color: #ff6b6b;
}

.sms-status-sending {
  color: var(--muted-foreground);
  animation: spin 1.5s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* 接收气泡：左下角直角 */
.sms-bubble-row.received .sms-bubble {
  background: var(--muted);
  color: var(--foreground);
  border-bottom-left-radius: 2px;
}

/* 发送气泡：右下角直角 */
.sms-bubble-row.sent .sms-bubble {
  background: var(--brand);
  color: #fff;
  border-bottom-right-radius: 2px;
}

/* 输入栏 */
.sms-input-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  border-top: 1px solid var(--border);
  flex-shrink: 0;
}

.sms-input-field {
  flex: 1;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--background);
  color: var(--foreground);
  padding: 5px 10px;
  font-size: 13px;
  outline: none;
}

.sms-input-field:focus {
  border-color: var(--brand);
}

.sms-input-field:disabled {
  opacity: 0.55;
}

.sms-send-btn {
  padding: 5px 16px;
  font-size: 12px;
  font-weight: 600;
  border-radius: 4px;
  border: 1px solid var(--border);
  background: var(--background);
  color: var(--foreground);
  cursor: pointer;
  white-space: nowrap;
  transition: all 0.15s;
}

.sms-send-btn:not(:disabled):hover {
  background: var(--brand);
  border-color: var(--brand);
  color: #fff;
}

.sms-send-btn:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}
</style>
