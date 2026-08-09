<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Terminal } from '@vicons/tabler'
import { WindowConsole20Regular } from '@vicons/fluent'
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
  <div class="module-at-terminal">
    <!-- 头部（含状态信息） -->
    <div class="terminal-card-header">
      <div class="terminal-icon-box">
        <el-icon size="14"><WindowConsole20Regular /></el-icon>
      </div>
      <div class="terminal-header-title">AT 终端</div>
      <div class="terminal-header-status">
        <span>AT · <span class="terminal-status-port">{{ atPort || '--' }}</span></span>
        <span class="terminal-status-state" :class="{ 'is-busy': atSending }">{{ statusText }}</span>
      </div>
    </div>

    <!-- 不可用提示 -->
    <div v-if="!canUseATTerminal" class="terminal-unavailable">
      <span class="terminal-unavailable-text">
        {{ !running ? '设备未运行' : !hasATPort ? '无可用 AT 口' : 'AT 终端暂不可用' }}
      </span>
    </div>

    <!-- 终端 -->
    <template v-else>
      <!-- 快捷工具栏 -->
      <div class="terminal-toolbar">
        <el-select v-model="atTemplate" filterable clearable placeholder="常用命令" size="small" class="terminal-toolbar-select">
          <el-option-group v-for="g in atTemplates" :key="g.label" :label="g.label">
            <el-option v-for="it in g.items" :key="it.value" :label="it.label" :value="it.value" />
          </el-option-group>
        </el-select>
        <el-button size="small" @click="clearATHistory">清空</el-button>
      </div>

      <!-- 会话记录区 -->
      <div class="terminal-output">
        <div v-if="atHistory.length === 0 && !atSending" class="terminal-output-empty">
          暂无记录，在下方输入指令并回车发送
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
        <el-icon size="14" class="terminal-input-prompt"><Terminal /></el-icon>
        <el-input
          v-model="atCmd"
          placeholder="输入命令"
          @keyup.enter="sendAT"
          :disabled="atSending"
          size="small"
          class="terminal-input-field"
        />
        <el-input v-model.number="atTimeoutMs" type="number" inputmode="numeric" placeholder="超时" title="超时毫秒(ms)" size="small" class="terminal-timeout-field" />
        <el-button type="primary" :loading="atSending" :disabled="!atCmd" @click="sendAT" size="small">
          发送
        </el-button>
      </div>
    </template>
  </div>
</template>

<style scoped>
@import '../assets/button/terminal-theme.css';

.module-at-terminal {
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

/* 不可用提示 */
.terminal-unavailable {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 48px 24px;
  text-align: center;
}

.terminal-unavailable-text {
  font-size: 13px;
  color: var(--muted-foreground);
}
</style>
