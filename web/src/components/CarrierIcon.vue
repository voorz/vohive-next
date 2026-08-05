<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from 'vue'
import { getCachedIcon, isPersonalizationEnabled } from '../composables/useOperatorIcon'

const props = defineProps<{
  mcc: string
  mnc: string
  name?: string
  size?: number
}>()

const cachedSrc = ref<string | null>(null)
const enabled = ref(true)

function refresh() {
  enabled.value = isPersonalizationEnabled()
  cachedSrc.value = enabled.value ? getCachedIcon(props.mcc, props.mnc, props.name) : null
}

watch(() => [props.mcc, props.mnc, props.name], refresh, { immediate: true })

// 监听图标下载完成事件，刷新缓存
function onIconUpdated(e: Event) {
  const detail = (e as CustomEvent).detail
  if (detail && detail.mcc === props.mcc && detail.mnc === props.mnc) {
    refresh()
  }
}

onMounted(() => window.addEventListener('vohive-icon-updated', onIconUpdated))
onUnmounted(() => window.removeEventListener('vohive-icon-updated', onIconUpdated))

const iconSize = computed(() => props.size ?? 28)
</script>

<template>
  <div class="carrier-icon-box" :style="{ width: `${iconSize}px`, height: `${iconSize}px` }">
    <img
      v-if="cachedSrc"
      :src="cachedSrc"
      class="carrier-icon-img"
      alt="operator icon"
    />
    <svg
      v-else
      class="carrier-icon-default"
      :width="iconSize"
      :height="iconSize"
      viewBox="0 0 24 24"
      fill="none"
    >
      <rect x="3" y="5" width="18" height="14" rx="2" stroke="currentColor" stroke-width="1.5" />
      <circle cx="8" cy="10" r="1.5" fill="currentColor" opacity="0.5" />
      <path d="M3 16l4-4 3 3 4-4 7 6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" opacity="0.5" />
    </svg>
  </div>
</template>

<style scoped>
.carrier-icon-box {
  flex-shrink: 0;
  border-radius: 6px;
  border: 1px solid var(--border);
  background: var(--muted);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.carrier-icon-img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.carrier-icon-default {
  color: var(--muted-foreground);
  opacity: 0.6;
}
</style>
