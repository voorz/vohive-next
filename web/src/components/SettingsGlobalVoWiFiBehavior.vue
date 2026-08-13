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
  rf_off_delay: 5
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
  // backup current values for cancel
  form.value = { ...form.value }
  editing.value = true
}

async function save() {
  saving.value = true
  try {
    const res = await systemService.saveVoWiFiBehavior(
      form.value.ike_retry_count,
      form.value.override_rf_off,
      form.value.rf_off_delay
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
  // reload from server
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
        <div>
          <div class="faq-item-title">IKE 重传与拆除重建</div>
          <div class="faq-item-desc">IKE 重传次数耗尽后触发整个 VoWiFi 拆除重建（等同手动重连）</div>
        </div>
        <div class="flex items-center gap-2">
          <template v-if="!editing">
            <el-button size="small" @click="startEdit">编辑</el-button>
          </template>
          <template v-else>
            <el-button size="small" @click="cancelEdit">取消</el-button>
            <el-button size="small" type="primary" :loading="saving" @click="save" class="!border-0">保存</el-button>
          </template>
        </div>
      </div>

      <div class="space-y-4 max-w-md">
        <!-- IKE 重传次数 -->
        <div class="space-y-1">
          <label class="settings-form-label">IKE 重传次数</label>
          <el-input-number
            v-model="form.ike_retry_count"
            :min="0"
            :max="20"
            :disabled="!editing"
            controls-position="right"
            class="w-full !w-full"
          />
          <div class="settings-form-hint">
            0 = 使用默认值（5 次）。重传耗尽后触发整个 VoWiFi 拆除重建，重建后重传次数重置。
          </div>
        </div>

        <!-- RFOff 延迟覆盖 -->
        <div class="pt-4 border-t" style="border-color: var(--border);">
          <div class="flex items-center justify-between mb-3">
            <div>
              <div class="faq-item-title">飞行模式延迟覆盖</div>
              <div class="faq-item-desc">勾选后使用全局值替代运营商预设的 RFOff 延迟</div>
            </div>
            <el-switch v-model="form.override_rf_off" :disabled="!editing" />
          </div>

          <div class="space-y-1" :class="{ 'opacity-50': !form.override_rf_off }">
            <label class="settings-form-label">RFOff 延迟（秒）</label>
            <el-input-number
              v-model="form.rf_off_delay"
              :min="0"
              :max="30"
              :disabled="!editing || !form.override_rf_off"
              controls-position="right"
              class="w-full !w-full"
            />
            <div class="settings-form-hint">
              飞行模式后等待网络栈稳定的秒数。仅在覆盖开启时生效。
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
