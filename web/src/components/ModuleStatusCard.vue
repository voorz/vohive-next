<script setup lang="ts">
/**
 * ModuleStatusCard - 新运行状态卡片（占位）
 */
</script>

<template>
  <div class="msc-card">
    <div class="msc-placeholder">
      <span class="msc-label">新运行状态卡片 (Gen 2) — 待实现</span>
    </div>
  </div>
</template>

<style scoped>
.msc-card {
  border-radius: 8px;
  overflow: hidden;
}

.msc-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 200px;
  background: var(--muted);
  border: 1px dashed var(--border);
  border-radius: 8px;
}

.msc-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--muted-foreground);
  letter-spacing: 0.02em;
}
</style>
