<script setup lang="ts">
import { computed } from 'vue'
import { ElDrawer, ElButton, ElIcon, ElMessage } from 'element-plus'
import { Copy24Regular } from '@vicons/fluent'
import { type LogEntry } from './LogLine.vue'

const props = defineProps<{
  modelValue: boolean
  log: LogEntry | null
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', v: boolean): void
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

const deviceTag = computed(() => {
  if (!props.log) return ''
  const m = props.log.message.match(/^\[([^\]]+)\]\s/)
  if (!m) return ''
  const tag = m[1]
  if (tag.length > 20) return ''
  return tag
})

const deviceColors = [
  '#00BC7D', '#5B9EFF', '#B06DFF', '#FF8C42', '#FF6B9D',
  '#4FD1C5', '#FFD93D', '#6FCF97', '#BB6BD9', '#EB5757',
]

const deviceStyle = computed(() => {
  if (!deviceTag.value) return {}
  let hash = 0
  const tag = deviceTag.value
  for (let i = 0; i < tag.length; i++) {
    hash = ((hash << 5) - hash + tag.charCodeAt(i)) | 0
  }
  const color = deviceColors[Math.abs(hash) % deviceColors.length]
  return { color, background: `${color}22`, borderColor: `${color}55` }
})

const cleanMessage = computed(() => {
  if (!props.log) return ''
  if (deviceTag.value) {
    return props.log.message.replace(/^\[[^\]]+\]\s/, '')
  }
  return props.log.message
})

const formattedTime = computed(() => {
  if (!props.log) return ''
  try {
    const d = new Date(props.log.time)
    return d.toLocaleString()
  } catch {
    return props.log.time
  }
})

// 数据体：仅展示 fields 的格式化 JSON
const fieldsJson = computed(() => {
  if (!props.log?.fields) return null
  try {
    return JSON.stringify(JSON.parse(props.log.fields), null, 2)
  } catch {
    return props.log.fields
  }
})

// 原始日志行：拼接完整日志消息
const rawLog = computed(() => {
  if (!props.log) return ''
  const time = formattedTime.value
  const level = props.log.level.toUpperCase()
  const caller = props.log.caller
  const msg = props.log.message
  const fields = props.log.fields ? ' ' + props.log.fields : ''
  return `[${time}] ${level} ${caller} ${msg}${fields}`
})

async function copyRawLog() {
  if (!rawLog.value) return
  try {
    await navigator.clipboard.writeText(rawLog.value)
    ElMessage.success('已复制日志')
  } catch {
    ElMessage.error('复制失败')
  }
}
</script>

<template>
  <el-drawer
    v-model="visible"
    direction="btt"
    size="50%"
    resizable
    :with-header="false"
    :destroy-on-close="false"
    class="logs-detail-drawer"
  >
    <div class="drawer-body" v-if="log">
      <!-- 标题行 -->
      <div class="drawer-header">
        <span class="drawer-title">日志详情</span>
        <el-button class="header-copy-btn" size="small" @click="copyRawLog">
          <el-icon><Copy24Regular /></el-icon>
          复制
        </el-button>
      </div>

      <!-- 信息网格 -->
      <div class="drawer-info-grid">
        <div class="info-row">
          <span class="info-label">时间</span>
          <span class="info-value">{{ formattedTime }}</span>
        </div>
        <div class="info-row">
          <span class="info-label">级别</span>
          <span class="info-value">{{ log.level.toUpperCase() }}</span>
        </div>
        <div class="info-row">
          <span class="info-label">关联文件</span>
          <span class="info-value mono">{{ log.caller }}</span>
        </div>
        <div v-if="deviceTag" class="info-row">
          <span class="info-label">设备</span>
          <span class="info-value">
            <el-tag size="small" effect="plain" :style="deviceStyle">{{ deviceTag }}</el-tag>
          </span>
        </div>
        <div class="info-row">
          <span class="info-label">消息</span>
          <span class="info-value">{{ cleanMessage }}</span>
        </div>
      </div>

      <!-- 数据体（JSON 格式化） -->
      <div v-if="fieldsJson" class="drawer-section">
        <span class="section-label">数据体</span>
        <pre class="section-json mono">{{ fieldsJson }}</pre>
      </div>
    </div>

    <!-- 空状态 -->
    <div v-else class="drawer-empty">
      未选择日志条目
    </div>
  </el-drawer>
</template>

<style scoped>
.drawer-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  flex-shrink: 0;
}

.drawer-title {
  font-size: 16px;
  font-weight: 700;
  color: var(--foreground, #EBEBEB);
}

.drawer-info-grid {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px;
  border: 1px solid var(--border, #393939);
  border-radius: 6px;
  background: var(--muted, #222222);
  flex-shrink: 0;
}

.info-row {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
}

.info-label {
  width: 70px;
  flex-shrink: 0;
  color: var(--muted-foreground);
  font-weight: 600;
}

.info-value {
  color: var(--foreground, #EBEBEB);
  min-width: 0;
  word-break: break-all;
}

.mono {
  font-family: var(--oomol-font-mono, monospace);
  font-size: 12px;
}

.drawer-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
  min-height: 0;
}

.section-label {
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-foreground);
  flex-shrink: 0;
}

.section-json {
  font-family: var(--oomol-font-mono, monospace);
  font-size: 12px;
  line-height: 1.5;
  color: var(--foreground, #EBEBEB);
  padding: 12px;
  border: 1px solid var(--border, #393939);
  border-radius: 6px;
  background: var(--muted, #222222);
  overflow: auto;
  white-space: pre;
  flex: 1;
  min-height: 0;
  margin: 0;
}

.drawer-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--muted-foreground);
  font-size: 14px;
}

:deep(.el-drawer) {
  border-top-left-radius: 8px;
  border-top-right-radius: 8px;
}

:deep(.el-drawer__body) {
  padding: 0;
  display: flex;
  flex-direction: column;
}
</style>
