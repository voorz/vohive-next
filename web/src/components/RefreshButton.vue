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
  <el-button :disabled="disabled || loading" class="refresh-btn" @click="$emit('click')">
    <el-icon :class="{ 'is-spinning': loading }"><ArrowSync24Regular /></el-icon>
    刷新
  </el-button>
</template>

<style scoped>
.refresh-btn {
  min-width: 80px;
  justify-content: center;
}

.is-spinning {
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
