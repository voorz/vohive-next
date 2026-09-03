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
const deviceColors = [
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

function deviceColor(tag: string): string {
  let hash = 0
  for (let i = 0; i < tag.length; i++) {
    hash = ((hash << 5) - hash + tag.charCodeAt(i)) | 0
  }
  return deviceColors[Math.abs(hash) % deviceColors.length]
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
  try {
    const d = new Date(isoTime)
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
</script>

<template>
  <div class="log-line" @click="emit('open-detail', props.log)">
    <span class="log-time">[{{ formatDateTime(log.time) }}]</span>
    <span class="log-level" :class="levelClass">{{ log.level.toUpperCase().padEnd(5) }}</span>
    <span class="log-caller" :title="log.caller">{{ log.caller }}</span>
    <span class="log-device-slot">
      <span v-if="deviceTag" class="log-device-tag" :style="deviceStyle">{{ deviceTag }}</span>
    </span>
    <span class="log-message" :class="messageClass">{{ cleanMessage }}</span>
    <span v-if="log.fields" class="log-fields">{{ log.fields }}</span>
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
  width: 80px;
  margin-left: 4px;
  flex-shrink: 0;
  overflow: hidden;
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
  color: var(--brand, #00BC7D);
  flex-shrink: 0;
}

.log-message {
  margin-left: 4px;
  min-width: 0;
}

.log-msg-warn  { color: var(--warning, #da9f00); }
.log-msg-error { color: var(--destructive, #ff3b30); }

.log-fields {
  margin-left: 4px;
  color: color-mix(in oklab, var(--warning, #da9f00) 70%, transparent);
}
</style>
