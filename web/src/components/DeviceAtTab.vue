<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Code24Regular, Warning24Regular } from '@vicons/fluent'
import { AT_TEMPLATES } from '../constants/atTemplates'
import { devicesService } from '../services/devices'

const props = defineProps<{
  deviceId: string
  backendMode?: string
  atPort?: string
  running?: boolean
}>()

const atCmd = ref('')
const atTemplate = ref('')
const atTimeoutMs = ref(10000)
const atSending = ref(false)
const atHistory = ref<Array<{ ts: number; cmd: string; ok: boolean; response: string }>>([])

const atTemplates = AT_TEMPLATES
const hasATPort = computed(() => String(props.atPort || '').trim().length > 0)
const canUseATTerminal = computed(() => Boolean(props.running) && hasATPort.value)
const unavailableTitle = computed(() => {
  if (!props.running) return '当前设备未运行'
  if (!hasATPort.value) return '当前设备没有可用 AT 口'
  return 'AT 终端暂不可用'
})
const unavailableDescription = computed(() => {
  if (!props.running) {
    return '设备当前未启动，AT 终端暂时不可用。待设备运行后，如果存在可用的 AT 口，即可在这里直接发送 AT 指令。'
  }
  if (!hasATPort.value && props.backendMode === 'qmi') {
    return '设备当前处于纯 QMI 模式，但没有解析到可用的 AT 口，因此无法提供 AT 串口终端。'
  }
  if (!hasATPort.value) {
    return '设备当前没有可用的 AT 口，因此无法提供 AT 串口终端。'
  }
  return '当前设备暂时无法提供 AT 串口终端，请稍后重试。'
})

const statusText = computed(() => atSending.value ? '发送中...' : '空闲')

watch(
  () => atTemplate.value,
  (v) => {
    const cmd = String(v || '').trim()
    if (cmd) atCmd.value = cmd
  }
)

async function sendAT() {
  const cmd = String(atCmd.value || '').trim()
  if (!cmd) return
  atSending.value = true
  atCmd.value = ''
  try {
    const result = await devicesService.sendAT(props.deviceId, {
      cmd: cmd,
      timeout_ms: atTimeoutMs.value || 10000
    })
    if (!result.ok) throw new Error(result.error.message || '请求异常')
    atHistory.value.push({
      ts: Date.now(),
      cmd,
      ok: result.data.ok,
      response: result.data.response
    })
  } catch (e: unknown) {
    atHistory.value.push({
      ts: Date.now(),
      cmd,
      ok: false,
      response: e instanceof Error ? e.message : '请求异常'
    })
  } finally {
    atSending.value = false
  }
}

function clearATHistory() {
  atHistory.value = []
}
</script>

<template>
  <div>
    <div class="flex items-center gap-3">
      <div class="tab-icon-box">
        <el-icon size="20"><Code24Regular /></el-icon>
      </div>
      <div>
        <div class="tab-title">AT 终端</div>
        <div class="tab-desc">发送 AT 指令并查看回显（多行响应会完整返回）</div>
      </div>
    </div>

    <template v-if="!canUseATTerminal">
      <div class="terminal-unavailable">
        <el-icon size="48" class="terminal-unavailable-icon"><Warning24Regular /></el-icon>
        <div class="terminal-unavailable-title">{{ unavailableTitle }}</div>
        <div class="terminal-unavailable-desc">{{ unavailableDescription }}</div>
      </div>
    </template>

    <template v-else>
      <div class="terminal">
        <!-- 状态栏 -->
        <div class="terminal-status-bar">
          <span>AT 协议指令终端 · <span class="terminal-status-port">{{ atPort || '--' }}</span></span>
          <span class="terminal-status-state" :class="{ 'is-busy': atSending }">{{ statusText }}</span>
        </div>

        <!-- 快捷工具栏 -->
        <div class="terminal-toolbar">
          <el-select v-model="atTemplate" filterable clearable placeholder="常用命令（可选）" size="small" class="terminal-toolbar-select">
            <el-option-group v-for="g in atTemplates" :key="g.label" :label="g.label">
              <el-option v-for="it in g.items" :key="it.value" :label="it.label" :value="it.value" />
            </el-option-group>
          </el-select>
          <el-button size="small" @click="clearATHistory" class="terminal-toolbar-btn">清空</el-button>
        </div>

        <!-- 会话记录区 -->
        <div class="terminal-output">
          <div v-if="atHistory.length === 0 && !atSending" class="terminal-output-empty">
            暂无 AT 会话记录，在下方输入指令并回车发送
          </div>
          <div v-for="(h, i) in atHistory" :key="h.ts + h.cmd + i" class="terminal-entry">
            <div class="terminal-cmd-line">
              <span class="terminal-prompt">&gt;</span>
              <span class="terminal-cmd-text">{{ h.cmd }}</span>
              <span class="terminal-ts">{{ new Date(h.ts).toLocaleTimeString() }}</span>
            </div>
            <div class="terminal-response" :class="{ 'is-error': !h.ok }">{{ h.response }}</div>
          </div>
          <div v-if="atSending" class="terminal-waiting">
            <span class="terminal-prompt">&gt;</span>
            <span class="terminal-dots"><span></span><span></span><span></span></span>
            <span class="terminal-waiting-text">等待模组响应...</span>
          </div>
        </div>

        <!-- 命令输入栏 -->
        <div class="terminal-input-bar">
          <span class="terminal-input-prompt">&gt;</span>
          <el-input
            v-model="atCmd"
            placeholder="输入命令"
            @keyup.enter="sendAT"
            :disabled="atSending"
            size="small"
            class="terminal-input-field"
          />
          <el-input v-model.number="atTimeoutMs" type="number" inputmode="numeric" placeholder="超时" title="超时毫秒(ms)" size="small" class="terminal-timeout-field" />
          <el-button type="primary" :loading="atSending" :disabled="!atCmd" @click="sendAT" size="small" class="terminal-send-btn">
            发送
          </el-button>
        </div>
      </div>
    </template>
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

/* ── 不可用提示 ── */
.terminal-unavailable {
  margin-top: 16px;
  border: 1px solid #2A2B2D;
  border-radius: 8px;
  background: #131416;
  padding: 48px 24px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
}

.terminal-unavailable-icon {
  color: #F59E0B;
  margin-bottom: 16px;
}

.terminal-unavailable-title {
  font-size: 16px;
  font-weight: 700;
  color: #F59E0B;
  margin-bottom: 8px;
}

.terminal-unavailable-desc {
  font-size: 13px;
  color: #9CA3AF;
  max-width: 400px;
  line-height: 1.5;
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

.terminal-toolbar-select {
  width: 260px;
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
