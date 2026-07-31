<script setup lang="ts">
import { computed } from 'vue'
import { ErrorCircle24Regular } from '@vicons/fluent'

const props = defineProps<{
  title?: string
  message: string
  details?: string
  statusCode?: number
  requestMethod?: string
  requestUrl?: string
  lastSuccessAt?: number | null
  retryText?: string
}>()
const emit = defineEmits<{
  (e: 'retry'): void
}>()

const metaText = computed(() => {
  const parts: string[] = []
  if (props.statusCode) parts.push(`HTTP ${props.statusCode}`)
  const method = (props.requestMethod || '').toUpperCase()
  if (method && props.requestUrl) parts.push(`${method} ${props.requestUrl}`)
  else if (props.requestUrl) parts.push(String(props.requestUrl))
  if (props.lastSuccessAt) parts.push(`最后成功：${new Date(props.lastSuccessAt).toLocaleString()}`)
  return parts.join(' · ')
})
</script>

<template>
  <div class="inline-error">
    <div class="inline-error-icon">
      <el-icon :size="16"><ErrorCircle24Regular /></el-icon>
    </div>
    <div class="inline-error-body">
      <strong>{{ title || '加载失败' }}</strong>
      <p>{{ message }}</p>
      <div v-if="metaText" class="mono" style="margin-top: 4px; opacity: 0.7;">{{ metaText }}</div>
      <div v-if="details" class="mono" style="margin-top: 4px; opacity: 0.6; white-space: pre-wrap;">{{ details }}</div>
    </div>
    <el-button v-if="retryText" size="small" @click="emit('retry')">{{ retryText }}</el-button>
  </div>
</template>
