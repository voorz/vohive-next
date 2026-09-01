<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { ChevronDown20Regular } from '@vicons/fluent'
import { systemService } from '../services/system'

const expanded = ref(true)
const editing = ref(false)
const saving = ref(false)
const loading = ref(false)

const form = ref({
  ike_retry_count: 5,
  override_rf_off: false,
  rf_off_delay: 5,
  recover_interval_seconds: 10
})

onMounted(async () => {
  loading.value = true
  try {
    const res = await systemService.getVoWiFiBehavior()
    if (res.ok) {
      form.value = res.data
    }
  } catch {
    // keep defaults
  } finally {
    loading.value = false
  }
})

function startEdit() {
  form.value = { ...form.value }
  editing.value = true
}

async function save() {
  saving.value = true
  try {
    const res = await systemService.saveVoWiFiBehavior(
      form.value.ike_retry_count,
      form.value.override_rf_off,
      form.value.rf_off_delay,
      form.value.recover_interval_seconds
    )
    if (!res.ok) throw new Error(res.error.message || '保存失败')
    ElMessage.success('VoWiFi 行为配置已保存')
    editing.value = false
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

function cancelEdit() {
  editing.value = false
  loadConfig()
}

async function loadConfig() {
  try {
    const res = await systemService.getVoWiFiBehavior()
    if (res.ok) {
      form.value = res.data
    }
  } catch {
    // keep defaults
  }
}
</script>

<template>
  <div class="faq-card mt-4" v-loading="loading">
    <div class="faq-header" @click="expanded = !expanded">
      <span class="faq-title">VoWiFi 行为控制</span>
      <el-icon class="faq-arrow" :class="{ expanded }" size="16">
        <ChevronDown20Regular />
      </el-icon>
    </div>
    <div v-show="expanded" class="faq-body">
      <div class="flex items-center justify-between mb-4">
        <div class="faq-item-title">VoWiFi 行为参数</div>
        <div class="flex items-center gap-2 shrink-0">
          <template v-if="!editing">
            <el-button size="small" @click="startEdit">编辑</el-button>
          </template>
          <template v-else>
            <el-button size="small" @click="cancelEdit">取消</el-button>
            <el-button size="small" type="primary" :loading="saving" @click="save" class="!border-0">保存</el-button>
          </template>
        </div>
      </div>

      <div class="space-y-4">
        <!-- IKE 重传次数 -->
        <div class="space-y-1">
          <div class="faq-item-title">IKE 重传次数</div>
          <el-input-number v-model="form.ike_retry_count" :min="0" :max="20" :disabled="!editing" size="small" controls-position="right" />
          <div class="faq-item-desc">单次VoWiFi流程中IKE重试次数</div>
        </div>

        <!-- RFOff 延迟 -->
        <div class="pt-4 border-t space-y-1" style="border-color: var(--border);">
          <div class="faq-item-title">RFOff 延迟（秒）</div>
          <div class="flex items-center gap-2">
            <el-input-number v-model="form.rf_off_delay" :min="0" :max="30" :disabled="!editing || !form.override_rf_off" size="small" controls-position="right" />
            <el-switch v-model="form.override_rf_off" :disabled="!editing" />
          </div>
          <div class="faq-item-desc">IKE_SA_INIT请求预热(等待网络栈重建路由表就绪)</div>
        </div>

        <!-- 重试间隔 -->
        <div class="pt-4 border-t space-y-1" style="border-color: var(--border);">
          <div class="faq-item-title">重试间隔（秒）</div>
          <el-input-number v-model="form.recover_interval_seconds" :min="0" :max="3600" :disabled="!editing" size="small" controls-position="right" />
          <div class="faq-item-desc">VoWiFi注册失败后多久内尝试下一次</div>
        </div>
      </div>
    </div>
  </div>
</template>
