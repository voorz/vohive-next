<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from 'vue'
import { storeToRefs } from 'pinia'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useCarrierStore } from '../stores/carrier'
import CarrierConfigForm from './CarrierConfigForm.vue'
import { Codemirror } from 'vue-codemirror'
import { json } from '@codemirror/lang-json'
import { oneDark } from '@codemirror/theme-one-dark'
import { EditorView } from 'codemirror'
import {
  Person24Regular,
  Save24Regular,
  Delete24Regular,
  DocumentEdit24Regular,
  Code24Regular,
  ArrowUpRight24Regular,
  DocumentAdd24Regular
} from '@vicons/fluent'

const store = useCarrierStore()
const { editingConfig, editMode, dirty, saving, detail } = storeToRefs(store)

// 代码模式 JSON 文本
const codeText = ref('')
const codeError = ref('')

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

const cmExtensions = computed(() => {
  const exts = [json(), EditorView.lineWrapping]
  if (isDark.value) exts.push(oneDark)
  return exts
})

// CodeMirror 变更回调
function onCmChange(value: string) {
  codeText.value = value
  onCodeInput()
}

// 从 store 同步到代码文本
watch(
  () => editingConfig.value,
  (cfg) => {
    if (cfg) {
      codeText.value = JSON.stringify(cfg, null, 2)
      codeError.value = ''
    }
  },
  { immediate: true, deep: false }
)

// 代码模式编辑时实时解析
function onCodeInput() {
  try {
    const parsed = JSON.parse(codeText.value)
    codeError.value = ''
    store.syncFromJson(parsed)
  } catch (e: unknown) {
    codeError.value = e instanceof Error ? e.message : 'JSON 解析错误'
  }
}

function switchMode(mode: 'param' | 'code') {
  if (mode === 'code' && editingConfig.value) {
    // 切到代码模式前，同步最新 editingConfig 到文本
    codeText.value = JSON.stringify(editingConfig.value, null, 2)
    codeError.value = ''
  }
  store.setEditMode(mode)
}

async function handleSave() {
  if (codeError.value) {
    ElMessage.warning('JSON 格式有误，请修正后再保存')
    return
  }
  // 如果在代码模式，确保同步
  if (editMode.value === 'code') {
    try {
      const parsed = JSON.parse(codeText.value)
      store.syncFromJson(parsed)
    } catch {
      ElMessage.warning('JSON 格式有误，请修正后再保存')
      return
    }
  }
  const ok = await store.saveUserConfig()
  if (ok) ElMessage.success('用户配置已保存')
  else ElMessage.error('保存失败')
}

async function handleDelete() {
  const confirmed = await ElMessageBox.confirm(
    '确定删除用户配置模板？删除后运营商条目保留，配置回退到系统默认。',
    '确认删除用户模板',
    { confirmButtonText: '删除模板', cancelButtonText: '取消', type: 'warning' }
  ).then(() => true).catch(() => false)
  if (!confirmed) return
  const ok = await store.deleteUserConfig()
  if (ok) ElMessage.success('用户配置模板已删除')
}

async function handleToggleActive() {
  if (!detail.value) return
  if (detail.value.active) {
    const ok = await store.deactivateUserConfig()
    if (ok) ElMessage.success('已禁用用户配置，回退系统默认')
  } else {
    const ok = await store.activateUserConfig()
    if (ok) ElMessage.success('已启用用户配置')
  }
}

const hasUserConfig = computed(() => !!detail.value?.user_config || !!editingConfig.value)
</script>

<template>
  <div class="edit-area">
    <!-- 无用户配置状态 -->
    <div v-if="!hasUserConfig" class="edit-empty">
      <div class="edit-empty-icon">
        <el-icon size="28"><Person24Regular /></el-icon>
      </div>
      <div class="edit-empty-title">尚未创建用户配置</div>
      <div class="edit-empty-desc">从系统默认或 3GPP 标准模板创建配置</div>
      <div class="edit-empty-actions">
        <el-button type="primary" @click="store.createFromSystemDefault()" class="!border-0" :disabled="!detail?.system_default">
          <el-icon class="mr-1.5"><ArrowUpRight24Regular /></el-icon>
          <span>从系统默认创建</span>
        </el-button>
        <el-button @click="store.createFromStandardTemplate()">
          <el-icon class="mr-1.5"><DocumentAdd24Regular /></el-icon>
          <span>从标准模板创建</span>
        </el-button>
      </div>
    </div>

    <!-- 有用户配置：编辑区 -->
    <div v-else class="edit-content">
      <!-- 编辑区头部 -->
      <div class="edit-header">
        <div class="edit-header-left">
          <div class="edit-header-icon">
            <el-icon size="16"><Person24Regular /></el-icon>
          </div>
          <span class="edit-header-label">用户配置</span>
          <span v-if="dirty" class="dirty-badge">未保存</span>
        </div>
        <div class="edit-header-right">
          <div class="activate-toggle">
            <span class="activate-label">启用</span>
            <el-switch
              :model-value="detail?.active || false"
              @change="handleToggleActive"
            />
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

      <!-- 编辑内容（内联滚动） -->
      <div class="edit-body">
        <!-- 参数模式 -->
        <CarrierConfigForm v-if="editMode === 'param'" />

        <!-- 代码模式 -->
        <div v-else class="code-editor-wrap">
          <div class="cm-container">
            <Codemirror
              :model-value="codeText"
              :extensions="cmExtensions"
              :style="{ height: '100%' }"
              @update:model-value="onCmChange"
            />
          </div>
          <div v-if="codeError" class="code-error">
            <el-alert type="error" :closable="false" show-icon>
              {{ codeError }}
            </el-alert>
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
  padding: 32px;
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

/* 编辑内容区（内联滚动） */
.edit-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 14px;
}

/* 代码编辑器 */
.code-editor-wrap {
  display: flex;
  flex-direction: column;
  gap: 8px;
  height: 100%;
  min-height: 0;
}

.cm-container {
  flex: 1;
  min-height: 300px;
  border: 1px solid var(--border);
  border-radius: 6px;
  overflow: hidden;
}

.cm-container :deep(.cm-editor) {
  height: 100%;
  font-size: 12px;
  font-family: var(--oomol-font-mono);
}

.cm-container :deep(.cm-scroller) {
  font-family: var(--oomol-font-mono);
}

.code-error {
  flex-shrink: 0;
}
</style>
