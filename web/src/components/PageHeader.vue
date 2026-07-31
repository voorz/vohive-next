<script setup lang="ts">
// 标题已由顶栏 (shell-header) 统一显示，此处仅保留操作按钮区域
</script>

<template>
  <div class="page-header-actions">
    <slot name="actions" />
  </div>
</template>

<style scoped>
.page-header-actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}
</style>
