<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { ChevronDown20Regular } from '@vicons/fluent'
import { systemService } from '../services/system'

const expanded = ref(true)
const saving = ref(false)
const form = ref({ login_window_minutes: 2, login_max_attempts: 10, token_ttl_hours: 720 })

onMounted(async () => {
  try {
    const res = await systemService.getSecurity()
    if (res.ok) form.value = res.data
  } catch { /* keep defaults */ }
})

async function save() {
  saving.value = true
  try {
    const res = await systemService.saveSecurity(
      form.value.login_window_minutes,
      form.value.login_max_attempts,
      form.value.token_ttl_hours,
    )
    if (!res.ok) throw new Error(res.error.message || '保存失败')
    ElMessage.success('安全配置已保存')
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
      <span class="faq-title">登录限速</span>
      <el-icon class="faq-arrow" :class="{ expanded }" size="16">
        <ChevronDown20Regular />
      </el-icon>
    </div>
    <div v-show="expanded" class="faq-body">
      <div class="flex items-center justify-between mb-4">
        <div>
          <div class="faq-item-title">登录限速配置</div>
          <div class="faq-item-desc">限制同一 IP 在指定时间窗口内的登录尝试次数</div>
        </div>
        <el-button size="small" type="primary" :loading="saving" @click="save" class="!border-0">保存配置</el-button>
      </div>
      <div class="flex flex-wrap items-center gap-6">
        <div class="flex items-center gap-2">
          <span class="faq-field-label">时间窗口</span>
          <el-input-number v-model="form.login_window_minutes" :min="1" :max="60" size="small" controls-position="right" />
          <span class="faq-field-unit">分钟</span>
        </div>
        <div class="flex items-center gap-2">
          <span class="faq-field-label">最大尝试</span>
          <el-input-number v-model="form.login_max_attempts" :min="1" :max="100" size="small" controls-position="right" />
          <span class="faq-field-unit">次</span>
        </div>
        <div class="flex items-center gap-2">
          <span class="faq-field-label">Token 有效期</span>
          <el-input-number v-model="form.token_ttl_hours" :min="1" :max="8760" size="small" controls-position="right" />
          <span class="faq-field-unit">小时</span>
        </div>
      </div>
    </div>
  </div>
</template>
