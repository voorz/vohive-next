<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount } from 'vue'
import { CheckmarkCircle24Filled, DismissCircle24Filled, Spinner24Regular } from '@vicons/fluent'

const props = defineProps<{
  visible: boolean
  progress: number
  message: string
  error: string
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

const progressDisplay = computed(() => {
  if (status.value === 'done') return '100%'
  return `${Math.min(props.progress, 99)}%`
})

// ESC 键关闭（仅完成/失败状态可关闭）
function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.visible && status.value !== 'downloading') {
    emit('close')
  }
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <Transition name="overlay-fade">
    <div v-if="visible" class="download-overlay" @click.self="status !== 'downloading' && emit('close')">
      <div class="overlay-card">
        <!-- 图标 -->
        <div class="overlay-icon-box">
          <el-icon v-if="status === 'downloading'" size="32" class="overlay-spinner">
            <Spinner24Regular />
          </el-icon>
          <el-icon v-else-if="status === 'done'" size="32" class="overlay-icon-done">
            <CheckmarkCircle24Filled />
          </el-icon>
          <el-icon v-else size="32" class="overlay-icon-error">
            <DismissCircle24Filled />
          </el-icon>
        </div>

        <!-- 标题 -->
        <div class="overlay-title">
          {{ status === 'downloading' ? '正在下载 eSIM Profile' : status === 'done' ? '下载完成' : '下载失败' }}
        </div>

        <!-- 批量计数 -->
        <div v-if="batchTotal && batchTotal > 1" class="overlay-batch-count">
          {{ batchCurrent }} / {{ batchTotal }}
        </div>

        <!-- 进度条 -->
        <div class="overlay-progress-track">
          <div
            class="overlay-progress-fill"
            :class="{ error: status === 'error', done: status === 'done' }"
            :style="{ width: progressDisplay }"
          />
        </div>

        <!-- 状态文字 -->
        <div class="overlay-message" :class="{ error: status === 'error' }">
          {{ error || message || progressDisplay }}
        </div>

        <!-- 关闭按钮（非下载中显示） -->
        <button
          v-if="status !== 'downloading'"
          class="overlay-close-btn"
          @click="emit('close')"
        >
          关闭
        </button>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.download-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.25);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

.overlay-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  padding: 32px 40px;
  border-radius: 12px;
  background: var(--card, #fff);
  border: 1px solid var(--border, #e4e4e7);
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.15);
  min-width: 360px;
  max-width: 480px;
}

.overlay-icon-box {
  display: flex;
  align-items: center;
  justify-content: center;
}

.overlay-spinner {
  color: var(--brand, #00bc7d);
  animation: overlay-spin 0.8s linear infinite;
}

@keyframes overlay-spin {
  to { transform: rotate(360deg); }
}

.overlay-icon-done {
  color: var(--brand, #00bc7d);
}

.overlay-icon-error {
  color: #ef4444;
}

.overlay-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--foreground, #18181b);
}

.overlay-batch-count {
  font-size: 12px;
  font-weight: 600;
  color: var(--muted-foreground, #71717a);
}

.overlay-progress-track {
  width: 100%;
  height: 8px;
  border-radius: 999px;
  background: var(--muted, #f4f4f5);
  overflow: hidden;
}

.overlay-progress-fill {
  height: 100%;
  border-radius: 999px;
  background: var(--brand, #00bc7d);
  transition: width 0.3s ease;
}

.overlay-progress-fill.error {
  background: #ef4444;
}

.overlay-progress-fill.done {
  background: var(--brand, #00bc7d);
}

.overlay-message {
  font-size: 12px;
  color: var(--muted-foreground, #71717a);
  text-align: center;
  word-break: break-word;
  max-width: 100%;
}

.overlay-message.error {
  color: #ef4444;
}

.overlay-close-btn {
  padding: 6px 20px;
  border: 1px solid var(--border, #e4e4e7);
  border-radius: 6px;
  background: transparent;
  color: var(--foreground, #18181b);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.12s;
}

.overlay-close-btn:hover {
  background: var(--muted, #f4f4f5);
}

/* Transition */
.overlay-fade-enter-active,
.overlay-fade-leave-active {
  transition: opacity 0.2s ease;
}

.overlay-fade-enter-from,
.overlay-fade-leave-to {
  opacity: 0;
}
</style>
