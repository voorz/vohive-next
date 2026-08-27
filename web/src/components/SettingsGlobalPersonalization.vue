<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ChevronDown20Regular } from '@vicons/fluent'
import { clearAllIconCache, getIconCacheCount } from '../composables/useOperatorIcon'
import { clearAllFlagCache, countFlagCache } from '../composables/useCountryFlag'

const STORAGE_KEY = 'vohive.personalization'

const expanded = ref(true)
const saving = ref(false)
const clearing = ref(false)
const form = ref({
  use_custom_icons: true,
})

const iconCacheCount = ref(0)

onMounted(() => {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      form.value.use_custom_icons = parsed.use_custom_icons ?? true
    }
  } catch { /* keep defaults */ }
  countIconCache()
})

function save() {
  saving.value = true
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(form.value))
    ElMessage.success('配置已保存')
  } catch {
    ElMessage.error('保存失败')
  } finally {
    saving.value = false
  }
}

async function countIconCache() {
  const [iconCounts, flagCount] = await Promise.all([
    getIconCacheCount(),
    countFlagCache(),
  ])
  iconCacheCount.value = iconCounts.icons + iconCounts.overrides + flagCount
}

async function clearIconCache() {
  try {
    await ElMessageBox.confirm(
      `将清除 ${iconCacheCount.value} 个已缓存的图标（运营商图标 + 国家旗帜），下次使用时需重新下载。是否继续？`,
      '清除图标缓存',
      { confirmButtonText: '清除', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }
  clearing.value = true
  try {
    await clearAllIconCache()
    await clearAllFlagCache()
    await countIconCache()
    ElMessage.success('图标缓存已清除')
  } catch {
    ElMessage.error('清除失败')
  } finally {
    clearing.value = false
  }
}
</script>

<template>
  <div class="faq-card mt-4">
    <div class="faq-header" @click="expanded = !expanded">
      <span class="faq-title">个性化配置</span>
      <el-icon class="faq-arrow" :class="{ expanded }" size="16">
        <ChevronDown20Regular />
      </el-icon>
    </div>
    <div v-show="expanded" class="faq-body">
      <div class="flex items-center justify-between mb-4">
        <div>
          <div class="faq-item-title">使用外部图标资源</div>
          <div class="faq-item-desc">显示个性化图标，关闭后使用默认图标</div>
        </div>
        <el-button size="small" type="primary" :loading="saving" @click="save" class="!border-0">保存</el-button>
      </div>
      <div class="flex items-center gap-3 mb-4">
        <el-switch v-model="form.use_custom_icons" />
        <span class="faq-field-label">{{ form.use_custom_icons ? '已开启' : '已关闭' }}</span>
      </div>

      <div class="flex items-center justify-between" style="border-top: 1px solid var(--border); padding-top: 12px;">
        <div>
          <div class="faq-item-title">缓存管理</div>
          <div class="faq-item-desc">已缓存 {{ iconCacheCount }} 个图标（运营商 + 国旗），清除后需重新下载</div>
        </div>
        <el-button
          size="small"
          type="danger"
          plain
          :loading="clearing"
          :disabled="iconCacheCount === 0"
          @click="clearIconCache"
        >
          清除缓存
        </el-button>
      </div>
    </div>
  </div>
</template>
