<script setup lang="ts">
import { computed } from 'vue'
import { QuestionCircle16Filled } from '@vicons/fluent'
import type { ParamAnnotation } from '../data/carrier-config-annotations'

const props = withDefaults(defineProps<{
  label: string
  section: string
  annotations: Record<string, Record<string, ParamAnnotation>>
  variant?: 'field' | 'switch'
}>(), {
  variant: 'field',
})

const annotation = computed<ParamAnnotation | null>(() => {
  const section = props.annotations[props.section]
  if (!section) return null
  return section[props.label] || null
})
</script>

<template>
  <div class="field-label-row" :class="{ 'switch-variant': variant === 'switch' }">
    <span class="form-label" :class="{ 'switch-label': variant === 'switch' }">{{ label }}</span>
    <el-popover
      v-if="annotation"
      trigger="hover"
      placement="top-start"
      :width="320"
      :show-after="200"
      popper-class="config-annotation-popover"
    >
      <template #reference>
        <span class="label-help-icon">
          <el-icon size="12"><QuestionCircle16Filled /></el-icon>
        </span>
      </template>
      <div class="annotation-content">
        <div class="annotation-desc">{{ annotation.desc }}</div>
        <div v-if="annotation.recommend" class="annotation-row">
          <span class="annotation-key">推荐</span>
          <span class="annotation-val">{{ annotation.recommend }}</span>
        </div>
        <div v-if="annotation.note" class="annotation-note">⚠️ {{ annotation.note }}</div>
        <div v-if="annotation.options" class="annotation-options">
          <div v-for="(optDesc, optVal) in annotation.options" :key="optVal" class="annotation-option">
            <code class="opt-val">{{ optVal || '空' }}</code>
            <span class="opt-desc">{{ optDesc }}</span>
          </div>
        </div>
      </div>
    </el-popover>
  </div>
</template>

<style scoped>
.field-label-row {
  display: flex;
  align-items: center;
  gap: 4px;
}

.field-label-row.switch-variant {
  gap: 6px;
}

.form-label.switch-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
  text-transform: none;
  letter-spacing: normal;
}

.label-help-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 14px;
  height: 14px;
  color: var(--muted-foreground);
  opacity: 0.5;
  cursor: help;
  transition: opacity 0.15s;
  flex-shrink: 0;
}

.label-help-icon:hover {
  opacity: 1;
  color: var(--brand);
}

.annotation-content {
  font-size: 12px;
  line-height: 1.6;
}

.annotation-desc {
  color: var(--foreground);
  margin-bottom: 8px;
}

.annotation-row {
  display: flex;
  gap: 6px;
  margin-bottom: 4px;
}

.annotation-key {
  font-weight: 700;
  color: var(--muted-foreground);
  flex-shrink: 0;
  font-size: 11px;
}

.annotation-val {
  color: var(--brand);
  font-family: var(--oomol-font-mono);
  font-size: 11px;
}

.annotation-note {
  color: var(--warning);
  font-size: 11px;
  margin-top: 6px;
  padding: 4px 6px;
  background: color-mix(in oklab, var(--warning) 8%, transparent);
  border-radius: 4px;
}

.annotation-options {
  margin-top: 6px;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.annotation-option {
  display: flex;
  gap: 6px;
  align-items: baseline;
}

.opt-val {
  font-size: 11px;
  padding: 1px 4px;
  background: var(--muted);
  border-radius: 3px;
  color: var(--foreground);
  flex-shrink: 0;
}

.opt-desc {
  font-size: 11px;
  color: var(--muted-foreground);
}
</style>
