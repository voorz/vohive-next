<script setup lang="ts">
import { computed } from 'vue'

export interface LogEntry {
  time: string
  level: string
  caller: string
  message: string
  fields?: string
}

const props = defineProps<{
  log: LogEntry
}>()

const emit = defineEmits<{
  (e: 'open-detail', log: LogEntry): void
}>()

const levelClass = computed(() => {
  switch (props.log.level.toLowerCase()) {
    case 'debug': return 'log-lvl-debug'
    case 'info': return 'log-lvl-info'
    case 'warn': return 'log-lvl-warn'
    case 'error': return 'log-lvl-error'
    case 'fatal': return 'log-lvl-fatal'
    default: return 'log-lvl-default'
  }
})

const messageClass = computed(() => {
  switch (props.log.level.toLowerCase()) {
    case 'warn': return 'log-msg-warn'
    case 'error': return 'log-msg-error'
    default: return ''
  }
})

// 预定义调色板（避免随机性太大）
const _deviceColors = [
  '#00BC7D', // 品牌绿
  '#5B9EFF', // 蓝
  '#B06DFF', // 紫
  '#FF8C42', // 橙
  '#FF6B9D', // 粉
  '#4FD1C5', // 青
  '#FFD93D', // 黄
  '#6FCF97', // 浅绿
  '#BB6BD9', // 深紫
  '#EB5757', // 红
]

// 从设备名称提取数字 → HSL 色相 → hex 颜色
function deviceColor(tag: string): string {
  // 提取所有数字字符拼接为整数
  const digits = tag.match(/\d/g)
  let hue: number
  if (digits && digits.length > 0) {
    hue = parseInt(digits.join(''), 10) % 360
  } else {
    // 无数字时用字符 ASCII 和作色相
    let sum = 0
    for (let i = 0; i < tag.length; i++) {
      sum += tag.charCodeAt(i)
    }
    hue = sum % 360
  }
  return hslToHex(hue, 70, 55)
}

// HSL → HEX
function hslToHex(h: number, s: number, l: number): string {
  s /= 100
  l /= 100
  const c = (1 - Math.abs(2 * l - 1)) * s
  const x = c * (1 - Math.abs(((h / 60) % 2) - 1))
  const m = l - c / 2
  let r = 0, g = 0, b = 0
  if (h < 60) { r = c; g = x; b = 0 }
  else if (h < 120) { r = x; g = c; b = 0 }
  else if (h < 180) { r = 0; g = c; b = x }
  else if (h < 240) { r = 0; g = x; b = c }
  else if (h < 300) { r = x; g = 0; b = c }
  else { r = c; g = 0; b = x }
  const toHex = (v: number) => Math.round((v + m) * 255).toString(16).padStart(2, '0')
  return `#${toHex(r)}${toHex(g)}${toHex(b)}`
}

const deviceTag = computed(() => {
  const m = props.log.message.match(/^\[([^\]]+)\]\s/)
  if (!m) return ''
  // 过滤掉过长的标识和纯 IMSI 数字
  const tag = m[1]
  if (tag.length > 20) return ''
  return tag
})

const deviceStyle = computed(() => {
  if (!deviceTag.value) return {}
  const color = deviceColor(deviceTag.value)
  return {
    color: color,
    background: `${color}22`,
    borderColor: `${color}55`,
  }
})

const cleanMessage = computed(() => {
  if (deviceTag.value) {
    return props.log.message.replace(/^\[[^\]]+\]\s/, '')
  }
  return props.log.message
})

function formatDateTime(isoTime: string): string {
  if (!isoTime) return ''
  try {
    const d = new Date(isoTime)
    if (isNaN(d.getTime())) return isoTime
    const MM = String(d.getMonth() + 1).padStart(2, '0')
    const dd = String(d.getDate()).padStart(2, '0')
    const HH = String(d.getHours()).padStart(2, '0')
    const mm = String(d.getMinutes()).padStart(2, '0')
    const ss = String(d.getSeconds()).padStart(2, '0')
    return `${MM}-${dd} ${HH}:${mm}:${ss}`
  } catch {
    return isoTime
  }
}
let mouseDownX = 0
let mouseDownY = 0

function onMouseDown(e: MouseEvent) {
  mouseDownX = e.clientX
  mouseDownY = e.clientY
}

function onClick(e: MouseEvent) {
  // 如果拖拽距离大于 5px，说明是在选中文本，不触发详情
  const dx = Math.abs(e.clientX - mouseDownX)
  const dy = Math.abs(e.clientY - mouseDownY)
  if (dx > 5 || dy > 5) return
  // 如果有文本被选中，不触发详情
  const selection = window.getSelection()
  if (selection && selection.toString().length > 0) return
  emit('open-detail', props.log)
}
</script>

<template>
  <div class="log-line" @mousedown="onMouseDown" @click="onClick">
    <span v-if="log.time" class="log-time">[{{ formatDateTime(log.time) }}]</span>
    <span v-if="log.level" class="log-level" :class="levelClass">{{ log.level.toUpperCase().padEnd(5) }}</span>
    <span v-if="log.caller" class="log-caller" :title="log.caller">{{ log.caller }}</span>
    <span class="log-device-slot">
      <span v-if="deviceTag" class="log-device-tag" :style="deviceStyle">{{ deviceTag }}</span>
    </span>
    <span class="log-message" :class="messageClass">{{ cleanMessage }}<span v-if="log.fields" class="log-fields"> {{ log.fields }}</span></span>
  </div>
</template>

<style scoped>
.log-line {
  display: flex;
  align-items: baseline;
  padding: 1px 8px;
  margin: 0 -8px;
  border-radius: 4px;
  white-space: nowrap;
  line-height: 1.6;
  cursor: pointer;
}

.log-line:hover {
  background: rgba(255, 255, 255, 0.08);
}

.log-time {
  color: rgba(224, 224, 224, 0.5);
  flex-shrink: 0;
}

.log-level {
  display: inline-block;
  width: 56px;
  margin-left: 4px;
  font-weight: 700;
  flex-shrink: 0;
}

.log-lvl-debug { color: #b06dff; }
.log-lvl-info  { color: #5b9eff; }
.log-lvl-warn  { color: var(--warning, #da9f00); }
.log-lvl-error { color: var(--destructive, #ff3b30); }
.log-lvl-fatal { color: #ff3b30; font-weight: 900; }
.log-lvl-default { color: #888; }

.log-device-slot {
  display: inline-block;
  margin-left: 4px;
  flex-shrink: 1;
  vertical-align: bottom;
}

.log-device-tag {
  display: inline-block;
  padding: 0 4px;
  border-radius: 3px;
  font-size: 10px;
  font-weight: 600;
  border: 1px solid transparent;
}

.log-caller {
  display: inline-block;
  width: 192px;
  margin-left: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  vertical-align: bottom;
  color: rgba(224, 224, 224, 0.5);
  flex-shrink: 0;
}

.log-message {
  margin-left: 4px;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}

.log-msg-warn  { color: var(--warning, #da9f00); }
.log-msg-error { color: var(--destructive, #ff3b30); }

.log-fields {
  color: rgba(224, 224, 224, 0.6);
}

.log-msg-warn .log-fields {
  color: var(--warning, #da9f00);
}

.log-msg-error .log-fields {
  color: var(--destructive, #ff3b30);
}

</style>
