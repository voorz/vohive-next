<script setup lang="ts">
import { computed } from 'vue'
import { CheckmarkCircle24Filled, DismissCircle24Filled, SpinnerIos20Regular } from '@vicons/fluent'

const props = defineProps<{
  visible: boolean
  progress: number
  message: string
  error: string
  errorCode?: string
  errorDetails?: string
  subjectCode?: string
  reasonCode?: string
  subjectIdentifier?: string
  batchCurrent?: number
  batchTotal?: number
}>()

const emit = defineEmits<{
  'close': []
}>()

const status = computed<'downloading' | 'done' | 'error'>(() => {
  if (props.error) return 'error'
  if (props.progress >= 100) return 'done'
  return 'downloading'
})

const safeProgress = computed(() => Math.max(0, Math.min(props.progress, 100)))

const canClose = computed(() => status.value !== 'downloading')

// SM-DP+ 错误码行：[SM-DP+ Error (subjectCode, reasonCode)]
const smdpErrorCodeLine = computed(() => {
  if (!props.subjectCode && !props.reasonCode) return ''
  return `[SM-DP+ Error (${props.subjectCode || 'N/A'}, ${props.reasonCode || 'N/A'})]`
})

// SM-DP+ Identifier 行：[Identifier: xxx]
const smdpIdentifierLine = computed(() => {
  if (!props.subjectIdentifier) return ''
  return `[Identifier: ${props.subjectIdentifier}]`
})

// 弹窗标题始终不变
const titleText = '下载 eSIM Profile'
</script>

<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="(v: boolean) => { if (!v && canClose) emit('close') }"
    :title="titleText"
    width="min(440px, 90vw)"
    :close-on-click-modal="false"
    :close-on-press-escape="canClose"
    :show-close="canClose"
    :align-center="true"
  >
    <div class="dl-body">
      <!-- 图标 -->
      <el-icon v-if="status === 'downloading'" size="32" class="dl-spinner">
        <SpinnerIos20Regular />
      </el-icon>
      <el-icon v-else-if="status === 'done'" size="32" class="dl-done-icon">
        <CheckmarkCircle24Filled />
      </el-icon>
      <el-icon v-else size="32" class="dl-error-icon">
        <DismissCircle24Filled />
      </el-icon>

      <!-- 百分比（下载中/成功时显示，失败时隐藏） -->
      <div v-if="status !== 'error'" class="dl-pct">
        {{ status === 'done' ? '100%' : `${safeProgress}%` }}
      </div>

      <!-- 进度条（下载中/成功时显示，失败时隐藏） -->
      <el-progress
        v-if="status !== 'error'"
        :percentage="status === 'done' ? 100 : safeProgress"
        :status="status === 'done' ? 'success' : undefined"
        :stroke-width="6"
        :show-text="false"
      />

      <!-- 状态消息 -->
      <div v-if="status === 'error'" class="dl-status-text">
        下载失败
      </div>
      <div v-else class="dl-status-text">
        {{ message }}
      </div>

      <!-- 批量计数 -->
      <div v-if="batchTotal && batchTotal > 1" class="dl-batch">
        {{ batchCurrent }} / {{ batchTotal }}
      </div>

      <!-- 错误信息卡片 -->
      <div v-if="status === 'error' && error" class="dl-error-card">
        <div class="dl-error-card-header">ERROR</div>
        <div v-if="smdpErrorCodeLine" class="dl-error-code-line">{{ smdpErrorCodeLine }}</div>
        <div class="dl-error-msg">{{ error }}</div>
        <div v-if="smdpIdentifierLine" class="dl-error-identifier-line">{{ smdpIdentifierLine }}</div>
      </div>

      <!-- 下载中提示 -->
      <div v-if="status === 'downloading'" class="dl-hint">
        请勿关闭此窗口，下载完成后将自动关闭
      </div>
    </div>

    <template #footer>
      <el-button
        v-if="canClose"
        size="default"
        @click="emit('close')"
      >
        关闭
      </el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.dl-body {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
}

.dl-spinner {
  color: var(--brand);
  animation: dl-spin 0.8s linear infinite;
}

@keyframes dl-spin {
  to { transform: rotate(360deg); }
}

.dl-done-icon {
  color: var(--brand);
}

.dl-error-icon {
  color: var(--destructive);
}

.dl-pct {
  font-size: 22px;
  font-weight: 700;
  color: var(--foreground);
  line-height: 1;
}

.dl-status-text {
  font-size: 13px;
  color: var(--muted-foreground);
  text-align: center;
}

.dl-batch {
  font-size: 12px;
  font-weight: 600;
  color: var(--muted-foreground);
}

.dl-hint {
  font-size: 11px;
  color: var(--muted-foreground);
  text-align: center;
  opacity: 0.7;
}

.dl-error-card {
  width: 100%;
  border: 1px solid var(--border);
  border-radius: var(--radius-md, 6px);
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  background: var(--muted);
}

.dl-error-card-header {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 1px;
  color: var(--destructive);
}

.dl-error-msg {
  font-size: 12px;
  color: var(--foreground);
  text-align: center;
  line-height: 1.5;
  word-break: break-word;
}

.dl-error-code-line {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-foreground);
  text-align: center;
  line-height: 1.5;
}

.dl-error-identifier-line {
  font-size: 11px;
  color: var(--muted-foreground);
  text-align: center;
  line-height: 1.5;
  opacity: 0.8;
}
</style>
