<script setup lang="ts">
</script>

<template>
  <div class="logs-network-tab">
    <div class="network-placeholder">
      <div class="network-empty">
        <div class="network-empty-icon">
          <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20z" />
            <path d="M2 12h20M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" />
          </svg>
        </div>
        <div class="network-empty-title">网络请求查看器</div>
        <div class="network-empty-desc">该功能正在开发中，敬请期待</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.logs-network-tab {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.network-placeholder {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #000000;
  padding: 24px;
}

.network-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.network-empty-icon {
  color: var(--muted-foreground);
  opacity: 0.4;
}

.network-empty-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--muted-foreground);
}

.network-empty-desc {
  font-size: 13px;
  color: var(--muted-foreground);
  opacity: 0.7;
}
</style>
