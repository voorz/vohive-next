<script setup lang="ts">
import { ref, computed } from 'vue'
import { Phone24Regular } from '@vicons/fluent'
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
      endSession()
    } else if (d.status === 2) {
      history.value.push({
        ts: Date.now(),
        type: 'err',
        content: `[被网络终止]\n` + (d.text || d.rawText || '[空响应]'),
        dcs: d.dcs,
        channel: d.channel
      })
      endSession()
    } else {
      history.value.push({
        ts: Date.now(),
        type: 'res',
        content: d.text || d.rawText || '[空响应]',
        dcs: d.dcs,
        channel: d.channel
      })
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
  endSession()
}
</script>

<template>
  <div>
    <div class="flex items-center gap-3">
      <div class="tab-icon-box">
        <el-icon size="20"><Phone24Regular /></el-icon>
      </div>
      <div class="flex-1">
        <div class="tab-title">USSD 交互终端</div>
        <div class="tab-desc">发送 USSD 代码 (如 *100#) 并等待网络菜单响应</div>
      </div>
    </div>

    <div class="terminal">
      <!-- 状态栏 -->
      <div class="terminal-status-bar">
        <span>USSD 基站短码通道<span v-if="sessionChannel" class="terminal-status-port"> · {{ sessionChannel === 'vowifi' ? 'VoWiFi' : 'CS' }}</span></span>
        <span class="terminal-status-state" :class="{ 'is-busy': sending, 'is-multi': isMultiRound }">{{ statusText }}</span>
      </div>

      <!-- 快捷工具栏 -->
      <div class="terminal-toolbar">
        <div class="terminal-toolbar-left">
          <el-button v-if="isMultiRound" size="small" type="warning" plain @click="cancelSession" :disabled="sending">取消会话</el-button>
        </div>
        <el-button size="small" @click="clearHistory" class="terminal-toolbar-btn">清空</el-button>
      </div>

      <!-- 会话记录区 -->
      <div class="terminal-output">
        <div v-if="history.length === 0 && !sending" class="terminal-output-empty">
          暂无 USSD 会话记录，在下方输入指令并回车发送
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
        <span class="terminal-input-prompt">&gt;</span>
        <el-input
          v-model="ussdCmd"
          :placeholder="inputPlaceholder"
          @keyup.enter="sendUSSD"
          :disabled="sending"
          size="small"
          class="terminal-input-field"
        />
        <el-input v-model.number="ussdTimeoutMs" type="number" inputmode="numeric" placeholder="超时" title="超时毫秒(ms)" size="small" class="terminal-timeout-field" />
        <el-button type="primary" :loading="sending" :disabled="!ussdCmd" @click="sendUSSD" size="small" class="terminal-send-btn">
          {{ isMultiRound ? '回复' : '发送' }}
        </el-button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tab-icon-box {
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

.tab-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
}

.tab-desc {
  font-size: 12px;
  color: var(--muted-foreground);
  margin-top: 2px;
}

/* ── 终端容器 ── */
.terminal {
  margin-top: 16px;
  border: 1px solid #2A2B2D;
  border-radius: 8px;
  overflow: hidden;
}

/* 状态栏 */
.terminal-status-bar {
  background: #131416;
  color: #9CA3AF;
  padding: 0 16px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
  border-bottom: 1px solid #2A2B2D;
}

.terminal-status-port {
  color: #D1D5DB;
  font-family: monospace;
}

.terminal-status-state {
  color: #6B7280;
}

.terminal-status-state.is-busy {
  color: #F59E0B;
}

.terminal-status-state.is-multi {
  color: #10B981;
}

/* 快捷工具栏 */
.terminal-toolbar {
  background: #0B0C0E;
  padding: 8px 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  border-bottom: 1px solid #2A2B2D;
}

.terminal-toolbar-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.terminal-toolbar-btn {
  flex-shrink: 0;
}

/* 会话记录区 */
.terminal-output {
  background: #111111;
  height: 320px;
  overflow-y: auto;
  padding: 12px 16px;
  font-family: 'Cascadia Code', 'Fira Code', 'JetBrains Mono', 'Consolas', monospace;
  font-size: 13px;
  color: #E5E7EB;
  border-bottom: 1px solid #2A2B2D;
}

.terminal-output-empty {
  color: #4B5563;
  font-size: 13px;
  text-align: center;
  padding-top: 120px;
}

.terminal-entry {
  margin-bottom: 8px;
}

.terminal-cmd-line {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.terminal-prompt {
  color: #10B981;
  font-weight: 700;
  flex-shrink: 0;
}

.terminal-cmd-text {
  color: #E5E7EB;
  word-break: break-all;
}

.terminal-ts {
  color: #374151;
  font-size: 11px;
  margin-left: auto;
  flex-shrink: 0;
}

.terminal-response {
  color: #9CA3AF;
  white-space: pre-wrap;
  word-break: break-all;
  padding-left: 20px;
  margin-top: 2px;
}

.terminal-response.is-error {
  color: #EF4444;
}

.terminal-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-left: 20px;
  margin-top: 4px;
}

.terminal-tag {
  color: #4B5563;
  font-size: 11px;
  background: #1F1F1F;
  padding: 1px 6px;
  border-radius: 3px;
}

.terminal-sys-msg {
  color: #4B5563;
  font-size: 12px;
  text-align: center;
  padding: 4px 0;
}

.terminal-waiting {
  display: flex;
  align-items: center;
  gap: 8px;
}

.terminal-dots {
  display: inline-flex;
  gap: 3px;
}

.terminal-dots span {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: #6B7280;
  animation: terminal-bounce 1.4s infinite ease-in-out both;
}

.terminal-dots span:nth-child(1) { animation-delay: -0.32s; }
.terminal-dots span:nth-child(2) { animation-delay: -0.16s; }

.terminal-waiting-text {
  color: #6B7280;
  font-size: 12px;
}

@keyframes terminal-bounce {
  0%, 80%, 100% { transform: scale(0.6); opacity: 0.4; }
  40% { transform: scale(1); opacity: 1; }
}

/* 命令输入栏 */
.terminal-input-bar {
  background: #131416;
  padding: 8px 16px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.terminal-input-prompt {
  color: #10B981;
  font-weight: 700;
  flex-shrink: 0;
  font-size: 14px;
}

.terminal-input-field {
  flex: 1;
}

.terminal-timeout-field {
  width: 100px;
  flex-shrink: 0;
}

.terminal-send-btn {
  flex-shrink: 0;
}
</style>
