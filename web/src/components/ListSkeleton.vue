<script setup lang="ts">
defineProps<{
  rows?: number
}>()
</script>

<template>
  <div class="list-skeleton">
    <el-skeleton :rows="rows || 6" animated />
  </div>
</template>

<style scoped>
.list-skeleton {
  padding: 24px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--card);
  box-shadow: var(--console-shadow-sm);
}
</style>
