<script setup lang="ts">
import { ElButton, ElButtonGroup } from 'element-plus'

export type LogsTab = 'history' | 'realtime' | 'network'

defineProps<{
  modelValue: LogsTab
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', tab: LogsTab): void
}>()

const tabs: { id: LogsTab; label: string }[] = [
  { id: 'history',  label: '历史' },
  { id: 'realtime', label: '实时' },
  { id: 'network',  label: '网络' },
]
</script>

<template>
  <div class="logs-tab-switcher">
    <el-button-group>
      <el-button
        v-for="tab in tabs"
        :key="tab.id"
        :type="modelValue === tab.id ? 'primary' : ''"
        @click="emit('update:modelValue', tab.id)"
      >
        {{ tab.label }}
      </el-button>
    </el-button-group>
  </div>
</template>

<style scoped>
.logs-tab-switcher {
  display: flex;
  align-items: center;
  flex-shrink: 0;
}
</style>
