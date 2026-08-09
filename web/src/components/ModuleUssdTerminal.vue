<script setup lang="ts">
import { ref, computed } from 'vue'
import { Chat24Regular } from '@vicons/fluent'
import { Terminal } from '@vicons/tabler'
import { devicesService } from '../services/devices'

const props = defineProps<{
  deviceId: string
  vowifiActive?: boolean
}>()

const ussdCmd = ref('')
const ussdTimeoutMs = ref(45000)
const sending = ref(false)
const sessionId = ref('')
const sessionChannel = ref('')
const history = ref<Array<{ ts: number; type: 'req' | 'res' | 'err' | 'sys'; content: string; dcs?: number; channel?: string }>>([])

const STORAGE_KEY = `vohive:ussd-history:${props.deviceId}`
const MAX_RECORDS = 50

// 从 localStorage 加载历史
try {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved) history.value = JSON.parse(saved).slice(-MAX_RECORDS)
} catch { /* ignore */ }

function saveHistory() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(history.value.slice(-MAX_RECORDS)))
  } catch { /* ignore quota */ }
}

const isMultiRound = computed(() => !!sessionId.value)
const inputPlaceholder = computed(() => isMultiRound.value ? '输入菜单选项数字' : '例如 *100# 或菜单回复数字')

const statusText = computed(() => {
  if (sending) return '等待响应...'
  if (isMultiRound.value) return '多轮会话中'
  return '空闲'
})

async function sendUSSD() {
  const cmd = String(ussdCmd.value || '').trim()
  if (!cmd) return

  history.value.push({ ts: Date.now(), type: 'req', content: cmd })
  saveHistory()
  sending.value = true
  ussdCmd.value = ''

  try {
    let d: { status?: number; text?: string; rawText?: string; dcs?: number; sessionId?: string; channel?: string }

    if (isMultiRound.value) {
      const result = await devicesService.continueUSSD(props.deviceId, {
        session_id: sessionId.value,
        input: cmd,
        timeout_ms: ussdTimeoutMs.value || 45000
      }, (ussdTimeoutMs.value || 45000) + 2000)
      if (!result.ok) throw new Error(result.error.message || '请求异常')
      d = result.data
    } else {
      const result = await devicesService.sendUSSD(props.deviceId, {
        command: cmd,
        timeout_ms: ussdTimeoutMs.value || 45000
      }, (ussdTimeoutMs.value || 45000) + 2000)
      if (!result.ok) throw new Error(result.error.message || '请求异常')
      d = result.data
    }

    if (d.channel) sessionChannel.value = d.channel

    if (d.status === 5) {
      history.value.push({
        ts: Date.now(),
        type: 'err',
        content: `[网络不支持/无响应]\n` + (d.text || d.rawText || '[空响应]'),
        dcs: d.dcs,
        channel: d.channel
      })
      saveHistory()
      endSession()
    } else if (d.status === 2) {
      history.value.push({
        ts: Date.now(),
        type: 'err',
        content: `[被网络终止]\n` + (d.text || d.rawText || '[空响应]'),
        dcs: d.dcs,
        channel: d.channel
      })
      saveHistory()
      endSession()
    } else {
      history.value.push({
        ts: Date.now(),
        type: 'res',
        content: d.text || d.rawText || '[空响应]',
        dcs: d.dcs,
        channel: d.channel
      })
      saveHistory()
      if (d.status === 1 && d.sessionId) {
        sessionId.value = d.sessionId
      } else {
        endSession()
      }
    }
  } catch (e: unknown) {
    history.value.push({
      ts: Date.now(),
      type: 'err',
      content: e instanceof Error ? e.message : '请求异常'
    })
    saveHistory()
    endSession()
  } finally {
    sending.value = false
  }
}

async function cancelSession() {
  if (!sessionId.value) return
  try {
    await devicesService.cancelUSSD(props.deviceId, sessionId.value)
    history.value.push({
      ts: Date.now(),
      type: 'sys',
      content: '会话已手动取消'
    })
    saveHistory()
  } catch {
    // 忽略取消错误
  }
  endSession()
}

function endSession() {
  sessionId.value = ''
  sessionChannel.value = ''
}

function clearHistory() {
  history.value = []
  try { localStorage.removeItem(STORAGE_KEY) } catch { /* ignore */ }
  endSession()
}
</script>

<template>
  <div class="module-ussd-terminal">
    <!-- 头部（含状态信息） -->
    <div class="terminal-card-header">
      <div class="terminal-icon-box">
        <el-icon size="14"><Chat24Regular /></el-icon>
      </div>
      <div class="terminal-header-title">USSD 交互终端</div>
      <div class="terminal-header-status">
        <span>USSD<span v-if="sessionChannel" class="terminal-status-port"> · {{ sessionChannel === 'vowifi' ? 'VoWiFi' : 'CS' }}</span></span>
        <span class="terminal-status-state" :class="{ 'is-busy': sending, 'is-multi': isMultiRound }">{{ statusText }}</span>
      </div>
    </div>

    <!-- 快捷工具栏 -->
    <div class="terminal-toolbar">
      <div class="terminal-toolbar-left">
        <el-button v-if="isMultiRound" size="small" type="warning" plain @click="cancelSession" :disabled="sending">取消会话</el-button>
      </div>
      <el-button size="small" @click="clearHistory">清空</el-button>
    </div>

    <!-- 会话记录区 -->
    <div class="terminal-output">
      <div v-if="history.length === 0 && !sending" class="terminal-output-empty">
        暂无记录，在下方输入指令并回车发送
      </div>
      <template v-for="(msg, i) in history" :key="i">
        <!-- 请求记录 -->
        <div v-if="msg.type === 'req'" class="terminal-entry">
          <div class="terminal-cmd-line">
            <span class="terminal-prompt">&gt;</span>
            <span class="terminal-cmd-text">{{ msg.content }}</span>
            <span class="terminal-ts">{{ new Date(msg.ts).toLocaleTimeString() }}</span>
          </div>
        </div>
        <!-- 系统消息 -->
        <div v-else-if="msg.type === 'sys'" class="terminal-sys-msg">
          — {{ msg.content }} —
        </div>
        <!-- 响应/错误 -->
        <div v-else class="terminal-entry">
          <div class="terminal-response" :class="{ 'is-error': msg.type === 'err' }">{{ msg.content }}</div>
          <div class="terminal-meta">
            <span class="terminal-ts">{{ new Date(msg.ts).toLocaleTimeString() }}</span>
            <span v-if="msg.dcs !== undefined" class="terminal-tag">DCS: {{ msg.dcs }}</span>
            <span v-if="msg.channel" class="terminal-tag">{{ msg.channel === 'vowifi' ? 'VoWiFi' : 'CS' }}</span>
          </div>
        </div>
      </template>
      <div v-if="sending" class="terminal-waiting">
        <span class="terminal-prompt">&gt;</span>
        <span class="terminal-dots"><span></span><span></span><span></span></span>
        <span class="terminal-waiting-text">等待网络响应...</span>
      </div>
    </div>

    <!-- 命令输入栏 -->
    <div class="terminal-input-bar">
        <el-icon size="14" class="terminal-input-prompt"><Terminal /></el-icon>
      <el-input
        v-model="ussdCmd"
        :placeholder="inputPlaceholder"
        @keyup.enter="sendUSSD"
        :disabled="sending"
        size="small"
        class="terminal-input-field"
      />
      <el-input v-model.number="ussdTimeoutMs" type="number" inputmode="numeric" placeholder="超时" title="超时毫秒(ms)" size="small" class="terminal-timeout-field" />
      <el-button type="primary" :loading="sending" :disabled="!ussdCmd" @click="sendUSSD" size="small">
        {{ isMultiRound ? '回复' : '发送' }}
      </el-button>
    </div>
  </div>
</template>

<style scoped>
@import '../assets/button/terminal-theme.css';

.module-ussd-terminal {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--background);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}

/* 头部 — 不可折叠 */
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
  flex-shrink: 0;
}

.terminal-header-status {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-left: auto;
  font-size: 12px;
  font-family: var(--oomol-font-sans);
  color: var(--muted-foreground);
}

.terminal-header-status .terminal-status-port {
  color: var(--foreground);
  font-family: var(--oomol-font-mono);
}

.terminal-header-status .terminal-status-state {
  color: var(--muted-foreground);
}

.terminal-header-status .terminal-status-state.is-busy {
  color: var(--warning);
}

.terminal-header-status .terminal-status-state.is-multi {
  color: var(--brand);
}
</style>
