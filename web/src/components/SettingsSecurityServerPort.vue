<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { ChevronDown20Regular } from '@vicons/fluent'
import { systemService } from '../services/system'

const expanded = ref(true)
const editing = ref(false)
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
    ElMessage.success(res.data?.message || '配置已保存')
    editing.value = false
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
      <span class="faq-title">Web 管理端口</span>
      <el-icon class="faq-arrow" :class="{ expanded }" size="16">
        <ChevronDown20Regular />
      </el-icon>
    </div>
    <div v-show="expanded" class="faq-body">
      <div class="flex items-center justify-between mb-4">
        <div>
          <div class="faq-item-title">服务器端口</div>
          <div class="faq-item-desc">修改后需重启服务生效</div>
        </div>
        <div class="flex items-center gap-2">
          <template v-if="!editing">
            <el-button size="small" @click="editing = true">编辑</el-button>
          </template>
          <template v-else>
            <el-button size="small" @click="editing = false">取消</el-button>
            <el-button size="small" type="primary" :loading="saving" @click="save" class="!border-0">保存端口</el-button>
          </template>
        </div>
      </div>
      <div class="flex items-center gap-3 max-w-md">
        <label class="faq-field-label shrink-0">端口</label>
        <el-input v-model="form.port" :disabled="!editing" placeholder="7575" size="large" class="flex-1" />
      </div>
    </div>
  </div>
</template>
