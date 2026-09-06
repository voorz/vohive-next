<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { Backspace24Regular, Call24Regular } from '@vicons/fluent'

const props = defineProps<{
  disabled?: boolean
  compact?: boolean
}>()

const emit = defineEmits<{
  dial: [number: string]
  'key-press': [key: string]
}>()

const input = ref('')

const keys = [
  { num: '1', letters: '' },
  { num: '2', letters: 'ABC' },
  { num: '3', letters: 'DEF' },
  { num: '4', letters: 'GHI' },
  { num: '5', letters: 'JKL' },
  { num: '6', letters: 'MNO' },
  { num: '7', letters: 'PQRS' },
  { num: '8', letters: 'TUV' },
  { num: '9', letters: 'WXYZ' },
  { num: '*', letters: '' },
  { num: '0', letters: '+' },
  { num: '#', letters: '' },
]

const canDial = computed(() => input.value.length > 0 && !props.disabled)

function pressKey(key: string) {
  if (props.disabled) return
  input.value += key
  emit('key-press', key)
}

function backspace() {
  if (props.disabled) return
  input.value = input.value.slice(0, -1)
}

function clearAll() {
  if (props.disabled) return
  input.value = ''
}

function onDial() {
  if (!canDial.value) return
  emit('dial', input.value)
}

function setNumber(num: string) {
  input.value = num
}

function clear() {
  input.value = ''
}

// PC 键盘输入支持
const validKeys = new Set(['0','1','2','3','4','5','6','7','8','9','*','#','+'])

function onKeydown(e: KeyboardEvent) {
  if (props.disabled) return
  const target = e.target as HTMLElement
  if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable) return

  const key = e.key

  if (key === 'Enter' && canDial.value) {
    e.preventDefault()
    onDial()
    return
  }
  if (key === 'Backspace') {
    e.preventDefault()
    backspace()
    return
  }
  if (key === 'Escape') {
    e.preventDefault()
    clearAll()
    return
  }
  if (validKeys.has(key)) {
    e.preventDefault()
    pressKey(key)
    return
  }
  if (e.shiftKey && key === '8') {
    e.preventDefault()
    pressKey('*')
    return
  }
  if (e.shiftKey && key === '3') {
    e.preventDefault()
    pressKey('#')
    return
  }
  if (e.shiftKey && (key === '=' || key === '+')) {
    e.preventDefault()
    pressKey('+')
    return
  }
}

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
})

defineExpose({ setNumber, clear })
</script>

<template>
  <div class="voice-dialer">
    <!-- 号码显示（compact 模式隐藏） -->
    <div v-if="!compact" class="number-display">
      <span v-if="!input" class="placeholder">输入号码</span>
      <span v-else class="number-text">{{ input }}</span>
    </div>

    <!-- 键盘 -->
    <div class="keypad">
      <button
        v-for="key in keys"
        :key="key.num"
        class="key"
        :disabled="disabled"
        @click="pressKey(key.num)"
      >
        <span class="num" :class="{ star: key.num === '*', hash: key.num === '#' }">{{ key.num }}</span>
        <span class="letters" :class="{ invisible: !key.letters }">{{ key.letters }}</span>
      </button>
    </div>

    <!-- 底部行：空位 + 拨号 + 退格（compact 模式隐藏） -->
    <div v-if="!compact" class="bottom-row">
      <div class="bottom-spacer" />
      <el-button
        class="dial-btn"
        type="success"
        :icon="Call24Regular"
        circle
        size="large"
        :disabled="!canDial"
        @click="onDial"
      />
      <button
        class="backspace-btn"
        :disabled="!input || disabled"
        @click="backspace"
        @contextmenu.prevent="clearAll"
        title="点击退格 / 右键清空"
      >
        <el-icon :size="30"><Backspace24Regular /></el-icon>
      </button>
    </div>
  </div>
</template>

<style scoped>
.voice-dialer {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 12px 0;
  width: 100%;
  margin: 0 auto;
}

/* compact 模式 */
.voice-dialer:has(.keypad:only-child) {
  padding: 8px 0;
  gap: 8px;
}

/* 号码显示 */
.number-display {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 40px;
  width: 100%;
}

.placeholder {
  font-size: 14px;
  color: var(--muted-foreground);
}

.number-text {
  font-size: 24px;
  font-weight: 500;
  color: var(--foreground);
  font-family: var(--oomol-font-mono);
  letter-spacing: 1px;
}

/* 退格按钮：原生 button，无悬停背景 */
.backspace-btn {
  flex-shrink: 0;
  width: 80px;
  height: 80px;
  justify-self: center;
  box-sizing: border-box;
  padding: 0;
  margin: 0;
  border: none;
  background: transparent;
  color: var(--muted-foreground);
  cursor: pointer;
  font-family: inherit;
  outline: none;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: color 0.15s;
}

.backspace-btn:hover:not(:disabled) {
  color: var(--foreground);
}

.backspace-btn:active:not(:disabled) {
  color: var(--brand);
}

.backspace-btn:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}

.backspace-btn :deep(svg) {
  width: 30px;
  height: 30px;
}

/* 键盘 */
.keypad {
  display: grid;
  grid-template-columns: repeat(3, 80px);
  gap: 14px;
  justify-content: center;
}

.key {
  width: 80px;
  height: 80px;
  border-radius: 999px;
  padding: 0;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  margin: 0;
  border: 1px solid var(--border);
  background: var(--card);
  color: var(--foreground);
  cursor: pointer;
  font-family: inherit;
  outline: none;
  transition: background 0.15s;
}

.key:hover:not(:disabled) {
  background: var(--muted);
}

.key:active:not(:disabled) {
  background: var(--border);
}

.key:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.key .num,
.key .letters {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
}

.key .num {
  font-size: 35px;
  font-weight: 400;
  line-height: 1;
}

.key .num.star,
.key .num.hash {
  font-size: 26px;
  line-height: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 35px;
}

.key .letters {
  font-size: 10px;
  font-weight: 400;
  color: var(--muted-foreground);
  line-height: 1;
  min-height: 10px;
}

.key .letters.invisible {
  visibility: hidden;
}

/* 底部行：和键盘同宽，三列等分 */
.bottom-row {
  display: grid;
  grid-template-columns: repeat(3, 80px);
  gap: 14px;
  justify-content: center;
  align-items: center;
}

.bottom-spacer {
  /* 占位，和键盘第一列对齐 */
}

/* 拨号按钮：居中于中间列 */
.dial-btn {
  width: 80px !important;
  height: 80px !important;
  min-width: 0 !important;
  justify-self: center;
  box-sizing: border-box !important;
  padding: 0 !important;
}

.dial-btn :deep(.el-icon),
.dial-btn :deep(svg) {
  width: 30px;
  height: 30px;
}
</style>
