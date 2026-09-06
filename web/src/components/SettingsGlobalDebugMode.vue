<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { ChevronDown20Regular } from '@vicons/fluent'
import { systemService } from '../services/system'

const expanded = ref(true)
const saving = ref(false)
const form = ref({ port: '7575', debug: false })

onMounted(async () => {
  try {
    const res = await systemService.getServerConfig()
    if (res.ok) form.value = res.data
  } catch { /* keep defaults */ }
})

async function save() {
  saving.value = true
  try {
    const res = await systemService.saveServerConfig(form.value.port, form.value.debug)
    if (!res.ok) throw new Error(res.error.message || '保存失败')
    ElMessage.success('配置已保存')
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="faq-card mt-4">
    <div class="faq-header" @click="expanded = !expanded">
      <span class="faq-title">调试模式</span>
      <el-icon class="faq-arrow" :class="{ expanded }" size="16">
        <ChevronDown20Regular />
      </el-icon>
    </div>
    <div v-show="expanded" class="faq-body">
      <div class="flex items-center justify-between mb-3">
        <div>
          <div class="faq-item-title">Debug 模式</div>
          <div class="faq-item-desc">开启后显示详细日志信息，用于排查问题</div>
        </div>
        <el-button size="small" type="primary" :loading="saving" @click="save" class="!border-0">保存</el-button>
      </div>
      <div class="flex items-center gap-3">
        <el-switch v-model="form.debug" />
        <span class="faq-field-label">{{ form.debug ? '已开启' : '已关闭' }}</span>
      </div>
    </div>
  </div>
</template>
