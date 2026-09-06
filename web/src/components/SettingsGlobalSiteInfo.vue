<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { ChevronDown20Regular } from '@vicons/fluent'
import { systemService } from '../services/system'
import { useSiteConfig } from '../composables/useSiteConfig'

const { siteConfig, reload } = useSiteConfig()

const expanded = ref(true)
const editing = ref(false)
const saving = ref(false)
const form = ref({ name: '', subtitle: '' })

onMounted(() => {
  form.value.name = siteConfig.value.name
  form.value.subtitle = siteConfig.value.subtitle
})

function startEdit() {
  form.value.name = siteConfig.value.name
  form.value.subtitle = siteConfig.value.subtitle
  editing.value = true
}

async function save() {
  if (!form.value.name.trim()) {
    ElMessage.error('站点名称不能为空')
    return
  }
  saving.value = true
  try {
    const res = await systemService.updateSite(form.value.name, form.value.subtitle)
    if (!res.ok) throw new Error(res.error.message || '保存失败')
    ElMessage.success('站点信息已保存')
    await reload()
    editing.value = false
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

// Logo 上传
async function onLogoChange(file: File) {
  try {
    const res = await systemService.uploadSiteLogo(file)
    if (!res.ok) throw new Error(res.error.message || '上传失败')
    ElMessage.success('Logo 已更新')
    await reload()
  } catch (e: any) {
    ElMessage.error(e.message || '上传失败')
  }
}

// Favicon 上传
async function onFaviconChange(file: File) {
  try {
    const res = await systemService.uploadSiteFavicon(file)
    if (!res.ok) throw new Error(res.error.message || '上传失败')
    ElMessage.success('Favicon 已更新')
    await reload()
  } catch (e: any) {
    ElMessage.error(e.message || '上传失败')
  }
}
</script>

<template>
  <div class="faq-card mt-4">
    <div class="faq-header" @click="expanded = !expanded">
      <span class="faq-title">站点信息</span>
      <el-icon class="faq-arrow" :class="{ expanded }" size="16">
        <ChevronDown20Regular />
      </el-icon>
    </div>
    <div v-show="expanded" class="faq-body">
      <!-- 名称 + 副标题（编辑锁） -->
      <div class="flex items-center justify-between mb-3">
        <div>
          <div class="faq-item-title">站点名称与副标题</div>
          <div class="faq-item-desc">显示在浏览器标签页标题和侧栏品牌区域</div>
        </div>
        <div class="flex items-center gap-2">
          <template v-if="!editing">
            <el-button size="small" @click="startEdit">编辑</el-button>
          </template>
          <template v-else>
            <el-button size="small" @click="editing = false">取消</el-button>
            <el-button size="small" type="primary" :loading="saving" @click="save" class="!border-0">保存</el-button>
          </template>
        </div>
      </div>
      <div class="space-y-3 max-w-md mb-3">
        <div class="space-y-1">
          <label class="settings-form-label">站点名称</label>
          <el-input v-model="form.name" :disabled="!editing" placeholder="VoHive" size="large" />
        </div>
        <div class="space-y-1">
          <label class="settings-form-label">副标题</label>
          <el-input v-model="form.subtitle" :disabled="!editing" placeholder="VOWIFI多设备管理控制台" size="large" />
        </div>
      </div>

      <!-- Logo 上传 -->
      <div class="flex items-center justify-between mb-3 pt-3 border-t" style="border-color: var(--border);">
        <div>
          <div class="faq-item-title">自定义 Logo</div>
          <div class="faq-item-desc">支持 PNG/SVG/GIF，正方形或长方形均可。为空时显示默认字母标记</div>
        </div>
        <el-upload :show-file-list="false" :before-upload="onLogoChange" accept="image/*">
          <el-button size="small">上传 Logo</el-button>
        </el-upload>
      </div>
      <div v-if="siteConfig.has_logo" class="flex items-center gap-3 mb-3">
        <img :src="'/api/site/logo'" alt="Logo" style="max-height: 40px; max-width: 160px; object-fit: contain;" />
        <el-tag type="success" size="small">已上传</el-tag>
      </div>

      <!-- Favicon 上传 -->
      <div class="flex items-center justify-between mb-3 pt-3 border-t" style="border-color: var(--border);">
        <div>
          <div class="faq-item-title">自定义 Favicon</div>
          <div class="faq-item-desc">建议使用正方形 ICO/PNG/SVG，显示在浏览器标签页</div>
        </div>
        <el-upload :show-file-list="false" :before-upload="onFaviconChange" accept="image/*,.ico">
          <el-button size="small">上传 Favicon</el-button>
        </el-upload>
      </div>
      <div v-if="siteConfig.has_favicon" class="flex items-center gap-3">
        <img :src="'/api/site/favicon'" alt="Favicon" style="max-height: 32px; max-width: 32px; object-fit: contain;" />
        <el-tag type="success" size="small">已上传</el-tag>
      </div>
    </div>
  </div>
</template>
