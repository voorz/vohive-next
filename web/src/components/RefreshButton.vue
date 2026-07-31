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
  <el-button :loading="loading" :disabled="disabled" @click="$emit('click')">
    <el-icon><ArrowSync24Regular /></el-icon>
    刷新
  </el-button>
</template>
