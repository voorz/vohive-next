<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { ArrowDownload24Regular, Delete24Regular } from '@vicons/fluent'
import {
  getAvailableIcons,
  getIconOverride,
  setIconOverride,
  clearIconOverride,
  downloadIcon,
  getIconInfo,
  getCachedIcon,
} from '../composables/useOperatorIcon'

const props = defineProps<{
  mcc: string
  mnc: string
}>()

const emit = defineEmits<{
  change: []
}>()

const selectedIcon = ref<string>('')
const selectedScope = ref<string>('')
const downloading = ref(false)
const previewSrc = ref<string | null>(null)

const availableIcons = computed(() => {
  if (!props.mcc) return []
  return getAvailableIcons(props.mcc)
})

const currentOverride = computed(() => {
  if (!props.mcc || !props.mnc) return null
  return getIconOverride(props.mcc, props.mnc)
})

const defaultIcon = computed(() => {
  if (!props.mcc || !props.mnc) return null
  return getIconInfo(props.mcc, props.mnc)
})

function refreshState() {
  const override = currentOverride.value
  if (override) {
    selectedIcon.value = override.iconName
    selectedScope.value = override.iconScope
  } else if (defaultIcon.value) {
    selectedIcon.value = defaultIcon.value.iconName
    selectedScope.value = defaultIcon.value.iconScope
  } else {
    selectedIcon.value = ''
    selectedScope.value = ''
  }
  previewSrc.value = props.mcc && props.mnc ? getCachedIcon(props.mcc, props.mnc) : null
}

watch(() => [props.mcc, props.mnc], refreshState, { immediate: true })

function onSelectIcon(icon: string, scope: string) {
  selectedIcon.value = icon
  selectedScope.value = scope
  if (props.mcc && props.mnc) {
    // 如果选的不是默认图标，设置 override
    const def = defaultIcon.value
    if (!def || def.iconName !== icon || def.iconScope !== scope) {
      setIconOverride(props.mcc, props.mnc, icon, scope)
    } else {
      clearIconOverride(props.mcc, props.mnc)
    }
    emit('change')
  }
}

function clearOverride() {
  if (props.mcc && props.mnc) {
    clearIconOverride(props.mcc, props.mnc)
    refreshState()
    emit('change')
    ElMessage.success('已恢复默认图标')
  }
}

async function handleDownload() {
  if (!props.mcc || !props.mnc) return
  downloading.value = true
  try {
    const result = await downloadIcon(props.mcc, props.mnc)
    if (result) {
      previewSrc.value = result
      ElMessage.success('图标下载成功')
    } else {
      ElMessage.error('图标下载失败')
    }
  } finally {
    downloading.value = false
  }
}

const isOverridden = computed(() => !!currentOverride.value)
</script>

<template>
  <div class="icon-picker">
    <label class="carrier-form-label">运营商图标</label>
    <div v-if="availableIcons.length === 0" class="icon-picker-empty">
      当前 MCC 无可用图标，将使用默认图标
    </div>
    <div v-else class="icon-picker-content">
      <!-- 预览 + 操作 -->
      <div class="icon-picker-preview-row">
        <div class="icon-picker-preview-box">
          <img v-if="previewSrc" :src="previewSrc" class="icon-picker-preview-img" alt="preview" />
          <div v-else class="icon-picker-preview-placeholder">
            {{ selectedIcon ? selectedIcon.slice(0, 2).toUpperCase() : '??' }}
          </div>
        </div>
        <div class="icon-picker-actions">
          <el-button size="small" :loading="downloading" @click="handleDownload">
            <el-icon class="mr-1"><ArrowDownload24Regular /></el-icon>
            <span>下载图标</span>
          </el-button>
          <el-button v-if="isOverridden" size="small" type="warning" plain @click="clearOverride">
            <el-icon class="mr-1"><Delete24Regular /></el-icon>
            <span>恢复默认</span>
          </el-button>
        </div>
      </div>
      <!-- 图标网格选择 -->
      <div class="icon-picker-grid">
        <div
          v-for="entry in availableIcons"
          :key="`${entry.icon}-${entry.scope}`"
          class="icon-picker-option"
          :class="{ selected: selectedIcon === entry.icon && selectedScope === entry.scope }"
          @click="onSelectIcon(entry.icon, entry.scope)"
        >
          <img
            :src="`https://cdn.jsdelivr.net/gh/NekokoLPA/operator-icons@master/icons/${entry.scope}/${entry.icon}.png`"
            class="icon-picker-option-img"
            loading="lazy"
            @error="($event.target as HTMLImageElement).style.display = 'none'"
          />
          <span class="icon-picker-option-label">{{ entry.icon }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.icon-picker {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.icon-picker-empty {
  font-size: 12px;
  color: var(--muted-foreground);
  padding: 8px 0;
}

.icon-picker-content {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.icon-picker-preview-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.icon-picker-preview-box {
  width: 36px;
  height: 36px;
  border-radius: 6px;
  border: 1px solid var(--border);
  background: var(--muted);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  flex-shrink: 0;
}

.icon-picker-preview-img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.icon-picker-preview-placeholder {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-foreground);
}

.icon-picker-actions {
  display: flex;
  gap: 8px;
}

.icon-picker-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(64px, 1fr));
  gap: 6px;
  max-height: 160px;
  overflow-y: auto;
  padding: 4px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--background);
}

.icon-picker-option {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 3px;
  padding: 6px 4px;
  border: 1px solid transparent;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.12s;
}

.icon-picker-option:hover {
  background: var(--accent);
}

.icon-picker-option.selected {
  background: color-mix(in oklab, var(--brand) 10%, transparent);
  border-color: color-mix(in oklab, var(--brand) 40%, var(--border));
}

.icon-picker-option-img {
  width: 24px;
  height: 24px;
  object-fit: contain;
}

.icon-picker-option-label {
  font-size: 9px;
  color: var(--muted-foreground);
  text-align: center;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 56px;
}

.carrier-form-label {
  font-size: 12px;
  font-weight: 700;
  color: var(--foreground);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
</style>
