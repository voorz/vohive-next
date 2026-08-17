<script setup lang="ts">
import { computed } from 'vue'
import ModuleVoiceDialer from './ModuleVoiceDialer.vue'
import {
  CallEnd24Regular,
  CallCheckmark24Regular,
  Dialpad24Regular,
} from '@vicons/fluent'

const props = defineProps<{
  callState: 'idle' | 'dialing' | 'ringing' | 'connected' | 'hanging'
  callNumber: string
  callTimer: number
  showDtmf: boolean
}>()

const emit = defineEmits<{
  answer: []
  hangup: []
  'toggle-dtmf': []
  'dtmf-key': [key: string]
}>()

const callHint = computed(() => {
  switch (props.callState) {
    case 'dialing': return '拨号中...'
    case 'ringing': return '振铃中...'
    case 'connected': return '通话中'
    case 'hanging': return '挂断中...'
    default: return ''
  }
})

function formatTimer(sec: number): string {
  const m = String(Math.floor(sec / 60)).padStart(2, '0')
  const s = String(sec % 60).padStart(2, '0')
  return `${m}:${s}`
}

function onDtmfKey(key: string) {
  emit('dtmf-key', key)
}
</script>

<template>
  <div class="call-overlay">
    <!-- 顶部号码栏 -->
    <div class="call-top-bar">
      <div class="call-number">{{ callNumber }}</div>
      <div class="call-hint-row">
        <span class="call-hint-text">{{ callHint }}</span>
        <span v-if="callState === 'connected'" class="call-timer-text">{{ formatTimer(callTimer) }}</span>
      </div>
    </div>
    <!-- DTMF 拨号盘（展开时在中间） -->
    <div v-if="showDtmf" class="dtmf-panel">
      <ModuleVoiceDialer :disabled="false" :compact="true" @dial="() => {}" @key-press="onDtmfKey" />
    </div>
    <!-- 底部操作区 -->
    <div class="call-bottom-bar">
      <div class="call-actions">
        <!-- 来电接听按钮 -->
        <el-button
          v-if="callState === 'ringing'"
          class="call-btn answer"
          type="success"
          :icon="CallCheckmark24Regular"
          circle
          size="large"
          @click="emit('answer')"
        />
        <!-- 挂断按钮 -->
        <el-button
          class="call-btn hangup"
          type="danger"
          :icon="CallEnd24Regular"
          circle
          size="large"
          @click="emit('hangup')"
        />
        <!-- DTMF 键盘展开/收起 -->
        <el-button
          v-if="callState === 'connected' || callState === 'dialing'"
          class="call-btn dtmf-toggle"
          :class="{ active: showDtmf }"
          :icon="Dialpad24Regular"
          circle
          size="large"
          @click="emit('toggle-dtmf')"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 通话状态覆盖层 */
.call-overlay {
  position: absolute;
  inset: 0;
  z-index: 20;
  background: linear-gradient(135deg, color-mix(in oklab, var(--brand) 15%, #000) 0%, color-mix(in oklab, #0a3d2e 60%, #000) 100%);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 24px 16px 32px;
  gap: 14px;
}

/* 顶部号码栏 */
.call-top-bar {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.call-top-bar .call-number {
  font-size: 24px;
  font-weight: 500;
  color: #fff;
  font-family: var(--oomol-font-mono);
}

.call-hint-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.call-hint-text {
  font-size: 13px;
  color: rgba(255, 255, 255, 0.7);
}

.call-timer-text {
  font-size: 14px;
  font-weight: 600;
  color: #4ade80;
  font-variant-numeric: tabular-nums;
  font-family: var(--oomol-font-mono);
}

/* 底部操作区 */
.call-bottom-bar {
  display: flex;
  justify-content: center;
  width: 100%;
}

.call-actions {
  display: flex;
  gap: 48px;
}

.call-btn {
  width: 80px !important;
  height: 80px !important;
  min-width: 0 !important;
  box-sizing: border-box !important;
  padding: 0 !important;
}

.call-btn.hangup-only {
  margin: 0 auto;
}

.call-btn.dtmf-toggle {
  background: rgba(255, 255, 255, 0.15) !important;
  border-color: rgba(255, 255, 255, 0.3) !important;
  color: #fff !important;
}

.call-btn.dtmf-toggle.active {
  background: rgba(255, 255, 255, 0.3) !important;
}

/* DTMF 面板 */
.dtmf-panel {
  width: 100%;
  max-width: 320px;
  padding: 0 16px;
  animation: slide-up 0.3s ease;
}

/* 有键盘时：号码固定顶部，按钮固定底部 */
.call-overlay:has(.dtmf-panel) {
  justify-content: space-between;
}

.call-overlay:has(.dtmf-panel) .call-top-bar {
  padding-top: 0;
}

@keyframes slide-up {
  from { opacity: 0; transform: translateY(20px); }
  to { opacity: 1; transform: translateY(0); }
}

.call-btn :deep(.el-icon),
.call-btn :deep(svg) {
  width: 30px;
  height: 30px;
}
</style>
