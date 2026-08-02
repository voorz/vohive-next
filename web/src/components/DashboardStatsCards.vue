<script setup lang="ts">
import { computed } from 'vue'
import { Phone24Regular, CheckmarkCircle24Regular, DismissCircle24Regular, ArrowSync24Regular } from '@vicons/fluent'

const props = defineProps<{
  total: number
  online: number
  offline: number
  lastUpdatedAt: number | null
}>()

const lastRefreshText = computed(() => {
  if (!props.lastUpdatedAt) return '--:--:--'
  return new Date(props.lastUpdatedAt).toLocaleTimeString()
})
</script>

<template>
  <div class="stats-cards-container">
    <!-- 设备总数 -->
    <div class="stats-card">
      <div class="stats-icon-box">
        <el-icon size="20"><Phone24Regular /></el-icon>
      </div>
      <div class="stats-info">
        <span class="stats-label">设备总数</span>
        <strong class="stats-value">{{ total }}</strong>
      </div>
    </div>

    <!-- 在线 -->
    <div class="stats-card">
      <div class="stats-icon-box">
        <el-icon size="20"><CheckmarkCircle24Regular /></el-icon>
      </div>
      <div class="stats-info">
        <span class="stats-label">在线</span>
        <strong class="stats-value stats-value-success">{{ online }}</strong>
      </div>
    </div>

    <!-- 离线 -->
    <div class="stats-card">
      <div class="stats-icon-box">
        <el-icon size="20"><DismissCircle24Regular /></el-icon>
      </div>
      <div class="stats-info">
        <span class="stats-label">离线</span>
        <strong class="stats-value stats-value-danger">{{ offline }}</strong>
      </div>
    </div>

    <!-- 最近刷新 -->
    <div class="stats-card">
      <div class="stats-icon-box">
        <el-icon size="20"><ArrowSync24Regular /></el-icon>
      </div>
      <div class="stats-info">
        <span class="stats-label">最近刷新</span>
        <strong class="stats-value stats-value-time">{{ lastRefreshText }}</strong>
      </div>
    </div>
  </div>
</template>

<style scoped>
.stats-cards-container {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0;
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
  background: var(--card);
  min-width: 0;
}

.stats-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px;
  border-right: 1px solid var(--border);
}

.stats-card:last-child {
  border-right: none;
}

.stats-icon-box {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border-radius: 6px;
  flex-shrink: 0;
  border: 1px solid var(--border);
  background: var(--card);
}

.stats-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.stats-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--muted-foreground);
}

.stats-value {
  font-size: 22px;
  font-weight: 700;
  color: var(--foreground);
  line-height: 1.2;
}

.stats-value-success {
  color: var(--success);
}

.stats-value-danger {
  color: var(--destructive);
}

.stats-value-time {
  font-size: 14px;
  font-weight: 600;
}

@media (max-width: 768px) {
  .stats-cards-container {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .stats-card:nth-child(2) {
    border-right: none;
  }

  .stats-card:nth-child(1),
  .stats-card:nth-child(2) {
    border-bottom: 1px solid var(--border);
  }
}
</style>
