<script setup lang="ts">
import { ref, computed } from 'vue'
import { Dismiss12Filled, AddCircle20Filled } from '@vicons/fluent'

/**
 * Tag 输入 + 预设下拉面板
 * - 支持手动输入文本，回车添加为 tag
 * - 聚焦时展开预设面板，点击预设 tag 直接添加/取消
 * - 已选的预设 tag 在面板中高亮标记
 */

const props = withDefaults(defineProps<{
  /** 当前选中的值列表 */
  modelValue: string[]
  /** 预设选项 { value: label } */
  presets?: Record<string, string>
  /** 占位文本 */
  placeholder?: string
  /** 输入框宽度（默认 100%） */
  fullWidth?: boolean
}>(), {
  presets: () => ({}),
  placeholder: '',
  fullWidth: true,
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string[]): void
}>()

// ---- 内部状态 ----
const inputVisible = ref(false)   // 预设面板是否展开
const inputText = ref('')           // 当前正在输入的文本
const inputRef = ref<HTMLInputElement | null>(null)

// ---- 当前选中的值（去重） ----
const selectedTags = computed({
  get: () => props.modelValue || [],
  set: (val: string[]) => emit('update:modelValue', val),
})

// ---- 预设列表 ----
const presetList = computed(() => {
  return Object.entries(props.presets).map(([value, label]) => ({ value, label }))
})

// ---- 过滤后的预设（排除已选的） ----
const availablePresets = computed(() => {
  return presetList.value.filter(p => !selectedTags.value.includes(p.value))
})

// ---- 是否有预设可用 ----
const hasPresets = computed(() => presetList.value.length > 0)

// ---- 添加 tag ----
function addTag(value: string) {
  const v = value.trim()
  if (!v) return
  if (!selectedTags.value.includes(v)) {
    selectedTags.value = [...selectedTags.value, v]
  }
  inputText.value = ''
}

// ---- 移除 tag ----
function removeTag(value: string) {
  selectedTags.value = selectedTags.value.filter(t => t !== value)
}

// ---- 切换预设 tag ----
function togglePreset(value: string) {
  if (selectedTags.value.includes(value)) {
    removeTag(value)
  } else {
    addTag(value)
  }
}

// ---- 输入框聚焦 ----
function onFocus() {
  inputVisible.value = true
}

// ---- 输入框失焦（延迟关闭面板，允许点击面板内 tag） ----
function onBlur() {
  setTimeout(() => {
    // 如果还有未提交的文本，先添加
    if (inputText.value.trim()) {
      addTag(inputText.value)
    }
    inputVisible.value = false
  }, 200)
}

// ---- 回车确认 ----
function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter') {
    e.preventDefault()
    if (inputText.value.trim()) {
      addTag(inputText.value)
    }
  } else if (e.key === 'Backspace' && !inputText.value && selectedTags.value.length > 0) {
    // 空输入时 Backspace 删除最后一个 tag
    removeTag(selectedTags.value[selectedTags.value.length - 1])
  }
}

// ---- 面板点击不失焦 ----
function onPanelMousedown(e: MouseEvent) {
  e.preventDefault()
}
</script>

<template>
  <div class="tag-input-wrap" :class="{ 'full-width': fullWidth }">
    <div class="tag-input-box" :class="{ focused: inputVisible }">
      <!-- 已选 tag 列表 -->
      <span
        v-for="tag in selectedTags"
        :key="tag"
        class="tag-chip"
      >
        <span class="tag-text">{{ tag }}</span>
        <el-icon class="tag-close" size="10" @click="removeTag(tag)"><Dismiss12Filled /></el-icon>
      </span>
      <!-- 输入框 -->
      <input
        ref="inputRef"
        v-model="inputText"
        class="tag-input-field"
        :placeholder="selectedTags.length === 0 ? placeholder : ''"
        @focus="onFocus"
        @blur="onBlur"
        @keydown="onKeydown"
      />
    </div>

    <!-- 预设下拉面板 -->
    <transition name="preset-panel">
      <div
        v-if="inputVisible && hasPresets"
        class="preset-panel"
        @mousedown="onPanelMousedown"
      >
        <div v-if="availablePresets.length === 0" class="preset-empty">
          全部预设已添加
        </div>
        <template v-else>
          <div class="preset-section-title">点击添加预设</div>
          <div class="preset-tags">
            <span
              v-for="p in availablePresets"
              :key="p.value"
              class="preset-chip"
              @click="togglePreset(p.value)"
            >
              <el-icon size="10" class="preset-add-icon"><AddCircle20Filled /></el-icon>
              <span class="preset-label">{{ p.label }}</span>
              <code class="preset-val">{{ p.value }}</code>
            </span>
          </div>
        </template>
      </div>
    </transition>

  </div>
</template>

<style scoped>
.tag-input-wrap {
  position: relative;
  display: inline-flex;
  flex-direction: column;
  width: 100%;
}

.tag-input-wrap.full-width {
  width: 100%;
}

.tag-input-box {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
  min-height: 32px;
  padding: 4px 8px;
  border: 1px solid var(--el-border-color, #dcdfe6);
  border-radius: 4px;
  background: var(--el-fill-color-blank, #fff);
  cursor: text;
  transition: border-color 0.2s;
}

.tag-input-box.focused {
  border-color: var(--el-color-primary, #409eff);
  box-shadow: 0 0 0 1px var(--el-color-primary, #409eff) inset;
}

.tag-chip {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 2px 6px;
  background: var(--el-color-primary-light-9, #ecf5ff);
  border: 1px solid var(--el-color-primary-light-7, #d9ecff);
  border-radius: 3px;
  font-size: 11px;
  color: var(--el-color-primary, #409eff);
  line-height: 1.4;
  white-space: nowrap;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tag-text {
  overflow: hidden;
  text-overflow: ellipsis;
}

.tag-close {
  cursor: pointer;
  opacity: 0.6;
  flex-shrink: 0;
  transition: opacity 0.15s;
}

.tag-close:hover {
  opacity: 1;
}

.tag-input-field {
  flex: 1;
  min-width: 60px;
  border: none;
  outline: none;
  background: transparent;
  font-size: 12px;
  color: var(--el-text-color-primary);
  line-height: 1.5;
  padding: 0;
}

.tag-input-field::placeholder {
  color: var(--el-text-color-placeholder, #a8abb2);
  font-size: 12px;
}

/* 预设下拉面板 */
.preset-panel {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  z-index: 2000;
  background: var(--el-bg-color, #fff);
  border: 1px solid var(--el-border-color-light, #e4e7ed);
  border-radius: 6px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
  padding: 8px;
  max-height: 240px;
  overflow-y: auto;
}

.preset-empty {
  text-align: center;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  padding: 12px 0;
}

.preset-section-title {
  font-size: 10px;
  font-weight: 700;
  color: var(--el-text-color-secondary);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  margin-bottom: 6px;
}

.preset-tags {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.preset-chip {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  border-radius: 4px;
  cursor: pointer;
  transition: background 0.15s;
  font-size: 11px;
}

.preset-chip:hover {
  background: var(--el-color-primary-light-9, #ecf5ff);
}

.preset-add-icon {
  color: var(--el-color-primary, #409eff);
  flex-shrink: 0;
}

.preset-label {
  color: var(--el-text-color-primary);
  font-weight: 500;
}

.preset-val {
  font-family: var(--el-font-mono, monospace);
  font-size: 10px;
  color: var(--el-text-color-secondary);
  margin-left: auto;
  padding: 1px 4px;
  background: var(--el-fill-color-light, #f5f7fa);
  border-radius: 3px;
}

/* 过渡动画 */
.preset-panel-enter-active,
.preset-panel-leave-active {
  transition: opacity 0.15s, transform 0.15s;
}

.preset-panel-enter-from,
.preset-panel-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
