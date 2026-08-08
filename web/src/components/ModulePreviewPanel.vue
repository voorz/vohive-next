<script setup lang="ts">
import { Shield24Regular } from '@vicons/fluent'
</script>

<template>
  <div class="module-preview-panel">
    <!-- 预览头部 (60px) -->
    <div class="preview-header">
      <div class="preview-header-left">
        <div class="preview-header-icon">
          <el-icon size="16"><Shield24Regular /></el-icon>
        </div>
        <div class="preview-title">预览</div>
      </div>
    </div>

    <!-- 预览内容（占位） -->
    <div class="preview-body">
      <div class="placeholder-hint">
        预览内容将在此显示
      </div>
    </div>
  </div>
</template>

<style scoped>
.module-preview-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--card);
  overflow: hidden;
}

.preview-header {
  height: 60px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 14px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.preview-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.preview-header-icon {
  width: 38px;
  height: 38px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  background: var(--background);
  border: 1px solid var(--border);
  color: var(--foreground);
}

.preview-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--foreground);
}

.preview-body {
  flex: 1;
  min-height: 0;
  overflow: auto;
}

.placeholder-hint {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--muted-foreground);
  font-size: 13px;
}
</style>
