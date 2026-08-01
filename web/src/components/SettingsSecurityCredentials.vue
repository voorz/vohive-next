<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { ChevronDown20Regular } from '@vicons/fluent'
import { systemService } from '../services/system'

const props = defineProps<{
  username: string
}>()

const expanded = ref(true)
const editing = ref(false)
const saving = ref(false)
const form = ref({ username: '', password: '' })

watch(() => props.username, (v) => {
  if (v) form.value.username = v
}, { immediate: true })

async function save() {
  if (!form.value.username || !form.value.password) {
    ElMessage.error('用户名和密码不能为空')
    return
  }
  saving.value = true
  try {
    const res = await systemService.updateWebCredentials(form.value.username, form.value.password)
    if (!res.ok) throw new Error(res.error.message || '更新失败')
    ElMessage.success('凭据已更新')
    form.value.password = ''
    editing.value = false
  } catch (e: any) {
    ElMessage.error(e.message || '更新失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="faq-card">
    <div class="faq-header" @click="expanded = !expanded">
      <span class="faq-title">登录凭据</span>
      <el-icon class="faq-arrow" :class="{ expanded }" size="16">
        <ChevronDown20Regular />
      </el-icon>
    </div>
    <div v-show="expanded" class="faq-body">
      <div class="flex items-center justify-between mb-4">
        <div>
          <div class="faq-item-title">管理员用户名与密码</div>
          <div class="faq-item-desc">修改后立即生效，新登录将使用新凭据</div>
        </div>
        <div class="flex items-center gap-2">
          <template v-if="!editing">
            <el-button size="small" @click="editing = true">编辑</el-button>
          </template>
          <template v-else>
            <el-button size="small" @click="editing = false; form.password = ''">取消</el-button>
            <el-button size="small" type="primary" :loading="saving" @click="save" class="!border-0">保存凭据</el-button>
          </template>
        </div>
      </div>
      <div class="space-y-3 max-w-md">
        <div class="space-y-1">
          <label class="settings-form-label">用户名</label>
          <el-input v-model="form.username" :disabled="!editing" placeholder="admin" size="large" />
        </div>
        <div class="space-y-1">
          <label class="settings-form-label">密码</label>
          <el-input v-model="form.password" :disabled="!editing" type="password" show-password placeholder="••••••••" size="large" />
        </div>
      </div>
    </div>
  </div>
</template>
