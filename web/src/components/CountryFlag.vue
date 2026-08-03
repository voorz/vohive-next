<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { getCachedFlag, getOrDownloadFlag } from '../composables/useCountryFlag'

const props = defineProps<{
  iso: string
  size?: number
}>()

const src = ref<string | null>(null)
const loading = ref(false)

async function refresh() {
  if (!props.iso) {
    src.value = null
    return
  }
  const cached = getCachedFlag(props.iso)
  if (cached) {
    src.value = cached
    return
  }
  loading.value = true
  const result = await getOrDownloadFlag(props.iso)
  src.value = result
  loading.value = false
}

watch(() => props.iso, refresh, { immediate: true })

const iconSize = computed(() => props.size ?? 20)
</script>

<template>
  <div class="country-flag-box" :style="{ width: `${iconSize}px`, height: `${iconSize}px` }">
    <img
      v-if="src"
      :src="src"
      class="country-flag-img"
      alt="flag"
    />
    <div v-else class="country-flag-placeholder">
      {{ iso?.toUpperCase()?.slice(0, 2) || '??' }}
    </div>
  </div>
</template>

<style scoped>
.country-flag-box {
  flex-shrink: 0;
  border-radius: 3px;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
}

.country-flag-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.country-flag-placeholder {
  font-size: 10px;
  font-weight: 600;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
}
</style>
