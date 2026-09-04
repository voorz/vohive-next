<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import LogLine, { type LogEntry } from './LogLine.vue'

const props = defineProps<{
  logs: LogEntry[]
  autoScroll?: boolean
}>()

const emit = defineEmits<{
  (e: 'open-detail', log: LogEntry): void
}>()

const ITEM_HEIGHT = 20 // 每行固定高度（px）
const BUFFER = 5 // 上下额外渲染的行数

const container = ref<HTMLElement | null>(null)
const scrollTop = ref(0)
const containerHeight = ref(0)

const totalCount = computed(() => props.logs.length)
const totalHeight = computed(() => totalCount.value * ITEM_HEIGHT)

const startIndex = computed(() => {
  return Math.max(0, Math.floor(scrollTop.value / ITEM_HEIGHT) - BUFFER)
})

const endIndex = computed(() => {
  const visible = Math.ceil(containerHeight.value / ITEM_HEIGHT)
  return Math.min(
    totalCount.value,
    Math.floor(scrollTop.value / ITEM_HEIGHT) + visible + BUFFER * 2
  )
})

const visibleLogs = computed(() => {
  return props.logs.slice(startIndex.value, endIndex.value)
})

const offsetY = computed(() => startIndex.value * ITEM_HEIGHT)

function onScroll() {
  if (!container.value) return
  scrollTop.value = container.value.scrollTop
  // 如果开启了自动追尾且滚动到底部，保持
}

function scrollToBottom() {
  if (!container.value) return
  container.value.scrollTop = totalHeight.value
}

function onResize() {
  if (!container.value) return
  containerHeight.value = container.value.clientHeight
}

let resizeObserver: ResizeObserver | null = null

onMounted(() => {
  if (container.value) {
    containerHeight.value = container.value.clientHeight
    resizeObserver = new ResizeObserver(() => onResize())
    resizeObserver.observe(container.value)
  }
})

onUnmounted(() => {
  resizeObserver?.disconnect()
})

// 自动追尾：当 logs 变化且开关打开时滚到底
watch(() => props.logs.length, () => {
  if (props.autoScroll) {
    nextTick(scrollToBottom)
  }
})

defineExpose({ scrollToBottom })
</script>

<template>
  <div
    ref="container"
    class="virtual-log-list"
    @scroll="onScroll"
  >
    <!-- 空状态 -->
    <div v-if="totalCount === 0" class="virtual-empty">
      <slot name="empty" />
    </div>
    <!-- 占位撑开总高度 -->
    <div v-else class="virtual-spacer" :style="{ height: `${totalHeight}px` }">
      <!-- 实际渲染的日志行 -->
      <div
        class="virtual-items"
        :style="{ transform: `translateY(${offsetY}px)` }"
      >
        <LogLine
          v-for="(log, idx) in visibleLogs"
          :key="startIndex + idx"
          :log="log"
          @open-detail="emit('open-detail', $event)"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.virtual-log-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: auto;
  font-family: var(--oomol-font-sans);
  font-size: 12px;
  background: #000000;
  color: #e0e0e0;
  padding: 0 8px;
}

.virtual-spacer {
  position: relative;
  width: 100%;
}

.virtual-items {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
}

.virtual-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  min-height: 200px;
}
</style>
