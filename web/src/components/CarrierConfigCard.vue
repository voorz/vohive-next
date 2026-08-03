<script setup lang="ts">
import { computed } from 'vue'
import {
  Shield24Regular,
  Person24Regular,
  CheckmarkCircle24Regular,
  CloudOff24Regular
} from '@vicons/fluent'

const props = defineProps<{
  type: 'system' | 'user'
  active: boolean
  hasConfig: boolean
  selected: boolean
}>()

const emit = defineEmits<{
  click: []
}>()

const isSystem = computed(() => props.type === 'system')

const statusLabel = computed(() => {
  if (isSystem.value) {
    return props.active ? '使用中' : '备用'
  }
  return props.active ? '使用中' : '空闲'
})

const titleLabel = computed(() => isSystem.value ? '系统默认' : '用户配置')

const icon = isSystem.value ? Shield24Regular : Person24Regular
</script>

<template>
  <div
    class="config-card"
    :class="{
      'is-system': isSystem,
      'is-user': !isSystem,
      'is-selected': selected,
      'is-active': active && hasConfig
    }"
    @click="emit('click')"
  >
    <div class="config-card-icon">
      <el-icon size="20"><component :is="icon" /></el-icon>
    </div>
    <div class="config-card-body">
      <div class="config-card-title">{{ titleLabel }}</div>
      <div class="config-card-status">
        <span v-if="active && hasConfig" class="status-dot active" />
        <span v-else class="status-dot idle" />
        <span>{{ statusLabel }}</span>
      </div>
    </div>
    <div v-if="!isSystem && !hasConfig" class="config-card-empty">
      <el-icon size="14"><CloudOff24Regular /></el-icon>
    </div>
    <div v-else-if="selected" class="config-card-check">
      <el-icon size="14"><CheckmarkCircle24Regular /></el-icon>
    </div>
  </div>
</template>

<style scoped>
.config-card {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--card);
  cursor: pointer;
  transition: border-color 0.15s, box-shadow 0.15s, background 0.15s;
  flex: 1;
  min-width: 0;
}

.config-card:hover {
  border-color: color-mix(in oklab, var(--brand) 40%, var(--border));
  box-shadow: var(--console-shadow-sm);
}

.config-card.is-selected {
  border-color: var(--brand);
  background: color-mix(in oklab, var(--brand) 5%, var(--card));
}

.config-card.is-active {
  border-color: color-mix(in oklab, var(--success) 40%, var(--border));
}

.config-card-icon {
  width: 38px;
  height: 38px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.is-system .config-card-icon {
  background: var(--background);
  border: 1px solid var(--border);
  color: var(--foreground);
}

.is-user .config-card-icon {
  background: var(--background);
  border: 1px solid var(--border);
  color: var(--foreground);
}

.config-card-body {
  flex: 1;
  min-width: 0;
}

.config-card-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--foreground);
}

.config-card-status {
  display: flex;
  align-items: center;
  gap: 5px;
  margin-top: 2px;
  font-size: 12px;
  color: var(--muted-foreground);
}

.status-dot {
  width: 7px;
  height: 7px;
  border-radius: 999px;
  flex-shrink: 0;
}

.status-dot.active {
  background: var(--success);
  box-shadow: 0 0 0 2px color-mix(in oklab, var(--success) 25%, transparent);
}

.status-dot.idle {
  background: var(--muted-foreground);
  opacity: 0.4;
}

.config-card-empty {
  color: var(--muted-foreground);
  opacity: 0.5;
  flex-shrink: 0;
}

.config-card-check {
  color: var(--brand);
  flex-shrink: 0;
}
</style>
