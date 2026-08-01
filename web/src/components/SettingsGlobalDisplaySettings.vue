<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ChevronDown20Regular } from '@vicons/fluent'

const expanded = ref(true)
const editing = ref(false)

const STORAGE_KEY = 'vohive:page-max-width'
const DEFAULT_WIDTH = 1240
const MIN_WIDTH = 1024
const MAX_WIDTH = 2560

const pageWidth = ref(DEFAULT_WIDTH)

onMounted(() => {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved) {
    const v = parseInt(saved, 10)
    if (!isNaN(v) && v >= MIN_WIDTH && v <= MAX_WIDTH) {
      pageWidth.value = v
    }
  }
  applyWidth()
})

function applyWidth() {
  document.documentElement.style.setProperty('--page-max-width', pageWidth.value + 'px')
}

function onSliderChange() {
  applyWidth()
}

function startEdit() {
  editing.value = true
}

function finishEdit() {
  localStorage.setItem(STORAGE_KEY, String(pageWidth.value))
  editing.value = false
}

function resetDefault() {
  pageWidth.value = DEFAULT_WIDTH
  applyWidth()
}
</script>

<template>
  <div class="faq-card mt-4">
    <div class="faq-header" @click="expanded = !expanded">
      <span class="faq-title">显示设置</span>
      <el-icon class="faq-arrow" :class="{ expanded }" size="16">
        <ChevronDown20Regular />
      </el-icon>
    </div>
    <div v-show="expanded" class="faq-body">
      <div class="flex items-center justify-between mb-4">
        <div>
          <div class="faq-item-title">页面宽度</div>
          <div class="faq-item-desc">调整管理界面的最大内容宽度</div>
        </div>
        <div class="flex items-center gap-2">
          <span class="text-sm font-mono" style="color: var(--muted-foreground);">{{ pageWidth }}px</span>
          <el-button size="small" @click="startEdit">编辑</el-button>
        </div>
      </div>
    </div>
  </div>

  <!-- 全屏半模糊遮罩 + 居中 Slider -->
  <Teleport to="body">
    <div v-if="editing" class="display-overlay">
      <div class="display-overlay-card">
        <div class="display-overlay-title">页面宽度调整</div>
        <div class="display-overlay-desc">拖动滑块实时预览，完成后点击保存</div>
        <div class="display-slider-row">
          <span class="display-slider-label">窄</span>
          <el-slider
            v-model="pageWidth"
            :min="MIN_WIDTH"
            :max="MAX_WIDTH"
            :step="20"
            :show-tooltip="true"
            @input="onSliderChange"
            class="flex-1"
          />
          <span class="display-slider-label">宽</span>
        </div>
        <div class="display-overlay-value">{{ pageWidth }}px</div>
        <div class="flex items-center justify-center gap-3 mt-6">
          <el-button size="small" @click="resetDefault">恢复默认</el-button>
          <el-button size="default" type="primary" @click="finishEdit" class="!border-0">保存</el-button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.display-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  display: flex;
  align-items: center;
  justify-content: center;
  background: color-mix(in oklab, var(--background) 60%, transparent);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

.display-overlay-card {
  width: 480px;
  max-width: 90vw;
  padding: 32px;
  border-radius: 12px;
  background: var(--card);
  border: 1px solid var(--border);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
}

.display-overlay-title {
  font-size: 16px;
  font-weight: 700;
  color: var(--foreground);
  text-align: center;
}

.display-overlay-desc {
  font-size: 13px;
  color: var(--muted-foreground);
  text-align: center;
  margin-top: 4px;
  margin-bottom: 24px;
}

.display-slider-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.display-slider-label {
  font-size: 12px;
  color: var(--muted-foreground);
  flex-shrink: 0;
}

.display-overlay-value {
  text-align: center;
  font-family: ui-monospace, monospace;
  font-size: 24px;
  font-weight: 700;
  color: var(--brand);
  margin-top: 16px;
}
</style>
