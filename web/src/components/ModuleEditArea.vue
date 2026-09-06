<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import {
  Save24Regular,
  Delete24Regular,
  DocumentEdit24Regular,
  Code24Regular,
  ArrowUpRight24Regular,
  DocumentAdd24Regular,
  Phone24Regular
} from '@vicons/fluent'

// 编辑模式: 'param' | 'code'
const editMode = ref<'param' | 'code'>('param')

// 是否有未保存更改
const dirty = ref(false)

// 是否正在保存
const saving = ref(false)

// 模拟配置是否存在（UI占位，后续接API替换）
const hasConfig = ref(false)

// 代码模式 JSON 文本（占位）
const codeText = ref('{}')

// 主题跟随
const isDark = ref(document.documentElement.classList.contains('dark'))
let themeObserver: MutationObserver | null = null
onMounted(() => {
  themeObserver = new MutationObserver(() => {
    isDark.value = document.documentElement.classList.contains('dark')
  })
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
})
onUnmounted(() => themeObserver?.disconnect())

function switchMode(mode: 'param' | 'code') {
  editMode.value = mode
}

// 占位操作（后续接API替换）
function handleSave() {
  saving.value = true
  setTimeout(() => {
    saving.value = false
    dirty.value = false
  }, 500)
}

function handleDelete() {
  hasConfig.value = false
  dirty.value = false
}

function handleToggleActive() {
  // 占位
}

function createFromTemplate() {
  hasConfig.value = true
  dirty.value = true
}
</script>

<template>
  <div class="edit-area">
    <!-- 无配置状态 -->
    <div v-if="!hasConfig" class="edit-empty">
      <div class="edit-empty-icon">
        <el-icon size="28"><Phone24Regular /></el-icon>
      </div>
      <div class="edit-empty-title">尚未创建模块配置</div>
      <div class="edit-empty-desc">从默认模板或手动创建配置</div>
      <div class="edit-empty-actions">
        <el-button type="primary" @click="createFromTemplate" class="!border-0">
          <el-icon class="mr-1.5"><ArrowUpRight24Regular /></el-icon>
          <span>从默认模板创建</span>
        </el-button>
        <el-button @click="createFromTemplate">
          <el-icon class="mr-1.5"><DocumentAdd24Regular /></el-icon>
          <span>手动创建</span>
        </el-button>
      </div>
    </div>

    <!-- 有配置：编辑区 -->
    <div v-else class="edit-content">
      <!-- 编辑区头部 -->
      <div class="edit-header">
        <div class="edit-header-left">
          <div class="edit-header-icon">
            <el-icon size="16"><Phone24Regular /></el-icon>
          </div>
          <span class="edit-header-label">模块配置</span>
          <span v-if="dirty" class="dirty-badge">未保存</span>
        </div>
        <div class="edit-header-right">
          <div class="activate-toggle">
            <span class="activate-label">启用</span>
            <el-switch :model-value="true" @change="handleToggleActive" />
          </div>
          <el-button size="small" @click="handleDelete" type="danger" plain>
            <el-icon><Delete24Regular /></el-icon>
          </el-button>
          <el-button size="small" type="primary" :loading="saving" :disabled="!dirty" @click="handleSave" class="!border-0">
            <el-icon class="mr-1"><Save24Regular /></el-icon>
            <span>保存</span>
          </el-button>
        </div>
      </div>

      <!-- 模式切换 -->
      <div class="mode-tabs">
        <button
          class="mode-tab"
          :class="{ active: editMode === 'param' }"
          @click="switchMode('param')"
        >
          <el-icon size="14"><DocumentEdit24Regular /></el-icon>
          <span>参数模式</span>
        </button>
        <button
          class="mode-tab"
          :class="{ active: editMode === 'code' }"
          @click="switchMode('code')"
        >
          <el-icon size="14"><Code24Regular /></el-icon>
          <span>代码模式</span>
        </button>
      </div>

      <!-- 编辑内容 -->
      <div class="edit-body">
        <!-- 参数模式（占位） -->
        <div v-if="editMode === 'param'" class="param-placeholder">
          <div class="param-section" v-for="i in 3" :key="i">
            <div class="param-section-label">配置分组 {{ i }}</div>
            <div class="param-items">
              <div class="param-item" v-for="j in 4" :key="j">
                <span class="param-item-label">参数 {{ j }}</span>
                <div class="param-item-value">
                  <el-input size="small" placeholder="待填充" disabled />
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 代码模式（占位） -->
        <div v-else class="code-editor-wrap">
          <div class="code-placeholder">
            <pre>{{ codeText }}</pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.edit-area {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--card);
  overflow: hidden;
}

/* 空状态 */
.edit-empty {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 12px;
}

.edit-empty-icon {
  width: 38px;
  height: 38px;
  border-radius: 6px;
  background: var(--background);
  border: 1px solid var(--border);
  color: var(--foreground);
  display: flex;
  align-items: center;
  justify-content: center;
}

.edit-empty-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--foreground);
}

.edit-empty-desc {
  font-size: 13px;
  color: var(--muted-foreground);
  text-align: center;
  max-width: 320px;
}

.edit-empty-actions {
  display: flex;
  gap: 10px;
}

/* 编辑内容 */
.edit-content {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

.edit-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.edit-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.edit-header-icon {
  width: 38px;
  height: 38px;
  border-radius: 6px;
  background: var(--background);
  border: 1px solid var(--border);
  color: var(--foreground);
  display: flex;
  align-items: center;
  justify-content: center;
}

.edit-header-label {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
}

.dirty-badge {
  display: inline-flex;
  align-items: center;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 600;
  background: color-mix(in oklab, var(--warning) 15%, transparent);
  color: var(--warning);
}

.edit-header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.activate-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 8px;
  height: 30px;
  border-radius: 6px;
  background: var(--muted);
}

.activate-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--muted-foreground);
}

/* 模式切换 */
.mode-tabs {
  display: flex;
  gap: 2px;
  padding: 6px 14px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.mode-tab {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 5px 12px;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--muted-foreground);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.12s;
}

.mode-tab:hover {
  background: var(--accent);
  color: var(--foreground);
}

.mode-tab.active {
  background: var(--background);
  border-color: var(--border);
  color: var(--foreground);
  box-shadow: var(--console-shadow-sm);
}

/* 编辑内容区 */
.edit-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 14px;
}

/* 参数模式占位 */
.param-placeholder {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.param-section {
  border: 1px solid var(--border);
  border-radius: 6px;
  overflow: hidden;
}

.param-section-label {
  padding: 6px 10px;
  font-size: 11px;
  font-weight: 700;
  color: var(--muted-foreground);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  background: var(--muted);
  border-bottom: 1px solid var(--border);
}

.param-items {
  display: flex;
  flex-direction: column;
}

.param-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--border);
}

.param-item:last-child {
  border-bottom: 0;
}

.param-item-label {
  font-size: 12px;
  color: var(--muted-foreground);
  flex-shrink: 0;
  min-width: 100px;
}

.param-item-value {
  flex: 1;
  min-width: 0;
}

/* 代码模式占位 */
.code-editor-wrap {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

.code-placeholder {
  flex: 1;
  border: 1px solid var(--border);
  border-radius: 6px;
  overflow: auto;
  padding: 12px;
}

.code-placeholder pre {
  margin: 0;
  font-family: var(--oomol-font-mono);
  font-size: 12px;
  color: var(--muted-foreground);
}
</style>
