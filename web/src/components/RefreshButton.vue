<script setup lang="ts">
import { ArrowSync24Regular } from '@vicons/fluent'

defineProps<{
  loading?: boolean
  disabled?: boolean
}>()

defineEmits<{
  (e: 'click'): void
}>()
</script>

<template>
  <button
    type="button"
    class="refresh-icon-btn"
    :disabled="disabled || loading"
    title="刷新"
    @click="$emit('click')"
  >
    <component :is="ArrowSync24Regular" class="refresh-icon" :class="{ spin: loading }" />
    <span class="refresh-label">刷新</span>
  </button>
</template>

<style scoped>
.refresh-icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 30px;
  padding: 0 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--background);
  box-shadow: var(--console-shadow-sm);
  color: var(--foreground);
  cursor: pointer;
  transition: background 150ms ease, transform 150ms ease;
  font-size: 13px;
}

.refresh-icon-btn:hover {
  background: var(--accent);
}

.refresh-icon-btn:active {
  transform: translateY(0.5px);
}

.refresh-icon-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.refresh-icon {
  width: 15px;
  height: 15px;
  flex-shrink: 0;
}

.spin {
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
