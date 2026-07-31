<script setup lang="ts">
defineProps<{
  title: string
  subtitle?: string
}>()
</script>

<template>
  <div class="empty-state">
    <div class="empty-icon">
      <slot name="icon">
        <span>∅</span>
      </slot>
    </div>
    <strong>{{ title }}</strong>
    <p v-if="subtitle">{{ subtitle }}</p>
    <div class="empty-actions">
      <slot name="actions" />
    </div>
  </div>
</template>
